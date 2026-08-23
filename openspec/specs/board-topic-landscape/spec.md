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

`active`/`stalled`/`pending`/`archived` 话题卡片 SHALL 内嵌一条近 N 日（默认 30）命中节奏迷你柱状图（ECharts），数据源为 identity 轨聚合（`section.persistent_topic_id` 按日计数）。`emerging` 话题卡片 SHALL NOT 渲染节奏图（其节奏信息由「话题节奏总览气泡图」承载）。

#### Scenario: 柱高编码数值

- **THEN** 每个日历日一根柱，柱高 SHALL 与该日该话题的 `section_count` 正相关
- **AND** 空日（`section_count=0`）SHALL 渲染为 0 高度占位（日期轴连续可见），SHALL NOT 跳过该列
- **AND** hover tooltip SHALL 显示「日期：N 节」

#### Scenario: 日期轴连续

- **THEN** 迷你柱状图日期轴 SHALL 连续（由后端 `generate_series` 补空日），即使该 board 某日无日报

#### Scenario: 卡片关键数字

- **THEN** 卡片 SHALL 显示 label、stance 图标、`hit_count`/`consecutive_hits` 或「N 天未命中」（停滞态）

#### Scenario: emerging 卡片无节奏图

- **GIVEN** 话题 `stance=emerging`
- **THEN** 其卡片 SHALL NOT 渲染节奏图，SHALL 保留 label、命中数与最近命中日期

#### Scenario: 归档分组懒挂载

- **GIVEN** `archived` 分组默认折叠
- **THEN** 折叠状态下其卡片图表 SHALL NOT 初始化，展开后 SHALL 才挂载渲染

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
- **AND** SHALL 渲染近 N 日每日 section 数的面积折线图（ECharts），带轻量坐标轴与 tooltip
- **AND** `feed_active`（活跃信息源数）MVP 可为空，SHALL NOT 因缺它而隐藏整栏

#### Scenario: 主题跟随

- **WHEN** 用户切换亮/暗主题
- **THEN** 面积图配色 SHALL 跟随主题重设

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

### Requirement: 话题节奏总览气泡图

态势区 SHALL 在活力顶栏之下、分区卡片墙之上渲染一张「话题节奏总览」气泡图，聚合全部可见话题的近 N 日命中节奏，作为节奏信息的主载体。

#### Scenario: 气泡映射

- **THEN** x 轴 SHALL 为近 N 日连续日期（category），y 轴 SHALL 为话题（category，inverse）
- **AND** 每个 `(话题, 日期, section_count > 0)` 渲染一个气泡，气泡面积 SHALL 与 `section_count` 正相关（sqrt 缩放并 clamp 到可读像素区间）
- **AND** `section_count = 0` 的日期 SHALL NOT 渲染气泡

#### Scenario: 话题排序与态势着色

- **THEN** y 轴话题排序 SHALL 与卡片墙一致：stance 分组序 `active` → `stalled` → `emerging` → `pending` → `archived`，组内 `hit_count DESC`
- **AND** 气泡 SHALL 按 stance 分 series 着色，并渲染 legend 支持按态势过滤
- **AND** `archived` series SHALL 默认不显示（legend unselected），用户可手动开启

#### Scenario: 话题过多时滚动缩放

- **GIVEN** 可见话题数超过单屏可读容量
- **THEN** y 轴 SHALL 提供 dataZoom（滚轮 + slider），默认窗口显示排序靠前的若干话题

#### Scenario: 点击气泡跳转

- **WHEN** 用户点击某气泡
- **THEN** SHALL 触发与话题卡片点击相同的链路：切到「话题总览」tab 并聚焦该 topic

#### Scenario: 主题跟随

- **WHEN** 用户切换亮/暗主题
- **THEN** 气泡图配色 SHALL 跟随主题重设，SHALL NOT 残留上一主题颜色

