## Purpose

管理 DailyReportSection 之间的多对多跨天关系，通过 `daily_report_section_relations` 关系表存储。支持同日 section 两阶段合并（embedding 确定性合并 + LLM 仲裁），以及跨日 section embedding 匹配写入关系。

## Requirements

### Requirement: 同日 Section 两阶段合并
`SaveReport()` 事务中，新 sections 写入数据库后、relations 写入前，系统 SHALL 对同日 sections 执行两阶段合并以消除聚类过碎问题。

**Stage 1：确定性合并（embedding）**
系统 SHALL 计算所有同日 section pairs 的 embedding cosine distance。distance < 0.20 的 pairs SHALL 自动合并为一个 section。

合并规则：
- 保留 `article_count` 最大的 section 作为主 section
- 被合并 section 的 threads SHALL 迁移到主 section 下
- 主 section 的 `cluster_label` 不变
- `cluster_tag_ids` SHALL 合并两个 section 的 tag IDs
- `article_count`、`best_tier`、`avg_score` SHALL 重新计算（合并后的值）
- 被合并 section SHALL 从 sections 列表中移除

连通性：如果 A↔B 和 B↔C 都 < 0.20，则 A、B、C SHALL 合并为一个 section（使用传递闭包）。

**Stage 2：LLM 仲裁（灰色地带）**
距离在 0.20 - 0.25 之间的 pairs SHALL 批量送 LLM 判断是否合并。

LLM 输入：每个 candidate pair 的 `(section_a_label, section_a_tag_labels[], section_b_label, section_b_tag_labels[])`。
LLM 输出：`merge_pairs: [[index_a, index_b], ...]` 列表。
LLM 判定为合并的 pairs SHALL 按 Stage 1 相同规则合并。

合并完成后，系统 SHALL 继续 relation 写入逻辑（基于合并后的 sections）。

#### Scenario: Stage 1 确定性合并
- **WHEN** sections [A(0.117↔B), B, C] 中 A↔B distance=0.117
- **THEN** 系统 SHALL 合并 A 和 B 为一个 section（保留 article_count 更大的），移除另一个

#### Scenario: 传递闭包合并
- **WHEN** sections [A, B, C] 中 A↔B=0.15, B↔C=0.18，但 A↔C=0.22
- **THEN** A、B、C SHALL 全部合并为一个 section

#### Scenario: Stage 2 LLM 仲裁合并
- **WHEN** sections [A, B] distance=0.21，LLM 判定为 merge=true
- **THEN** 系统 SHALL 合并 A 和 B

#### Scenario: Stage 2 LLM 仲裁拒绝合并
- **WHEN** sections [A, B] distance=0.23，LLM 判定为 merge=false
- **THEN** 系统 SHALL 保留 A 和 B 为独立 section

#### Scenario: 无灰色地带 pairs
- **WHEN** 同日所有 section pairs 的 distance 均 < 0.20 或 > 0.25
- **THEN** 系统 SHALL 跳过 Stage 2，不调用 LLM

### Requirement: Section 关系表
系统 SHALL 创建 `daily_report_section_relations` 表，存储 Section 之间的多对多跨天关系。

字段：id (SERIAL PK), from_section_id (INT NOT NULL REFERENCES daily_report_sections(id)), to_section_id (INT NOT NULL REFERENCES daily_report_sections(id)), distance (FLOAT NOT NULL), created_at (TIMESTAMP DEFAULT NOW())。

约束：
- `from_section_id` 指向较早日期的 section，`to_section_id` 指向较晚日期的 section
- `(from_section_id, to_section_id)` SHALL 有 UNIQUE 约束防止重复关系

索引：
- `idx_section_relations_from` ON (from_section_id)
- `idx_section_relations_to` ON (to_section_id)

#### Scenario: 创建关系表
- **WHEN** 数据库迁移运行
- **THEN** 系统 SHALL 创建 `daily_report_section_relations` 表，包含上述字段、约束和索引

#### Scenario: 防止重复关系
- **WHEN** 尝试插入 (from=10, to=20, distance=0.25) 而该关系已存在
- **THEN** 数据库 SHALL 拒绝插入

