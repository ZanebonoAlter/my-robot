## MODIFIED Requirements

### Requirement: 关系写入逻辑
`SaveReport()` 事务中，新 sections 写入数据库后（含两阶段合并），系统 SHALL 对该 board 执行全量关系重建：清除该 board 所有 relation，按相邻天对逐对执行三阶段二分图匹配（Phase 1 匈牙利 1:1 + Phase 2 split/merge + Phase 3 skip-day），重新写入 relation。

embedding 为空的 section SHALL 跳过关系写入。section 的 embedding SHALL 基于 section 内容聚合文本生成（见 section-content-embedding 能力：所聚 tag 的 label/description/代表文章摘录），SHALL NOT 再基于 `cluster_label` 标题文本。

distance 字段存储 pgvector 计算的 cosine distance 值，写入方式使用 raw SQL `INSERT ... ON CONFLICT DO UPDATE` 确保 upsert 时 distance 被正确写入。

以下旧函数/逻辑 SHALL 被删除，由二分图匹配取代：
- `shouldWriteRelation`（两层时间过滤）→ 由 Phase 1 penalty 机制取代
- `competitiveFilter`（候选竞争过滤）→ 由 Phase 2 gap 检测取代
- `hasContinuationInIntermediateDays`（中间天延续检查）→ 由 Phase 3 skip-day 逻辑取代
- `matchCandidate` 结构体 → 由新的匹配结果结构取代

此外，`SaveReport` 中**旧 relations 删除逻辑**（line 64-70，删除当天旧 section 的 relation）SHALL 一并移除——`RebuildBoardRelations` 会清除整个 board 的 relations，旧逻辑变为冗余。

**回刷支持**：系统 SHALL 提供 `RebuildBoardRelations(tx *gorm.DB, boardID uint) error` 核心函数，接收 `*gorm.DB`（事务或裸 DB），内部不做事务管理，由调用方控制事务生命周期。

`BackfillRelations(boardID uint)` SHALL 为薄包装：自行开启事务，调用 `RebuildBoardRelations(tx, boardID)`，提交事务。`BackfillAllRelations()` SHALL 查询所有有 embedding section 的 board 逐个调用 `BackfillRelations`。

**失败处理**：`RebuildBoardRelations` 任何阶段失败时，整个 board 的 relation 重建回滚（由调用方事务保证），报告保存本身不受影响（仍以 warn 日志记录，不中止报告保存）。半完成的拓扑比没有拓扑更糟糕，选择全有或全无。

#### Scenario: SaveReport 触发全量重连
- **WHEN** 新报告保存到 board 2853
- **THEN** 系统 SHALL 清除 board 2853 的所有 relation，按相邻天对逐对执行三阶段二分图匹配，写入新的 relation

#### Scenario: 单个相邻天完美延续
- **WHEN** Day_i section "美伊谈判进展" (id=692) 和 Day_{i+1} section "美伊谈判动态" (id=788) embedding distance=0.073
- **THEN** Phase 1 匈牙利匹配 SHALL 产生 primary relation (from=692, to=788, distance=0.073)

#### Scenario: 不相关 section 不被强制匹配
- **WHEN** Day_i section "日本推进北约合作" 和 Day_{i+1} section "塞尔维亚经济危机" distance=0.337 > penalty
- **THEN** 匈牙利算法 SHALL 不匹配这两个 section（penalty 机制阻止硬凑）

#### Scenario: 距离超阈值不写关系
- **WHEN** 某新 section 与相邻天所有旧 section 的 distance 均 > penalty=0.28
- **THEN** 系统 SHALL 不为该 section 写入任何 relation，其 status 为 emerging

#### Scenario: 空 embedding 跳过
- **WHEN** 新 section 的 embedding 为 NULL
- **THEN** 系统 SHALL 跳过该 section 的关系写入

#### Scenario: 日期不连续时视为相邻天
- **WHEN** board 只有 6/1 和 6/3 的报告（6/2 无报告）
- **THEN** 6/1 和 6/3 SHALL 被视为相邻天，执行 Phase 1 匹配

#### Scenario: BackfillRelations 全量回刷
- **WHEN** 运行 `BackfillRelations(boardID=2853)`
- **THEN** 系统 SHALL 删除 board 2853 涉及的所有 relation，按相邻天对逐对执行三阶段二分图匹配

#### Scenario: BackfillAllRelations 批量回刷
- **WHEN** 运行 `BackfillAllRelations()`
- **THEN** 系统 SHALL 查询所有有 embedding section 的 board，逐个调用 `BackfillRelations`

#### Scenario: BackfillSectionEmbeddings Phase 2
- **WHEN** 运行 `BackfillSectionEmbeddings`
- **THEN** Phase 1 按 section-content-embedding 能力的内容化规则生成/重算 embedding 后，Phase 2 SHALL 对每个 board 调用 `BackfillRelations(boardID)`
