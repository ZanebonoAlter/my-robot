## MODIFIED Requirements

### Requirement: 关系写入逻辑（含 distance 修复）
`SaveReport()` 事务中，新 sections 写入数据库后（含两阶段合并），系统 SHALL 对每个合并后的 section 用 embedding 查询同 board 下所有非当日已完成 section，找出匹配后按以下规则写入 relation（from=旧 section, to=新 section, distance）：

**匹配结果按天分组后，分两层过滤：**

1. **相邻天匹配**：from_section 的日期和 to_section 的日期之间无其他已完成报告的天。distance < 0.35 → 直接写入。

2. **跨天匹配**：from_section 的日期和 to_section 的日期之间有至少一天有已完成报告。需同时满足：
   - from_section 在中间天无任何延续关系（即 from_section 没有指向中间天 section 的已写入 relation）
   - distance < 0.25

embedding 为空的 section SHALL 跳过关系写入。新 section 的 embedding 仍基于 `cluster_label` 文本生成，逻辑不变。

distance 字段 SHALL 正确存储实际计算的距离值（非零值）。写入方式 SHALL 使用 raw SQL `INSERT ... ON CONFLICT DO UPDATE` 确保 distance 在 upsert 时也被正确写入。

**回刷支持**：系统 SHALL 提供 `BackfillRelations(boardID)` 函数，清理指定 board 的所有 relation 并按日期从早到晚逐个 section 重新执行上述写入逻辑（确保中间天判断基于已写入的 relation）。

#### Scenario: 单个相邻天匹配写入关系
- **WHEN** 新 section "开发者 Agent 工具链进入平台化竞争" (embedding=E1, date=6/2) 存入，同 board 下 6/1 已有 section "AI Agent 生态进入平台化竞争" (id=100, embedding=E2, date=6/1)，E1<=>E2=0.28，6/1 和 6/2 之间无其他已完成报告
- **THEN** 系统 SHALL 插入 relation (from=100, to=新section_id, distance=0.28)（相邻天，dist < 0.35，直接写入）

#### Scenario: 跨天匹配写入——无中间天延续（真隔天续上）
- **WHEN** 新 section (date=6/3) 匹配旧 section #80 (date=6/1, distance=0.094)，6/1 和 6/3 之间有 6/2（有已完成报告），但 section #80 在 6/2 无任何延续关系
- **THEN** 系统 SHALL 插入 relation (from=80, to=新id, distance=0.094)（跨天，无中间天延续，dist < 0.25，写入）

#### Scenario: 跨天匹配过滤——有中间天延续（冗余噪声）
- **WHEN** 新 section (date=6/3) 匹配旧 section #85 (date=6/1, distance=0.213)，6/1 和 6/3 之间有 6/2（有已完成报告），且 section #85 在 6/2 已有延续关系（#85→某 6/2 section）
- **THEN** 系统 SHALL 不插入该 relation（跨天，有中间天延续，过滤掉）

#### Scenario: 跨天匹配过滤——距离超阈值
- **WHEN** 新 section (date=6/3) 匹配旧 section #90 (date=6/1, distance=0.27)，section #90 在 6/2 无延续关系
- **THEN** 系统 SHALL 不插入该 relation（跨天，无中间天延续，但 dist ≥ 0.25，过滤掉）

#### Scenario: 多个相邻天匹配写入多条关系（split）
- **WHEN** 新 section 与相邻天旧 section #80 (distance=0.20) 和 #85 (distance=0.30) 均低于阈值
- **THEN** 系统 SHALL 插入两条 relation：(from=80, to=新id, distance=0.20) 和 (from=85, to=新id, distance=0.30)

#### Scenario: 距离超阈值不写关系
- **WHEN** 新 section 与同 board 所有旧 section 的 cosine distance 均不满足过滤条件（相邻天 ≥ 0.35，跨天 ≥ 0.25 或有中间天延续）
- **THEN** 系统 SHALL 不插入任何 relation，该 section status 为 emerging，ended 由后续判断决定

#### Scenario: 空 embedding 跳过
- **WHEN** 新 section 的 cluster_label 为空导致 embedding 为 NULL
- **THEN** 系统 SHALL 跳过该 section 的关系写入

#### Scenario: 日期不连续时视为相邻天
- **WHEN** board 只有 6/1 和 6/3 的报告（6/2 无报告），新 section (date=6/3) 匹配旧 section (date=6/1, distance=0.30)
- **THEN** 6/1 和 6/3 之间无其他已完成报告，SHALL 视为相邻天匹配，distance < 0.35 直接写入

#### Scenario: BackfillSectionEmbeddings 改写
- **WHEN** 运行 `BackfillSectionEmbeddings` 回填历史 section 的 embedding
- **THEN** Phase 1 补 embedding 后，Phase 2 SHALL 对每个 board 调用 `BackfillRelations(boardID)` 来写入 relation（复用统一过滤逻辑），而非原有的 `LIMIT 1` 最近邻逻辑

#### Scenario: BackfillRelations 全量回刷
- **WHEN** 运行 `BackfillRelations(boardID=2853)`
- **THEN** 系统 SHALL 删除 board 2853 涉及的所有 relation 记录，按日期从早到晚逐个 section 重新执行带过滤的写入逻辑

#### Scenario: BackfillAllRelations 批量回刷
- **WHEN** 运行 `BackfillAllRelations()`
- **THEN** 系统 SHALL 查询所有有 embedding section 的 board，逐个调用 `BackfillRelations(boardID)` 进行全量回刷