### Requirement: 关系写入逻辑（含 distance 修复）
`SaveReport()` 事务中，新 sections 写入数据库后（含两阶段合并），系统 SHALL 对每个合并后的 section 用 embedding 查询同 board 下所有非当日已完成 section，找出匹配后按以下规则写入 relation（from=旧 section, to=新 section, distance）：

**匹配结果按天分组后，分两层过滤：**

1. **相邻天匹配**：from_section 的日期和 to_section 的日期之间无其他已完成报告的天。distance < 0.35 → 直接写入。

2. **跨天匹配**：from_section 的日期和 to_section 的日期之间有至少一天有已完成报告。需同时满足：
   - from_section 在中间天无任何延续关系（即 from_section 没有指向中间天 section 的已写入 relation）
   - distance < 0.25

embedding 为空的 section SHALL 跳过关系写入。新 section 的 embedding 仍基于 `cluster_label` 文本生成，逻辑不变。

distance 字段 SHALL 正确存储实际计算的距离值（非零值）。写入方式 SHALL 使用 raw SQL `INSERT ... ON CONFLICT DO UPDATE` 确保 distance 在 upsert 时也被正确写入。

**竞争过滤**：通过时间维度过滤后的候选 relation，SHALL 经过竞争过滤后才写入 DB。对每个 section 的所有候选按 distance 升序排列：
- 若候选数为 0 或 1 → 原样通过
- 若 best 与 2nd 的 gap ≥ 0.03 → 只保留 best
- 若 gap < 0.03 → 保留所有 distance ≤ best + 0.03 的候选（真正的 split/merge）

竞争过滤 SHALL 作为纯函数 `competitiveFilter(candidates) []matchCandidate` 实现，无 DB 依赖，在 `shouldWriteRelation` 之后、写入 DB 之前调用。

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
- **WHEN** 新 section 与相邻天旧 section #80 (distance=0.20) 和 #85 (distance=0.22) 均低于阈值，两者都通过了 `shouldWriteRelation`
- **AND** 竞争过滤中 best=0.20, 2nd=0.22, gap=0.02 < 0.03
- **THEN** 系统 SHALL 插入两条 relation（gap < 0.03，保留 split）：(from=80, to=新id, distance=0.20) 和 (from=85, to=新id, distance=0.22)

