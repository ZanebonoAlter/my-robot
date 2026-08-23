## Why

后台分析任务（AI 摘要、全文抓取、打标签、向量化、日报、生命线汇总）持续运行会抢占 LLM 额度、CPU 与网络资源。用户需要「一键暂停所有分析」以便专注其他事务（如打游戏），同时**文章入库（RSS 拉取）照常进行**；事后一键恢复，堆积的队列任务自动续跑。现有持久化队列（`firecrawl_jobs` / `tag_jobs` / `embedding_queues`）已天然支持「停 worker 后任务静卧、恢复后接着消化」，本变更只需在调度入口加一道全局总闸。

## What Changes

- 新增**全局「分析暂停」运行时开关**，状态持久化到 DB（复用 `ai_settings` config store），服务重启后保持暂停状态。
- 暂停生效时，分析类调度任务的 `JobFunc` 在每次 tick 自检暂停标志，若已暂停则直接 return、不 lease 新任务，覆盖：`content_completion` / `firecrawl` / `daily_report` / `board_upgrade_suggest` / `lifeline_weekly` / `lifeline_monthly` / `lifeline_yearly` / `tag_quality_score`。
- 独立于调度注册表的 **tag worker 池**（`tagging.StartAllWorkers`）同步接入暂停 gate，暂停时不消费 `tag_jobs` / `embedding_queues`。
- **保留运行**：`auto_refresh`（RSS 入库）及维护类调度（`log_cleanup` / `aux_label_cleanup` / `blocked_article_recovery` / `rsshub_catalog_sync` / `preference_profile_update`）。
- **优雅停**：暂停只阻断新 tick，当前正在执行的 job 自然跑完，不强杀、不产生半截数据。
- 新增后端 API：`GET /api/analysis/pause`（读状态）与 `POST /api/analysis/pause`（切换暂停/恢复）。
- 前端：首页顶部栏（`AppHeaderView`）新增二态开关按钮（`mdi:pause` ↔ `mdi:play`，暂停态琥珀色高亮）；暂停时经 `useHead` 将浏览器 favicon 切换为带 ⏸ 角标的 SVG；切换时 `useNotify` 弹提示；暂停态经 `useSchedulerStatus` 轮询维持。
- iconify 离线子集脚本（`scripts/generate-icon-subset.mjs`）注册 `mdi:pause` / `mdi:play`。
- 恢复后：堆积的 pending 任务按既有 `created_at` 顺序自动消化，无需手动干预。

## Capabilities

### New Capabilities

- `analysis-pause-control`: 全局「分析暂停/恢复」的运行时控制——开关状态持久化、优雅停、统一覆盖调度器与 tag worker 池，并与现有 per-feed 的 `feed-tagging-control` 形成「总闸/分闸」共存关系（全局暂停时即使 feed 开启打标签也不跑）。

### Modified Capabilities

- `scheduler-observability`: 调度器状态 API（`GET /api/schedulers`）与前端状态面板需体现「分析已暂停」语义，使暂停态在调度器可观测层端到端可见。

## Impact

- **后端**
  - 各分析类 `JobFunc` 增加暂停标志自检（一处 gate，各 job 复用）。
  - `tagmanagement` worker 池增加暂停 gate。
  - 新增 analysis pause handler + route（`internal/app/router.go`）。
  - `ai_settings` config store 复用，无破坏性 schema 变更。
- **前端**
  - `features/shell/components/AppHeaderView.vue`：顶部栏右侧操作区加二态开关按钮。
  - `composables/useSchedulerStatus.ts`：承载暂停态轮询。
  - `useHead`：favicon SVG 角标动态切换（参考 `useTheme.ts` 既有 useHead 先例）。
  - `composables/useNotify.ts`：切换提示。
  - `api/scheduler.ts`：新增 pause API 封装。
  - `scripts/generate-icon-subset.mjs`：注册 `mdi:pause` / `mdi:play`。
- **DB**：复用 `ai_settings`，无 migration、无破坏性变更。
- **交互**：与 `feed-tagging-control` 共存——全局暂停为总闸，`feed.tagging_enabled` 为分闸；总闸关闭时分闸无效，恢复后分闸重新生效。
