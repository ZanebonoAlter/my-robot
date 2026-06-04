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
`SaveReport()` 事务中，新 sections 写入数据库后（含两阶段合并），系统 SHALL 对每个合并后的 section 用 embedding 查询同 board 下所有非当日已完成 section，找出所有 cosine distance < 0.35 的匹配，为每个匹配插入一条 relation（from=旧 section, to=新 section, distance）。

embedding 为空的 section SHALL 跳过关系写入。新 section 的 embedding 仍基于 `cluster_label` 文本生成，逻辑不变。

distance 字段 SHALL 正确存储实际计算的距离值（非零值）。写入方式 SHALL 使用 raw SQL `INSERT ... ON CONFLICT DO UPDATE` 确保 distance 在 upsert 时也被正确写入。

#### Scenario: 单个匹配写入关系
- **WHEN** 新 section "开发者 Agent 工具链进入平台化竞争" (embedding=E1) 存入，同 board 下已有 section "AI Agent 生态进入平台化竞争" (id=100, embedding=E2)，E1<=>E2=0.28
- **THEN** 系统 SHALL 插入 relation (from=100, to=新section_id, distance=0.28)

#### Scenario: 多个匹配写入多条关系（split）
- **WHEN** 新 section 与旧 section #80 (distance=0.20) 和 #85 (distance=0.30) 均低于阈值
- **THEN** 系统 SHALL 插入两条 relation：(from=80, to=新id, distance=0.20) 和 (from=85, to=新id, distance=0.30)

#### Scenario: 距离超阈值不写关系
- **WHEN** 新 section 与同 board 所有旧 section 的 cosine distance 均 ≥ 0.35
- **THEN** 系统 SHALL 不插入任何 relation，该 section status 为 emerging，ended 由后续判断决定

#### Scenario: 空 embedding 跳过
- **WHEN** 新 section 的 cluster_label 为空导致 embedding 为 NULL
- **THEN** 系统 SHALL 跳过该 section 的关系写入

#### Scenario: BackfillSectionEmbeddings 改写
- **WHEN** 运行 `BackfillSectionEmbeddings` 回填历史 section 的 embedding
- **THEN** Phase 2 的匹配结果 SHALL 写入 `daily_report_section_relations` 表，不再写 `prev_section_id` 和 `status`

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
