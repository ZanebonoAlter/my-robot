# Design: revamp-landscape-charts

## Context

「话题态势版图」（`front/app/features/tags/components/topic-landscape/`）当前可视化全部手写：

- `MiniLifeline.vue`：卡片内节奏条 = 5px 宽 div 色块，高度恒定，数值仅靠 0-4 五档背景色区分。`emerging` 分组常有上百话题（实测 113），每条 lifeline 1-2 个命中点，渲染出来是满屏相同的小灰条+小红棍，零信息量纯噪点。
- `VitalityBar.vue`：手算坐标的 SVG polyline，无轴无网格。
- 数据侧：`GET /semantic-boards/:id/topic-landscape` 已返回全部所需数据（`topics[].lifeline[{date, section_count}]` 后端 `generate_series` 补空日保证日期轴连续；`vitality.trend` 每日 article/section 数），**本 change 零后端改动**。
- 项目无图表库（无 echarts/d3/chart.js）；主题系统：`data-theme="editorial|dark"` on `<html>`，`useTheme()` 提供响应式 `currentTheme`，颜色定义在 `main.css` CSS 变量。

## Goals / Non-Goals

**Goals:**

- 一张 ECharts 气泡总览图承载全部话题的命中节奏（解决"图太多"——百条卡片小图聚合成一张图）。
- 活跃/停滞等少量重点卡片的节奏图换成高度编码的真图表（ECharts 迷你柱图 + tooltip）。
- emerging 卡片删除节奏条（降噪）。
- 活力顶栏折线升级为带轴面积图。
- 全部图表跟随亮/暗主题。
- option 构建逻辑纯函数化，Vitest 可测。

**Non-Goals:**

- 不改后端接口/聚合逻辑/数据模型。
- 不改「话题总览」tab 的手写 SVG DAG（`BoardThreadBrowser.vue`）——它表达话题演化关系，职能不同。
- 不引入 vue-echarts 等封装库；不建全站通用 chart 组件库（本期只服务 topic-landscape）。
- 不做气泡图的高级交互（框选、时间范围刷选等），仅点击跳转。

## Decisions

### D1: 引 `echarts` 模块化按需引入，自研轻封装，不引 vue-echarts

`echarts/core` + 按需注册 `BarChart`/`ScatterChart`/`LineChart` + `GridComponent`/`TooltipComponent`/`DataZoomComponent`/`LegendComponent` + `CanvasRenderer`。自研 `useEcharts(elRef)` composable（~40 行：`onMounted` init、`ResizeObserver` 自动 resize、`onBeforeUnmount` dispose、暴露 `setOption`；init 抽函数 + `watch(elRef)` 兜底——容器晚于 onMounted 挂载（v-if/延迟渲染）时自动 init，见 D6 时序坑）。

- **为什么不引 vue-echarts**：多一层依赖且其按需注册、主题注入同样要手写；自研 40 行完全可控，无封装黑盒。
- **为什么 Canvas 而非 SVG renderer**：气泡图数据点上千（118 话题 × 30 日），Canvas 性能稳；迷你柱图无交互重绘压力，同用 Canvas 保持一致。

### D2: option 构建逻辑全部抽成纯函数模块

新建 `topic-landscape/chart-options.ts`，导出三个纯函数：

- `buildRhythmOption(topics, dates, palette)` → 气泡总览图 option
- `buildMiniBarOption(lifeline, palette)` → 卡片迷你柱图 option
- `buildVitalityOption(trend, palette)` → 活力面积图 option

组件只负责挂载/传参/事件桥接。**理由**：happy-dom 无 canvas，单测无法渲染 echarts；纯函数可用 Vitest 直接断言 series/axis/映射正确性（符合 `standard/frontend/testing.md` 的"逻辑与渲染分离"惯例）。

### D3: 气泡总览图 `TopicRhythmChart.vue`（方案 C）

