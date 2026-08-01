# board-topic-landscape Specification Delta

## ADDED Requirements

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

## MODIFIED Requirements

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

### Requirement: 活力顶栏

态势区顶部 SHALL 渲染一行板块活力指标。

#### Scenario: 指标内容

- **THEN** SHALL 显示近 N 日 `article_count`、`section_count`、`active_topic_count`
- **AND** SHALL 渲染近 N 日每日 section 数的面积折线图（ECharts），带轻量坐标轴与 tooltip
- **AND** `feed_active`（活跃信息源数）MVP 可为空，SHALL NOT 因缺它而隐藏整栏

#### Scenario: 主题跟随

- **WHEN** 用户切换亮/暗主题
- **THEN** 面积图配色 SHALL 跟随主题重设
