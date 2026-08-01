## Why

「叙事工坊」的「板块内容」tab（`BoardCompositionPanel`）当前只承载**构成标签管理**（增删辅助标签 chip）。标签少时页面下半部大片空白。而它是用户选中板块后**默认首屏**——第一眼应获得的是"板块整体态势"，而非一个管理工具。

Syntopica 的产品定位（见 `README.md`）回答的是「我长期关注的领域今天发生了什么 / 一个话题是刚出现、持续发展、还是正在分化与结束」，组织单位是语义板块与叙事脉络，**明确不做热点榜单与舆情排行**。因此首屏该展示的不是「每日 top 标签 / 热力图」这类舆情榜单形态，而是**板块级话题态势版图**：让研究者一眼看清跟踪的各持久话题分别处在什么阶段、哪个最近有变化。

关键约束：话题态势**只能基于 identity 轨（持久话题泳道）的可靠字段**派生，**不可使用 similarity 轨**（匈牙利二分法在 section↔section 时间线连线上派生的 emerging/continuing/split/merge/ending 五态），后者在长跨度数据上不可靠。本 change 严格区分双轨，所有态势只读 `BoardPersistentTopic` 的 `planLifecycle` 维护字段（`status` / `hit_count` / `consecutive_hits` / `first_seen_date` / `last_seen_date` / `is_vacuum`）与 `section.persistent_topic_id`（AND-gate 锚定写入的 identity 快照）聚合。

## What Changes

- 后端新增聚合接口 `GET /api/semantic-boards/:id/topic-landscape?days=N`：基于 identity 轨返回板块全部持久话题的态势派生结果 + 各话题的近 N 日命中节奏（mini-lifeline），并顺带聚合板块活力顶栏指标。零数据迁移，不改任何现有表/逻辑。
- 「板块内容」tab 在构成标签管理区**下方**新增「话题态势版图」区，与上方管理区视觉分隔（不混语义）。
- 态势分区卡片墙：话题按派生态势分组（🌱新冒头 / 🔴待激活 / 🟢活跃 / ⏸️停滞 / ⬛已归档），每张卡片含 label + 关键数字 + mini-lifeline 节奏条；强吸引（`is_vacuum`）作叠加标记。
- 卡片点击跳「话题总览」tab 深挖；待激活话题红色描边引导转正。
- 顶部一行活力指标（近 N 日文章/section 数、活跃话题数、产出趋势缩略折线）。
- 空态：板块无日报时引导「生成日报」，不展示空版图。

## Capabilities

### New Capabilities

- `board-topic-landscape` — 板块内容首屏的「话题态势版图」：基于 identity 轨的态势派生 + 分区卡片墙 + mini-lifeline + 活力顶栏 + 后端聚合接口。

### Modified Capabilities

（无 —— 不改动既有持久话题状态机、锚定机制或日报生成管线；只**读取**已有 identity 字段做派生聚合。）

## Impact

- **前端**：`BoardCompositionPanel.vue` 下方挂载新区；新增 `TopicLandscape*.vue` 组件族（版图容器 / 态势卡片 / mini-lifeline / 活力顶栏）；`front/app/api/` 加接口 client。
- **后端**：`internal/topicgraph/handler/daily_report_handler.go` 注册 1 条新路由；`internal/topicgraph/repository/` 加聚合查询方法（`GetBoardTopicLandscape`）。**不触碰** `daily_report_assignment.go`（生命周期）、`daily_report_matching.go`（匈牙利 similarity）、`daily_report_lane.go`（锚定）。
- **数据**：零迁移。纯读 `board_persistent_topics` + `daily_report_sections`（`persistent_topic_id`）+ `board_daily_reports`（`period_date`）。
- **文档**：`docs/reference/flow/semantic-board.md` 增「话题态势版图」链路段；`docs/reference/api/` 加接口条目；归档时按 §12 补 flow 变更溯源。
- **部署后影响（必读）**：(a) 用户选中板块后，首屏下方多出「话题态势版图」区，行为变化仅展示层，无破坏性；(b) 无需手动操作，无数据重生成；(c) 旧数据无降级问题——历史已锚定 `persistent_topic_id` 的 section 直接参与态势计算，未锚定（NULL）的 section 自然不归入任何话题，符合既有语义。
