## Why

当前日报详情仍以 1100px 定宽弹窗呈现，宽屏空间利用率低，并大量使用硬编码浅色样式，无法可靠适配 editorial/dark 双主题。持久话题、话题生命线和 thread 文章关系已经具备稳定数据能力，现在适合把日报升级为沉浸式、可追踪的长篇阅读界面。

## What Changes

- 将 `BoardDailyReportTimeline` 的详情从定宽纸张弹窗升级为占满 viewport 的全屏阅读层，保留关闭、Esc、日期切换和滚动控制。
- 采用项目 Editorial Magazine 设计语言重组日报：产品化刊头、头条、highlights、sticky 目录/话题/历史日期边栏和按话题分区的正文。
- 所有颜色、阴影和 surface 改用现有 `--color-*` / `--shadow-*` 语义 token，同时支持 editorial 与 dark 主题。
- active/candidate/unassigned section 继续按现有 `qualityZones` 契约分区；active 话题卡支持原位展开横向 mini 生命线。
- 生命线调用现有 `getTopicLifeline(topicId)`，仅用 identity relation 绘制跨日贝塞尔连线；节点可展开当日 threads。
- thread 在原位展开 `related_article_ids` 对应文章，文章选择继续触发 `openArticle(articleId)`。
- 保留进入侦探墙完整生命线的出口；仅在存在持久话题且当前设备支持该入口时展示。
- 1100px 以下退化为单栏，窄屏取消 sticky 边栏并保证主题、阅读和交互可用。
- 增加组件级状态/数据映射测试和关键浏览器流程回归，覆盖双主题及 1440+/1100-/720- 三档布局。
- 不新增日报后端、数据库迁移或独立路由；依赖 `2026-06-19-persistent-topic` 已提供的 topic brief、lifeline 与 identity relation。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `daily-report-system`: 修改日报详情的展示、响应式、主题、话题生命线和 thread 文章展开交互要求。

## Impact

- 主要修改 `front/app/features/tags/components/BoardDailyReportTimeline.vue`，并按职责拆出 masthead、边栏、话题区块和 mini 生命线等局部组件/工具，避免继续扩大单文件状态复杂度。
- 复用 `front/app/api/dailyReports.ts` 的 `getBoardDailyReports`、`getDailyReportDetail`、`getTopicLifeline`，以及 articles API 的 `getArticle`；不改变现有 API 契约。
- 复用 `BoardThreadBrowser.vue` 的 identity edge 语义和贝塞尔路径规则，但不直接耦合其完整画布实现。
- 更新 `docs/reference/architecture/frontend.md` 与相关日报 API/交互说明；demo 仅作视觉参考，不作为生产验收替代。
- 前置依赖：原 `persistent-topic` change 的日报详情 topic brief 修复完成并重启部署后的后端进程。
