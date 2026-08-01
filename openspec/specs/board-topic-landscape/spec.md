# board-topic-landscape Specification

## Purpose
TBD - created by archiving change board-topic-landscape. Update Purpose after archive.
## Requirements
### Requirement: 话题态势版图入口

「板块内容」tab（`BoardCompositionPanel`）SHALL 在构成标签管理区下方承载「话题态势版图」区，作为板块选中后的默认首屏统计概览，且该区 SHALL 只读展示态势。

#### Scenario: 默认展示态势区

- **GIVEN** 用户选中某板块且切到「板块内容」tab
- **THEN** 下方 SHALL 渲染「话题态势版图」区，与上方构成标签管理区 SHALL 有视觉分隔

#### Scenario: 管理区与态势区语义不混

- **THEN** 态势区 SHALL 只读展示，不含任何构成标签增删操作；管理区操作 SHALL NOT 触发态势区重算以外的副作用

### Requirement: 态势派生基于 identity 轨

态势标签 SHALL 完全基于 identity 轨（持久话题泳道）字段派生，SHALL NOT 读取 similarity 轨（`daily_report_section_relations` 表的匈牙利配对边或 emerging/continuing/split/merge/ending 五态）。

#### Scenario: 主态势派生规则

- **GIVEN** 某持久话题的 identity 字段
- **THEN** 后端 SHALL 按如下顺序（首匹配）派生 `stance`：
  - `status='archived'` → `archived`
  - `status='candidate' AND hit_count >= upgrade_threshold` → `pending`
  - `status='candidate' AND hit_count < upgrade_threshold` → `emerging`
  - `status='active' AND consecutive_hits > 0 AND days_since(last_seen_date) <= N` → `active`
  - `status='active' AND (consecutive_hits = 0 OR days_since(last_seen_date) > N)` → `stalled`
- **WHERE** `N` = `topic_landscape_active_window_days`（默认 7）

#### Scenario: 强吸引叠加标记

- **GIVEN** 某话题 `is_vacuum=true`
- **THEN** 该话题 SHALL 额外携带 `is_vacuum=true` 与 `vacuum_strong` 标记，且可与 active/stalled 等主态势叠加，SHALL NOT 改变主态势判定

#### Scenario: 派生口径单一

- **THEN** `stance` SHALL 在后端接口派生，前端 SHALL NOT 重复实现派生规则

### Requirement: 态势分区卡片墙

话题 SHALL 按 `stance` 分组渲染为分区卡片墙。

#### Scenario: 分组与排序

- **THEN** 分组顺序 SHALL 为：`active` → `stalled` → `emerging` → `pending` → `archived`
- **AND** 每分组内话题 SHALL 按 `hit_count DESC` 排序

#### Scenario: 已归档分组折叠

- **GIVEN** 存在 `archived` 话题
- **THEN** 该分组 SHALL 默认折叠，SHALL 显示归档计数；用户可手动展开

#### Scenario: 无话题分组隐藏

- **GIVEN** 某 `stance` 分组无话题
- **THEN** 该分组 SHALL NOT 渲染

### Requirement: 话题卡片 mini-lifeline

每张话题卡片 SHALL 内嵌一条近 N 日（默认 30）命中节奏条，数据源为 identity 轨聚合（`section.persistent_topic_id` 按日计数）。

#### Scenario: 节奏条格点

- **THEN** 每个日历日一格，格深浅 SHALL 与该日该话题的 `section_count` 正相关
- **AND** 空日（`section_count=0`）SHALL 渲染为浅灰空格，SHALL NOT 跳过该列

#### Scenario: 日期轴连续

- **THEN** 节奏条日期轴 SHALL 连续（由后端 `generate_series` 补空日），即使该 board 某日无日报

#### Scenario: 卡片关键数字

- **THEN** 卡片 SHALL 显示 label、stance 图标、`hit_count`/`consecutive_hits` 或「N 天未命中」（停滞态）
- **AND** hover SHALL 显示近期 section 标题预览（可延后到 apply 阶段，若成本高可只显示数字）

### Requirement: 待激活话题引导

`stance=pending`（candidate 且达 `upgrade_threshold`）话题卡片 SHALL 视觉突出并引导转正。

#### Scenario: 红描边与角标

- **GIVEN** 话题 `stance=pending`
- **THEN** 卡片 SHALL 红色描边 + 角标「待激活」，hover 提示「够格转正，去话题管理激活」

#### Scenario: 不自动转正

- **THEN** 本视图 SHALL NOT 提供一键转正操作（转正属话题管理 UI 职责），SHALL 引导用户前往话题管理

### Requirement: 活力顶栏

态势区顶部 SHALL 渲染一行板块活力指标。

#### Scenario: 指标内容

- **THEN** SHALL 显示近 N 日 `article_count`、`section_count`、`active_topic_count`
- **AND** SHALL 渲染近 N 日每日 section 数的缩略折线
- **AND** `feed_active`（活跃信息源数）MVP 可为空，SHALL NOT 因缺它而隐藏整栏

### Requirement: 空态处理

板块无日报时 SHALL 渲染空态引导，SHALL NOT 渲染空版图骨架。

#### Scenario: 无日报引导生成

- **GIVEN** 板块在 `board_daily_reports` 无任何记录
- **THEN** 接口 SHALL 返回 `topics=[]` 且 `vitality.trend=[]`
- **AND** 前端 SHALL 渲染空态卡「该板块还没有日报…[生成日报]」

#### Scenario: 生成日报按钮

- **WHEN** 用户点击「生成日报」
- **THEN** SHALL 调用 `POST /api/daily-reports/generate`（`{board_id, date:今日}`），SHALL 经 WS 显示进度

### Requirement: 后端聚合接口

后端 SHALL 提供 `GET /api/semantic-boards/:id/topic-landscape?days=N`，返回板块全部可见持久话题的态势派生结果 + mini-lifeline + 活力指标。

#### Scenario: 入参与默认

- **THEN** `days` 缺省或 `<=0` SHALL 视为 30；合法值 7/14/30/90；其余值 SHALL clamp 到最近合法值
- **AND** 响应 SHALL 含 `topics[]`（每条含 stance/is_vacuum/hit_count/consecutive_hits/first_seen_date/last_seen_date/days_since_last/lifeline）与 `vitality`

#### Scenario: 可见话题过滤

- **THEN** 接口 SHALL 保留 `hit_count >= 1` 的全部话题（含 `emerging` candidate，即 candidate 且 hit∈[1,threshold)），SHALL ONLY 剔除 `hit_count = 0` 的纯 orphan（从未命中的手工候选）
- **AND** 态势版图口径 SHALL 与话题管理 UI **不同**（后者用 `FilterVisibleTopics` 剔 observing candidate；版图需展示 emerging 新苗头信号），SHALL NOT 复用 `FilterVisibleTopics`

#### Scenario: identity 轨只读

- **THEN** 接口 SHALL 只读 identity 字段，SHALL NOT 调用 `daily_report_assignment.go` / `daily_report_matching.go` / `daily_report_lane.go` 的任何写或重算逻辑

