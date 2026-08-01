# Tasks — board-topic-landscape

<!-- doc-impact: api, flow -->
<!-- doc-impact-excuse: database=工作区脏改动误报(catalog_sync_service.go/db_unit_test.go 属其他 change)，本 change 零迁移；归档 verify 以本声明为准 -->

> 主线程只调度，重活按根 `AGENTS.md` 模型表派子线程。每任务附验收标准。
> 关键挂点：路由注册于 `RegisterDailyReportRoutes`（topicgraph/handler/daily_report_handler.go，`/semantic-boards/:id/topics` 同组）；可见过滤复用 `ListTopicsByBoardAll`+`FilterVisibleTopics`；N 用包级常量（零迁移）。

## 后端

- [x] 1.1 新增 `Repository.GetBoardTopicLandscape(boardID, days)`：复用 `ListTopicsByBoardAll` + `FilterVisibleTopics`（与 `/topics` 同口径）拿可见话题；遍历算 `stance`（design §2 规则，用 `UpgradeThreshold`/`status`/`consecutive_hits`/`last_seen_date`/`is_vacuum`）；每 topic 用 `generate_series` 聚合近 N 日 `section_count`（identity 轨 `section.persistent_topic_id`）。验收：返回结构与 design §3 契约一致；空日补行。
  - **测试**：`daily_report_repository_test.go` 加用例——多 stance 话题、空日、未达阈值 candidate 被过滤、`archived` 计数。
- [x] 1.2 在 `daily_report_topic_repository.go` 加包级常量 `topicLandscapeActiveWindowDays = 7`、`topicLandscapeDefaultDays = 30`（零迁移，不进 ai_settings）。验收：常量可被 1.1 引用。
- [x] 1.3 在 `RegisterDailyReportRoutes`（daily_report_handler.go:39 附近，`listBoardTopics` 后）注册 `api.GET("/semantic-boards/:id/topic-landscape", getBoardTopicLandscape)`；`days` clamp 到 7/14/30/90，缺省/`<=0`→30。验收：`curl` 返回 `success:true` + `topics`/`vitality`；空板块返回空数组非 null。
  - **测试**：handler 用例——非法 `days` clamp、不存在 board、stance 正确、空板块。

## 前端

- [x] 2.1 接口 client：`front/app/api/` 加 `getTopicLandscape(boardId, days)`，类型对齐 design §3。
- [x] 2.2 组件族 `features/tags/components/topic-landscape/`：
  - [x] 2.2.1 `TopicLandscapePanel.vue` — 容器（拉接口、loading、空态分发）。
  - [x] 2.2.2 `VitalityBar.vue` — 活力顶栏（数字 + 缩略折线；`feed_active=null` 时该子项隐藏不阻断）。
  - [x] 2.2.3 `StanceCardWall.vue` — 按 stance 分组（active→stalled→emerging→pending→archived），归档默认折叠，空分组不渲染。
  - [x] 2.2.4 `TopicStanceCard.vue` — 单卡（label/stance 图标/关键数字/红描边 pending/强吸引角标）。
  - [x] 2.2.5 `MiniLifeline.vue` — 近 N 日节奏条（格深浅 ∝ section_count，空日浅灰）。
- [x] 2.3 挂载到 `BoardCompositionPanel.vue`：构成标签管理区下方加分隔 + `<TopicLandscapePanel :board-id>`。
  - **测试**：`BoardCompositionPanel` 既有测试不破；新增态势区渲染用例（有/无 topic、pending 红描边、归档折叠）。
- [x] 2.4 卡片 click → 跳「话题总览」tab 并选中该 topic（复用 `openTopicOverviewDetectiveWall` / BoardThreadBrowser 既有选择路径）。验收：跳转后聚焦该 topic 泳道。
- [x] 2.5 空态卡：「该板块还没有日报…[生成日报]」→ `POST /api/daily-reports/generate` + WS 进度（复用既有 `handleTriggerBackfill`/生成逻辑）。验收：无日报板块展示空态卡；点击触发生成。

## 文档与归档门禁

- [x] 3.1 `docs/reference/flow/semantic-board.md` 增「话题态势版图」链路段（需求说明 + 代码入口），加变更溯源行（§12.2）。
- [x] 3.2 `docs/reference/api/` 增 `topic-landscape` 接口条目。
- [x] 3.3 跑 `bash scripts/doc-impact.sh verify` + `bash scripts/check-standards.sh`（§11.4）。
- [x] 3.4 门禁：后端 `golangci-lint run ./... && go vet ./... && go test ./internal/topicgraph/... && go build ./...`；前端 `pnpm lint && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build`（前端 typecheck/build/test:unit 经 Windows cmd）。
- [x] 3.5 部署影响汇报：按根 `AGENTS.md` 要求，完工汇报含「部署后行为变化 / 需手动操作 / 旧数据降级」一节。
