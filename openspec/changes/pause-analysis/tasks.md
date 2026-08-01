## 1. 后端：暂停标志存储

- [x] 1.1 在 `internal/platform/aisettings/config_store.go` 新增 `analysis_paused`(bool) 与 `analysis_paused_at`(time) 的 Load/Save 函数（参考 `daily_report_time` 同款模式）
- [x] 1.2 单元测试：默认 false、写入后读回一致、时间戳正确

## 2. 后端：analysispause gate 模块

- [x] 2.1 新建 gate 包（如 `internal/platform/analysispause`），提供 `IsPaused(db) bool` 与 `Guard(job scheduler.JobFunc) scheduler.JobFunc` wrapper（暂停时返回 `skipped: paused` 的 JobResult、不执行原 job）
  - 实现：`IsPaused() bool`（无 db 参数，aisettings 用全局 database.DB；见 design D2）；Guard 因避免 import cycle 拆为 `scheduler.PauseAware`（见 design D1）
- [x] 2.2 单元测试：Guard 在 paused 时返回 skip 结果且不调用原 job、非 paused 时透传执行原 job 并返回其结果

## 3. 后端：scheduler 侧接入 gate

- [x] 3.1 在 `internal/app/runtime.go` 注册以下分析类 job 时包 `Guard`：`content_completion`、`firecrawl`、`daily_report`、`board_upgrade_suggest`、`lifeline_weekly`、`lifeline_monthly`、`lifeline_yearly`、`tag_quality_score`
- [x] 3.2 让受影响调度器在 `GET /api/schedulers` 返回中体现 paused 语义（scheduler-observability：受影响调度器状态条目带 paused 标记/说明）
  - 实现：`GetSchedulersStatus` 顶层加全局 `analysis_paused`/`analysis_paused_at`（符合 flow/scheduler 约束#1「单一事实源」——不加第二份清单，靠 PauseAware skip 行为 + 全局标志体现）

## 4. 后端：tag worker 池接入 gate

- [x] 4.1 `TagQueue`（`tag_queue.go`）lease 循环加 `analysispause.IsPaused` 自检，暂停时 sleep 一个 poll 周期、不 lease
- [x] 4.2 `EmbeddingQueueWorker`（`embedding_queue.go`）lease 循环加自检
- [x] 4.3 `MergeReembeddingQueueWorker` lease 循环加自检

## 5. 后端：暂停控制 API

- [x] 5.1 新增 analysis pause handler（`GET` 读状态 / `POST {paused}` 切换）+ 在 `internal/app/router.go` 注册 `/api/analysis/pause` 路由
  - 实现：路由注册在 `internal/admin/routes.go`（`app/router.go` 只调 `admin.RegisterRoutes`），handler 经 `admin/wire.go` re-export
- [x] 5.2 单元测试：`GET` 返回 `paused`/`paused_at`；`POST {paused:true}` 写入后 `GET` 一致且后续分析 job 不 lease；`POST {paused:false}` 恢复后状态回滚

## 6. 前端：API 封装 + 状态轮询

- [x] 6.1 `front/app/api/scheduler.ts` 新增 `getAnalysisPause` / `setAnalysisPause` 封装
- [x] 6.2 `composables/useSchedulerStatus.ts` 承载暂停态（随既有轮询周期获取 `paused`）
- [x] 6.3 单元测试：API 封装与状态 composable

## 7. 前端：顶部栏暂停开关

- [x] 7.1 在 `front/scripts/generate-icon-subset.mjs` 注册 `mdi:pause`、`mdi:play`，跑 `pnpm generate:icons` 生成离线子集
  - 实现：脚本是自动扫描源码图标名，无需手改；源码用 mdi:pause/play 后跑 generate:icons 即含（subset.json 已确认含 pause/play）
- [x] 7.2 `features/shell/components/AppHeaderView.vue` 顶部栏右侧操作区加二态开关按钮（运行态 `mdi:pause` 灰色、暂停态 `mdi:play` 琥珀高亮），点击调用 `setAnalysisPause`
- [x] 7.3 接 `useNotify`：暂停/恢复分别弹对应提示
- [x] 7.4 组件测试：二态渲染、点击触发对应 API 调用、tooltip 文案

## 8. 前端：favicon 暂停态标识

- [x] 8.1 watch 暂停态经 `useHead` 切换 `link[rel=icon]`：暂停注入带 ⏸ 角标的 SVG data URI，恢复切回 `/favicon.png`（参考 `useTheme.ts` 既有 useHead 先例）
- [x] 8.2 验证运行态保持原 `favicon.png` 不被污染

## 9. 测试

- [x] 9.1 后端：gate、API、三个 worker 自检的单元测试全绿（仅跑影响包）
- [x] 9.2 前端：API 封装、`useSchedulerStatus`、`AppHeaderView` 开关组件的单元测试全绿

## 10. 文档
<!-- doc-impact: flow api -->

- [ ] 10.1 apply 启动时跑 `bash scripts/doc-impact.sh suggest` 预勾选受影响文档，`bash scripts/doc-impact.sh context` 注入 flow 业务约束（最终声明以 suggest 为准）
- [x] 10.2 按 suggest 结果更新 `docs/reference/flow/scheduler.md`（新增"分析暂停总闸"业务约束）及相关 flow（`ai-summary`/`content-enrichment`/`data-enrichment` 注明暂停时行为）
- [ ] 10.3 归档前跑 `bash scripts/doc-impact.sh verify` 对账

## 11. 验证

- [x] 11.1 `cd backend-go && golangci-lint run ./internal/platform/analysispause ./internal/platform/aisettings ./internal/app ./internal/admin/... ./internal/tagmanagement/service/core` → 期望：无 lint 错误
- [x] 11.2 `cd backend-go && go vet ./internal/platform/analysispause ./internal/platform/aisettings ./internal/app ./internal/admin/... ./internal/tagmanagement/service/core` → 期望：无 vet 报错（已跑 exit 0）
- [x] 11.3 `cd backend-go && go test ./internal/platform/analysispause ./internal/platform/aisettings ./internal/tagmanagement/service/core` → 期望：所有测试 PASS（已跑 4 包全 ok）
- [x] 11.4 `cd backend-go && go build ./...` → 期望：编译成功（已跑 exit 0）
- [x] 11.5 `cd front && pnpm lint` → 期望：无 lint 错误（0 errors, 5 warnings 既有文件）
- [x] 11.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 期望：无类型错误（exit 0，vue-tsc 静默通过）
- [x] 11.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 期望：开关/API/状态相关用例 PASS（487 tests passed / 44 files）
- [ ] 11.8 手动验收：启动后端+前端 → 点顶部栏暂停，确认 content_completion/firecrawl 不再 lease、tag worker 不消费、auto_refresh 仍入库、favicon 出现 ⏸、重启后仍暂停；再点恢复 → 堆积任务续跑、favicon 复原
- [ ] 11.9 `bash scripts/doc-impact.sh verify` → 期望：文档节声明与实际改动一致（F/G 段通过）