#### Scenario: 竞争过滤——gap ≥ 0.03 只保留 best
- **WHEN** 新 section 通过 `shouldWriteRelation` 后有 3 条候选：#A (distance=0.15), #B (distance=0.22), #C (distance=0.30)
- **AND** best=0.15, 2nd=0.22, gap=0.07 ≥ 0.03
- **THEN** 系统 SHALL 只插入 1 条 relation：(from=#A, to=新id, distance=0.15)

#### Scenario: 竞争过滤——gap < 0.03 保留多候选
- **WHEN** 新 section 通过 `shouldWriteRelation` 后有 4 条候选：#A (distance=0.20), #B (distance=0.22), #C (distance=0.24), #D (distance=0.28)
- **AND** best=0.20, 2nd=0.22, gap=0.02 < 0.03
- **THEN** 系统 SHALL 保留 distance ≤ 0.20 + 0.03 = 0.23 的候选，即 #A (0.20) 和 #B (0.22)
- **AND** 系统 SHALL 不插入 #C (0.24 > 0.23) 和 #D (0.28 > 0.23) 的 relation

#### Scenario: 竞争过滤——单候选直接通过
- **WHEN** 新 section 通过 `shouldWriteRelation` 后只有 1 条候选
- **THEN** 系统 SHALL 直接插入该 relation，不执行竞争过滤

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

### Requirement: Section status 动态推导（两阶段）
系统 SHALL 在 timeline/lifecycle API 返回时动态推导 section 的 `status`（关系状态）和 `ended`（结束标记），不存储在数据库中。

**阶段一：关系状态（status）**——描述该 section 与前驱的关系（按优先级从高到低）：
1. 无 from 关系（无 relation 的 to_section_id 指向它）→ `emerging`
2. 有多个 from 关系（to 入度 > 1，多个旧 section 指向它）→ `merge`
3. from 的出度 > 1（前驱 section 还被其他新 section 指向，该 section 是分化出的子叙事之一）→ `split`
4. 有 from 关系且 from 出度 = 1 → `continuing`

注意：一个 section 可以同时是 split 和 merge。取优先级 merge > split > continuing。

**阶段二：结束标记（ended）**——描述该 section 是否无后继：
- 无 to 关系（无 relation 的 from_section_id 指向它）且 不是最新一天 → `ended = true`
- 最新一天的 section 即使无 to 关系 SHALL NOT 被标记为 ended

#### Scenario: 新兴叙事
- **WHEN** section #50 无任何 relation 的 to_section_id 指向它
- **THEN** status SHALL 为 `emerging`，ended SHALL 为 false

#### Scenario: 延续叙事
- **WHEN** section #60 的唯一 from relation 指向 section #50，且 section #50 只有 #60 这一个 to relation
- **THEN** section #60 的 status SHALL 为 `continuing`

#### Scenario: 叙事分化
- **WHEN** section #50 被 section #60 和 #61 同时指向（#50 的出度 = 2）
- **THEN** section #60 和 #61 的 status SHALL 为 `split`（它们是从 #50 分化出的子叙事）

#### Scenario: 叙事合并
- **WHEN** section #70 被 section #50 和 #55 同时指向（to 入度 = 2）
- **THEN** section #70 的 status SHALL 为 `merge`

#### Scenario: 延续后结束
- **WHEN** section #50 在 05-25（有 from 关系，status=continuing），无任何 relation 的 from_section_id 指向它（无后续），且时间范围最新天为 05-28
- **THEN** section #50 的 status SHALL 为 `continuing`，ended SHALL 为 true

#### Scenario: 新兴后结束（一日叙事）
- **WHEN** section #40 在 05-25（无 from 关系，status=emerging），无后续 relation，且时间范围最新天为 05-28
- **THEN** section #40 的 status SHALL 为 `emerging`，ended SHALL 为 true

#### Scenario: 最新一天不标记 ended
- **WHEN** section #80 在最新一天 05-28，无后续 relation
- **THEN** section #80 的 ended SHALL 为 false（无论 status 是什么）

### Requirement: Section 关系查询 API
系统 SHALL 提供 `GET /api/semantic-boards/:id/section-timeline?days=14` 端点，返回该板块最近 N 天所有 section 的扁平列表（含动态推导的 status），以及所有 section 间关系。

响应格式：
```json
{
  "sections": [SectionTimelineNode...],
  "relations": [{"from_id": 50, "to_id": 60, "distance": 0.25}, ...]
}
```

SectionTimelineNode SHALL 包含 id、report_id、period_date、cluster_label、status（动态推导，emerging/continuing/split/merge）、ended（boolean，动态推导）、article_count、thread_count。

#### Scenario: 查询板块 section 时间线和关系
- **WHEN** 请求 `GET /api/semantic-boards/5/section-timeline?days=14`
- **THEN** 系统 SHALL 返回 board #5 最近 14 天的所有 section 和它们之间的关系

#### Scenario: 无数据返回空
- **WHEN** 板块无 section
- **THEN** 系统 SHALL 返回 `{sections: [], relations: []}`

### Requirement: Section Lifecycle API 适配关系表
`GET /api/daily-reports/sections/:id/lifecycle` SHALL 改为基于关系表查询。返回目标 section 的完整关系网络：沿 from 关系向前追溯，沿 to 关系向后扩展，返回所有相关 section（含动态推导的 status 和 ended）和 relation。

#### Scenario: 查询 section 生命周期含分叉
- **WHEN** 请求 `GET /api/daily-reports/sections/60/lifecycle`，section #60 from → #50，#50 同时被 #60 和 #61 指向
- **THEN** 系统 SHALL 返回 sections [#50, #60, #61] 和 relations [{from:50,to:60}, {from:50,to:61}]

#### Scenario: 孤立 section
- **WHEN** 请求 `GET /api/daily-reports/sections/20/lifecycle`，section #20 无任何 relation
- **THEN** 系统 SHALL 返回 sections [#20] 和空 relations
