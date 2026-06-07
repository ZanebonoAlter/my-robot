## Context

当前 `MatchAndSaveRelations`（`repository.go:390`）采用增量贪心匹配：`SaveReport` 中对每个新 section 查询同 board 下所有更早 section 的 embedding 距离，经 `shouldWriteRelation`（两层时间过滤）+ `competitiveFilter`（候选竞争）后写入 relation。依赖已有 relation 构建 adjacency 做中间天延续检查。

实测暴露的问题：
- Board 3639（AI技术，71 sections）：118 条 relation 中 50 条 dist>0.25，31 个 section 被误标 merge，仅 2 个 continuing
- Board 2853（伊朗局势，37 sections）：36 条 relation 中 8 条跳天连线，9 个 merge
- 根本原因：增量匹配依赖历史 relation 正确性，误差在 adjacency 上累积传播

二分图模拟验证（penalty=0.28, split_gap=0.03, skip_day_threshold=0.20）：
- Board 3639：118→46 relation，merge 31→4，continuing 2→31
- Board 2853：36→20 relation，merge 9→1，continuing 6→13，保住了 792→996 的有价值 skip-day

## Goals / Non-Goals

**Goals:**
- 用相邻天二分图最优匹配替换增量贪心匹配，消除累积误差
- 每个 board 的 relation 拓扑在新报告产生后自动全量重建
- 保留真正有价值的 skip-day 关系（中间天无延续 + 强匹配）
- 保留 split/merge 检测能力

**Non-Goals:**
- 不改变 `DeriveSectionStatuses` 的 status 推导算法
- 不改变 embedding 生成逻辑
- 不改变同日 section 合并逻辑
- 不改变 relation 表 schema
- 不涉及前端 API 响应格式变更
- 不做回溯修正（历史 relation 通过 backfill 重建，不做增量修补）

## Decisions

### D1: 三阶段匹配算法

**选择**：对每对相邻天执行三阶段匹配。

**Phase 1 — 匈牙利 1:1 + Penalty**
- 构建距离矩阵：`left_sections × right_sections`，dist > penalty 的位置设为 INF
- 加入虚拟节点（penalty 代价），允许算法选择"不匹配"
- 匈牙利算法求全局最优 1:1 分配
- penalty=0.28：超过此距离的匹配不被考虑

**Phase 2 — Split/Merge 检测**
- 对未匹配的 right section：查已匹配 left section 的次优匹配，gap < split_gap(0.03) → split
- 对未匹配的 left section：查已匹配 right section 的次优匹配，gap < split_gap(0.03) → merge
- split_ceiling=0.30：超过此距离的候选不考虑

**Phase 3 — Skip-Day 补查**
- 对 Phase 1/2 中完全未匹配的 section，检查**隔一天**（Day_i → Day_{i+2}）的 section
- 条件：from_section 在中间天（Day_{i+1}）无任何延续关系 + distance < 0.20（严格阈值）
- 只对 unmatched section 做补查，量极少
- **限制为仅隔一天**：隔两天以上的匹配语义过弱，且实际数据中几乎所有有价值的 skip-day 均为隔一天

**备选**：只做 Phase 1（纯 1:1），不保留 split/merge。简单但丢失多对多语义。

**理由**：三阶段设计在模拟中验证了 merge 从 31→4（Board 3639）和 9→1（Board 2853），同时保留了有价值的 skip-day（792→996, dist=0.088）。

### D2: 新报告触发 board 级全量重连

**选择**：`SaveReport` 中，新报告保存后调用 `RebuildBoardRelations(tx, boardID)`，清除该 board 所有 relation 并按相邻天对逐对执行三阶段匹配。

**事务模型**：`RebuildBoardRelations` 接收 `*gorm.DB` 参数（可以是事务也可以是裸 DB），内部不做事务管理（不 Begin/Commit/Rollback），由调用方控制事务生命周期：
- `SaveReport` 在自己的 `database.DB.Transaction` 中传入 `tx`，relation 重建与报告保存在同一事务中
- `BackfillRelations` 自行 `Begin()` 后传入 `tx`，完成后 `Commit()`

**备选**：
- 只对新 section 做匹配（增量）。保留了旧算法的累积误差问题，不采用。
- 只对新旧两天做匹配。不会修正历史 relation，不采用。

**理由**：全量重建保证拓扑一致性。成本分析：单个 board 通常 < 100 sections，每天 5-15 个 section，相邻天对约 30 对，每对一次 cross-join SQL + 内存匈牙利匹配（<50ms），总计 < 2 秒。可接受。

### D3: 匈牙利算法实现

**选择**：纯 Go 实现 O(n³) 匈牙利算法，n ≤ max(left, right)。

**备选**：使用第三方 Go 库。但对于 n≤50 的场景，手写更简单且无外部依赖。

**理由**：Board 单天 section 数通常在 5-20，O(n³) 完全够用。即使未来单天 50 sections，50³=125000 次操作在 Go 中 < 1ms。

