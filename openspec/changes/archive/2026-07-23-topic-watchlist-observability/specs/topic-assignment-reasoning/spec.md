## ADDED Requirements

### Requirement: 归属理由数据持久化

`daily_report_sections` SHALL 持久化 `matched_topic_id` 列（LLM 在 ClusterTags 阶段选择的 persistent_topic id）。该列当前为瞬态字段（`gorm:"-"`，用完即弃），本变更 SHALL 将其转为持久化列，使"AI 选了哪个话题"可追溯。

持久化 SHALL 通过显式版本化数据库迁移完成（按开发执行规范 §10），列 SHALL 允许 NULL（兼容历史 section）。`topic_match_distance` 与 `topic_match_confidence` 两列已持久，本变更不改动其定义。

历史 section 的 `matched_topic_id` SHALL 保持 NULL（不做回填），仅对变更上线后的新日报生效。

#### Scenario: 新日报写入 LLM 选择

- **WHEN** 新日报生成，某 section 经 ClusterTags 后 LLM 标记 matched_topic_id=8，且归属判定为 anchor_hit
- **THEN** 系统 SHALL 在该 section 行持久化 `matched_topic_id=8`

#### Scenario: auto_new 时的 matched_topic_id

- **WHEN** 某 section 归属为 auto_new（开新 candidate）
- **THEN** 系统 SHALL 持久化 `matched_topic_id=NULL`（LLM 未指向已有 topic）

#### Scenario: 历史 section 不回填

- **WHEN** 变更上线后查询变更前的历史 section
- **THEN** 其 `matched_topic_id` SHALL 为 NULL

### Requirement: 归属理由 API 暴露

section / timeline / lifeline 相关接口 SHALL 在 section 表示中暴露归属理由三元组：`topic_match_distance`、`topic_match_confidence`、`matched_topic_id`（可空）。

#### Scenario: section 详情含理由

- **WHEN** 前端请求日报 section 列表或话题生命线
- **THEN** 每个 section SHALL 返回 `topic_match_distance` / `topic_match_confidence` / `matched_topic_id`

### Requirement: 话题泳道节点按置信度分层渲染

`BoardThreadBrowser`（话题总览）在 timeline 与 lanes 视图 SHALL 按 section 的 `topic_match_confidence` 与 `topic_match_distance` 对节点分样式，使可疑归属一眼可见：

- `anchor_hit` 且距离远低于阈值 → **实心**（高置信）
- `anchor_hit` 但距离接近阈值（边界命中）→ **半实心**（可疑，需留意）
- `auto_new` → **空心**（新候选）
- `unmatched` → 按未归属样式

"边界命中"的具体距离比例（如 `distance > match_threshold × 0.85`）SHALL 在 design.md 确定，SHALL 可配置。

#### Scenario: 高置信命中实心

- **GIVEN** section 的 `topic_match_confidence=anchor_hit`，`topic_match_distance` 远低于阈值
- **WHEN** 渲染该节点
- **THEN** 节点 SHALL 显示为实心样式

#### Scenario: 边界命中半实心

- **GIVEN** section 的 `confidence=anchor_hit`，但 `distance` 落在边界区间
- **WHEN** 渲染该节点
- **THEN** 节点 SHALL 显示为半实心样式以提示可疑

#### Scenario: 新候选空心

- **GIVEN** section 的 `confidence=auto_new`
- **WHEN** 渲染该节点
- **THEN** 节点 SHALL 显示为空心样式

### Requirement: 归属理由 hover 气泡

`BoardThreadBrowser` 节点 SHALL 在 hover 时显示归属理由气泡，将原始数值翻译为人话理由，并同时保留原始数值供进阶查看：

- 人话理由 SHALL 说明"与某话题相似 X%，AI 是否认同"
- 气泡 SHALL 同时展示 `topic_match_distance`、`topic_match_confidence`、`matched_topic_id` 原始值

#### Scenario: hover 显示人话理由

- **WHEN** 用户 hover 一个 anchor_hit 节点（distance=0.27，matched_topic_id=8）
- **THEN** 气泡 SHALL 显示形如"与『以黎冲突升级』约 73% 相关，AI 也认同"的人话理由，并附原始数值

#### Scenario: 两信号不一致时提示

- **WHEN** 用户 hover 一个 auto_new 节点（embedding 命中但 LLM 未指向）
- **THEN** 气泡 SHALL 提示"相似度接近但 AI 未确认，已开新候选"并附原始数值

### Requirement: 话题详情信度列表

点击话题泳道中的话题，侧栏（或详情面板）SHALL 展示该话题的全部历史 section，每条标注其归属置信度（anchor_hit / auto_new）与 distance，复用 `getTopicLifeline` 聚合。

#### Scenario: 话题历史含信度

- **WHEN** 用户点击话题「以黎冲突升级」
- **THEN** 详情 SHALL 列出该话题下全部历史 section，每条标注 `topic_match_confidence` 与 `topic_match_distance`
