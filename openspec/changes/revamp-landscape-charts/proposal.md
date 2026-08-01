# Proposal: revamp-landscape-charts

## Why

「话题态势版图」的可视化目前全是手写 CSS/SVG：话题卡片内的「近 30 日命中节奏条」（`MiniLifeline.vue`）是 5px 宽、高度全同、仅靠 5 档色阶区分的 div 小棍，数值高低完全看不出来；「新冒头」分组动辄上百张卡片各带一条，满屏重复噪点（"图太多"）。顶部活力折线（`VitalityBar.vue`）是手算坐标的无轴 polyline。项目尚无图表库，继续手写 CSS 天花板太低。

## What Changes

- 引入 **ECharts**（`echarts` 模块化按需引入，Canvas 渲染，自研轻封装，不引 vue-echarts）。
- **新增「话题节奏总览气泡图」**：一张图聚合全部话题的命中节奏——x 轴=日期（近 N 日）、y 轴=话题（按态势分组序 + hit_count 排序）、气泡大小∝当日 section_count、颜色=态势（stance），y 轴支持滚动缩放，点击气泡跳「话题总览」聚焦该话题。它替代卡片小图成为节奏信息的主载体。
- **话题卡片 mini-lifeline 改造**：
  - `active`/`stalled`/`pending`/`archived` 卡片：CSS 色阶条 → ECharts 迷你柱状图（**高度编码数值**，hover tooltip 显示「日期：N 节」），空日渲染 0 高度柱保持日期轴连续。
  - `emerging`（新冒头）卡片：**不再渲染节奏条**（命中 1-2 次的话题节奏条零信息量，是纯噪点），卡片保留 label/关键数字/最近命中。
  - `archived` 分组默认折叠，图表在展开时才初始化（懒挂载）。
- **活力顶栏折线升级**：手算 polyline → ECharts 面积图（带轻量坐标轴/网格/tooltip），指标数字行不变。
- 全部图表跟随亮/暗主题切换（读 CSS 变量构建 option，主题变更时重设）。
- 纯前端改动：复用既有 `GET /semantic-boards/:id/topic-landscape` 响应（topics[].lifeline + vitality.trend 已含全部所需数据），**后端零改动**。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `board-topic-landscape`：
  - 「话题卡片 mini-lifeline」Requirement 变更：渲染方式从 CSS 色阶格点改为 ECharts 高度编码迷你柱图；emerging 卡片移除节奏条。
  - 「活力顶栏」Requirement 变更：缩略折线从手写 SVG polyline 改为 ECharts 面积图。
  - 新增 Requirement「话题节奏总览气泡图」：态势区顶部一张聚合气泡图承载全部话题节奏。

## Impact

- **前端**：`front/app/features/tags/components/topic-landscape/`（新增 `TopicRhythmChart.vue`、`MiniLifelineChart.vue`，改造 `TopicStanceCard.vue`/`StanceCardWall.vue`/`VitalityBar.vue`/`TopicLandscapePanel.vue`，删除 `MiniLifeline.vue`）；新增 echarts option 纯函数模块 + `useEcharts` composable。
- **依赖**：`front/package.json` 新增 `echarts`（模块化引入，bundle 增量约 300-400KB）。
- **后端/API**：零改动。
- **文档**：`docs/reference/flow/semantic-board.md`、`docs/reference/architecture/frontend.md`（图表库选型）、`docs/reference/standard/frontend/`（如图表封装约定）。
- **测试**：Vitest 单测覆盖 option 构建纯函数（happy-dom 无 canvas，不直接测 echarts 渲染）；opencli 端到端按需验证视觉/交互。
