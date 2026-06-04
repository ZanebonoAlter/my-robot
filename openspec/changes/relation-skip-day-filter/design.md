## Context

`MatchAndSaveRelations` 当前对每个新 section 做 embedding 全量匹配：查同 board 下所有非当日 section，distance < 0.35 就写 relation。这导致跨越多天的"跳跃连线"——例如 6/1 的 section 同时匹配 6/2 和 6/3，使得 6/1→6/3 产生直连。

实际数据（board 2853 伊朗局势）暴露的问题：
- 6/1→6/2（相邻天）：5 条 relation，全部 dist=0.000，完美延续
- 6/1→6/3（跨天跳跃）：37 条 relation，平均 dist=0.266，几乎全是噪声
- 6/3 的 section 入度 13-14，导致 status 推导严重失真
- 6/2 的 section 本应是 `continuing`（从 6/1 完美延续），却因 6/1 出度 > 1 被标为 `split`

但数据中也存在有价值的跨天关系：690（6/1 伊朗军事冲突）在 6/2 无延续，但到 932（6/3 美伊直接军事冲突）dist=0.094——这是真正的"隔天续上"，应保留。

## Goals / Non-Goals

**Goals:**
- 消除跨天跳跃噪声 relation，使 status 推导基于真实的叙事流动
- 保留有价值的隔天续上关系（中间天无延续的强匹配）
- 不改变 relation 表 schema 和 API 响应格式

**Non-Goals:**
- 不改变 status 推导算法（DeriveSectionStatuses）
- 不改变 embedding 生成逻辑
- 不改变同日 section 合并逻辑
- 不涉及前端改动

## Decisions

### D1: 两层过滤策略（相邻天 vs 跨天）

**选择**：对 `MatchAndSaveRelations` 的匹配结果按天分组，对"跨天匹配"施加额外过滤。

**规则**：
1. **相邻天匹配**（from_section 和 to_section 之间无其他已完成报告的天）：distance < 0.35 → 直接写入（不变）
2. **跨天匹配**（from_section 和 to_section 之间有至少一天有已完成报告）：需同时满足：
   - from_section 在中间天无任何延续关系（即 from_section 没有指向中间天 section 的 relation）
   - distance < 0.25（比相邻天阈值更严格）

**备选**：只匹配相邻天，完全不匹配跨天。简单但会丢失"隔天续上"的真正关系。

**理由**：数据表明跨天 relation 的价值高度依赖是否有中间天延续。690→932（无中间天延续 + dist=0.094）是真隔天续上；692→932（有 6/2 延续 + dist=0.213）是冗余噪声。两层过滤既消除噪声又保留真实跨天关系。

### D2: 跨天距离阈值 0.25

**选择**：跨天匹配收紧到 0.25（相邻天仍为 0.35）。

**数据依据**：
- 690→932 (0.094) 和 690→933 (0.131)：真隔天续上，STRONG
- 700→934 (0.215) 和 700→935 (0.222)：有一定相关性，MODERATE
- 693→933 (0.254+)：弱，噪声

0.25 阈值能保留 690 和 700 的跨天关系，过滤掉 693/697/694 的噪声。

### D3: 回刷策略

**选择**：新增 `BackfillRelations(boardID)` 函数，清理指定 board 的所有 relation 并按新规则重建。

**流程**：
1. 删除该 board 涉及的所有 relation 记录
2. 按日期从早到晚遍历每个 section，对每个 section 重新执行带过滤的 `MatchAndSaveRelations` 逻辑
3. 按天顺序处理确保"中间天是否有延续"的判断基于已写入的 relation

**备选**：只清理跨天 relation，保留相邻天 relation 不动。实现复杂，且相邻天 relation 也可能受旧 bug 影响。全量重建更简单可靠。

### D4: BackfillSectionEmbeddings Phase 2 重写策略

**选择**：将 `BackfillSectionEmbeddings` Phase 2（relation 写入部分）改为调用 `BackfillRelations(boardID)`。

**理由**：当前 Phase 2 实现有三个根本性问题：
1. `LIMIT 1` 只找最近邻，丢失合法的多匹配（如 split 场景）
2. 没有日期方向检查，可能产生 from=晚天、to=早天的反向 relation
3. 没有按天分组，完全没有 skip-day 概念

修补现有代码工作量大且容易遗漏。改为 Phase 1 补 embedding 后，Phase 2 直接按 board 调用 `BackfillRelations`，复用统一的过滤逻辑。

### D5: 批量回刷入口

**选择**：除了 `BackfillRelations(boardID)` 单 board 回刷外，新增 `BackfillAllRelations()` 遍历所有 board 逐个执行回刷。

**理由**：噪声存在于所有 board，不仅限于 2853。单 board 函数用于定向修复，批量入口用于一次性全量清理。

### D6: 中间天延续检查的性能优化

**选择**：在 `BackfillRelations` 和改写后的 `MatchAndSaveRelations` 中，一次性加载 board 下所有已有 relation 到内存，构建出邻接表（from_section_id → []to_section），中间天检查转为内存查询。

**备选**：每个跨天匹配都执行 SQL 查询 from_section 的出边。简单但产生 N×M 次查询，board 数据量增长后不可接受。

**理由**：单 board 的 relation 数量通常在几十到几百条，内存占用可忽略。邻接表查找 O(1) 比逐条 SQL 快几个数量级。

## Risks / Trade-offs

- **[隔天续上仍可能产生噪声]** → 0.25 阈值下 700→934(0.215) 会被保留，但"内塔尼亚胡批评本-格维尔"和"特朗普与内塔尼亚胡关系破裂"确实有叙事关联，保留可接受。如果后续发现仍偏多可进一步收紧到 0.20。
- **[回刷期间数据短暂缺失]** → 回刷是删除+重建，中间时刻 relation 表为空。在低峰期执行或加事务包裹可缓解。单 board 数据量小（~60 条 relation），执行秒级。
- **[多 board 回刷时间]** → 如果需要全量回刷所有 board，需要遍历所有 board 逐个执行。可接受，一次性操作。
- **[回刷后 ending 状态变化]** → 回刷后 relation 拓扑显著变化，部分原本无出边的 section 可能获得出边（因保留跨天关系）或失去出边（因过滤噪声）。`DeriveSectionStatuses` 中 `ending` 的覆盖逻辑（无出边 + 非最新天）在新的拓扑下仍正确运行，但 section 的 ended 分布会变化——需在验证阶段确认无异常。