- **映射**：x=日期（category，后端连续的近 N 日）；y=话题（category，inverse，排序=stance 分组序 active→stalled→emerging→pending→archived，组内 hit_count DESC）；气泡大小∝`section_count`（sqrt 缩放，clamp 6~26px）；`section_count=0` 不画点。**y 轴只保留所选范围内至少有一个命中的话题**（lifeline 全 0 的话题不渲染——无气泡的空泳道行易被误读为「有节奏没画出来」）。
- **分 series 按 stance 着色**（5 个 series → 免费获得 legend 可点选过滤；`archived` 默认 legend unselected——已归档话题不是节奏关注点，与卡片墙"归档默认折叠"语义一致）。
- **话题过多**：y 轴 `dataZoom`（inside 滚轮 + 右侧 slider），默认窗口显示排序前 ~25 个话题。
- **点击气泡** → emit `selectTopic(topicId)`，TagsPage 切「话题总览」tab 并聚焦该话题（`BoardThreadBrowser` 新增 `focusTopicId` prop → `enterFocus` 进入 focus 专注视图；**不弹侦探墙**——spec 要求切 tab 聚焦，早期实现误用侦探墙已修正）。
- 位置：`VitalityBar` 之下、`StanceCardWall` 之上，作为态势区的节奏主视图。

### D4: 卡片迷你柱图 `MiniLifelineChart.vue`，emerging 去图（方案 A）

- `active`/`stalled`/`pending`/`archived` 卡片：ECharts 迷你柱状图——category x 轴取 lifeline 全部日期（空日=0 值柱，日期轴连续语义保留），柱高=section_count，无轴无网格，tooltip 显示「M/D：N 节」。
- `emerging` 卡片：**不渲染任何图表**（命中 1-2 次，节奏信息由总览气泡图承载）。
- `archived` 分组默认折叠，折叠时 `v-if` 不挂载图表（天然懒初始化）。
- 实测活跃+停滞+待激活一般 <15 个实例，内存/初始化开销可忽略。
- 删除 `MiniLifeline.vue`。

### D5: 主题适配——palette 从 CSS 变量读取

`chart-options.ts` 内 `readPalette(): ChartPalette` 用 `getComputedStyle(document.documentElement).getPropertyValue()` 读 `--color-accent`/`--color-text-muted` 等 + stance 语义色常量（绿/蓝灰/黄/红/灰，对齐 main.css 双主题取值）。组件 `watch(useTheme().currentTheme)` 变化后 `setOption(buildXxxOption(...))` 重建。

- **为什么读 CSS 变量而非硬编码色板**：主题色单一事实源在 `main.css`，硬编码会随主题迭代漂移。

### D6: 静态模块化 import，SSR 安全（不包 `<ClientOnly>`）

echarts 模块化 import 本身 SSR 安全（init 才触碰 DOM），init 放 `onMounted`，**模板不包 `<ClientOnly>`**。

- **时序坑（实测）**：`<ClientOnly>` 的插槽内容在自身 `onMounted` 后才异步渲染，父组件 `onMounted`（`useEcharts` init）执行时容器尚未挂载 → `elRef.value===null` → init 被静默跳过、永不重试（曾导致全部图表空白、无 JS 报错）。项目 `ssr:false`（纯 SPA），`<ClientOnly>` 本就多余；`onMounted` 自身保证 client-only（SSR 开启时服务端不执行 mounted）。
- 防回归：`useEcharts` 的 init 抽函数 + `watch(elRef)` 兜底（容器晚于 onMounted 挂载时自动 init）。
bundle 增量约 350KB min / ~110KB gzip，记为接受的代价；不做动态 `import()` 延迟加载（YAGNI，若 build 告警再议）。

## Risks / Trade-offs

- [happy-dom 无 canvas，echarts 组件无法单测渲染] → option 纯函数化（D2），组件层靠 opencli 端到端按需验证。
- [去掉 emerging 卡片节奏条后单卡信息变少] → 该信息本就零价值（1-2 命中点）；节奏洞察由总览气泡图承载，卡片保留命中数/最近命中日期。
- [bundle 增大 ~110KB gzip] → 模块化按需引入已是最小集；tags 是核心页非首屏落地页，可接受。
- [getComputedStyle 读到的颜色与 SSR 首帧不一致] → 图表全部 `onMounted` 后才 init，主题已由 nuxt.config 内联脚本在水合前设置，无闪烁窗口。
- [气泡图 y 轴 100+ 话题拥挤] → dataZoom 默认窗口 25 + 滚轮缩放 + legend 过滤组合缓解。

## Migration Plan

纯前端改动，无数据/状态迁移。部署后用户直接看到新版图。回滚 = revert 本 change + 卸载 echarts 依赖。

## Open Questions

- 气泡图默认 y 窗口 25 是否合适 → apply 阶段按真实数据（最大板块 118 话题）目测调优，可在任务内微调。
- archived series 默认隐藏是否在图上也保留"显示归档"的 legend 入口 → 保留 legend（默认 unselected），用户可点开。