### D4: 距离矩阵查询策略

**选择**：对每对相邻天，用单条 SQL cross-join + pgvector 距离计算一次性获取所有 pair 距离，在内存中构建矩阵。

```sql
SELECT s1.id, s2.id, s1.embedding <=> s2.embedding AS dist
FROM daily_report_sections s1, daily_report_sections s2
JOIN ... -- 日期过滤
WHERE s1.embedding <=> s2.embedding < 0.35
```

**备选**：对每个 section 单独查询最近邻。查询次数 = section 数 × 天数，不可接受。

**理由**：cross-join 在 20×20=400 对上极快，pgvector hnsw 索引加持下单次查询 < 10ms。内存构建矩阵后匈牙利算法在纯内存中运行，无 DB 往返。

### D5: BackfillRelations 重写

**选择**：重写为 `BackfillRelations(boardID)` 的薄包装——自行开启事务，调用 `RebuildBoardRelations(tx, boardID)`，提交事务。

**流程**：
1. `tx := database.DB.Begin()`
2. 调用 `RebuildBoardRelations(tx, boardID)`（清除 relation → 加载 section → 按相邻天对逐对 Phase 1+2+3 → 写入 relation）
3. `tx.Commit()`

**理由**：与日常 `SaveReport` 触发的重连完全一致，无逻辑分歧。事务由 `BackfillRelations` 自行管理，`RebuildBoardRelations` 不涉及事务。

### D6: 重连期间前端状态

**选择**：不做特殊处理。全量重连耗时 < 2 秒（单个 board），且在事务中执行（事务原子性保证前端查询要么看到旧拓扑要么看到新拓扑，不会看到中间状态）。下次查询 timeline API 时自然获取最新 relation。

**备选**：WebSocket 推送重连完成通知或 `SaveReport` 响应添加标记。但 `SaveReport` 由后台生成任务调用，前端不直接接收其 HTTP 响应；WebSocket 推送则是过度设计。

**理由**：重连在 `SaveReport` 事务中同步完成，前端在收到 WebSocket progress "completed" 事件后查询 timeline，此时 relation 已就绪。无需额外机制。

### D7: 阈值参数

| 参数 | 值 | 说明 |
|------|------|------|
| penalty | 0.28 | 匈牙利算法的不匹配代价，超过此距离不考虑 |
| split_gap | 0.03 | Phase 2 中 split/merge 检测的 gap 阈值 |
| split_ceiling | 0.30 | Phase 2 候选的最大距离上限 |
| skip_day_threshold | 0.20 | Phase 3 skip-day 补查的严格距离阈值 |
| query_cutoff | 0.35 | SQL cross-join 的距离截断（减少无效计算） |

这些参数在两个 board 上都验证过效果良好。

## Risks / Trade-offs

- **[全量重连的并发安全]** → `SaveReport` 已在事务中运行，重连也在事务中。如果两个报告同时保存到同一 board，后一个会等前一个事务完成。可接受。
- **[penalty=0.28 仍会产生勉强匹配]** → 如 Board 2853 中 696→792 (d=0.275) "革命卫队打击视频"→"最高领袖公开露面"。语义上弱相关但非完全错误。降低 penalty 会丢失更多关系。可在后续根据更多 board 数据微调。
- **[Phase 3 skip-day 阈值 0.20 严格]** → 某些有价值的 skip-day 可能被过滤（如 761→945 英伟达 Gamma-World→英伟达生态 d=0.204）。但宁可漏掉也不要引入噪声。
- **[大 board 的性能]** → 1000 sections / 30 天 = 29 对相邻天，每天约 33 sections。单次 cross-join 33×33 + 匈牙利 ≈ 20ms，总计 ≈ 600ms。可接受。极端情况（单天 100 sections）下 100×100 cross-join 仍 < 50ms。
- **[前端暂存状态]** → 重连在 `SaveReport` 事务中同步完成，前端收到 WebSocket "completed" 事件时 relation 已就绪。查询 timeline API 要么看到旧拓扑要么看到新拓扑（事务原子性），无中间状态。
- **[RebuildBoardRelations 失败处理]** → 任何阶段（Phase 1/2/3 或 relation 写入）失败时，整个 board 的 relation 重建回滚（事务原子性），报告保存本身不受影响（relation 匹配错误仍以 warn 日志记录，不中止报告保存）。半完成的拓扑比没有拓扑更糟糕，因此选择全有或全无。
- **[generateAllBoards 串行延迟]** → `generateAllBoards` 串行处理所有 board，每个 board 多 1-2 秒重建时间。10 个 board 约 10-20 秒增量。当前 board 数量有限（< 20），可接受。未来 board 数量增长时可考虑仅对受影响天对做增量重建。
- **[cluster_label 过滤]** → cross-join SQL 必须保留 `cluster_label IS NOT NULL AND cluster_label != ''` 过滤条件。当前 `BackfillRelations` 已有此过滤，遗漏会导致空标签 section 参与匹配。
