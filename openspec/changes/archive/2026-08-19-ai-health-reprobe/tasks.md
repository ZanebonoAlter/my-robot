<!-- doc-impact: flow api database architecture configuration -->

## 1. 后端：aihealth 重探能力（TDD）

- [x] 1.1 拆出 `runProbeLocked`（RunStartupProbe 现有逻辑整体搬入，含 listRoutesWithRetry/outcomes/宽松判定/setSnapshot），`RunStartupProbe` 改为 TryLock 失败打日志跳过、成功调 runProbeLocked
- [x] 1.2 新增 `TryStartProbe(ctx, store, autoStart) bool`：TryLock 成功 → go runProbeLocked → true；失败 → false（含单元测试：in-flight 时返回 false 且不并发探测、正常时返回 true 且快照最终更新）
- [x] 1.3 新增 `StartPeriodicReprobe(ctx, store, autoStart)`：Ticker 间隔 `reprobeInterval`（默认 60s，包级变量），tick 时快照 healthy 则跳过，否则 TryStartProbe；ctx 取消即退出（含单元测试：not healthy 时按间隔触发、healthy 后不再探测、in-flight 时跳过、ctx cancel 停止）
- [x] 1.4 后端测试：`go test ./internal/platform/aihealth/...` 全绿

## 2. 后端：reprobe API + 启动挂载

- [x] 2.1 新增 handler `ReprobeAIHealth`（admin handler 包）：调 TryStartProbe 返回 `{success, data:{triggered, skipped}}`；in-flight 跳过不报错（含单元测试：正常触发 triggered=true、in-flight 时 skipped=true）
- [x] 2.2 路由注册：`POST /api/ai/health/reprobe`（admin/routes.go 与现有 ai/health 同组）
- [x] 2.3 runtime.go：StartRuntime 里 `go aihealth.StartPeriodicReprobe(...)`（紧邻现有启动探测，复用同一 store 与 autoStart 值）
- [x] 2.4 后端测试：`go test ./internal/admin/... ./internal/platform/aihealth/...` + `golangci-lint run ./...` + `go vet ./...` + `go build ./...`

## 2b. 后端：全局代理绕过回环地址（代理污染修复，TDD）

- [x] 2b.1 `httpclient.go`：SetProxy 安装的 Proxy 函数改为「回环目标（localhost / 127.x / ::1 / 空 host）→ 直连；其余 → 代理」；空代理 URL 行为不变
- [x] 2b.2 单元测试：设假代理 server + 假目标 server，验证回环目标直连（代理收不到请求）、外部目标走代理、未设代理时直连
- [x] 2b.3 后端测试：`go test ./internal/platform/httpclient/...` + lint/vet/build

## 3. 前端：手动重新检测入口

- [x] 3.1 `front/app/api/aiAdmin.ts` 新增 `reprobeHealth()` → POST /ai/health/reprobe；`front/app/types/ai.ts` 新增 `AIHealthReprobeResult {triggered, skipped}`
- [x] 3.2 新增 composable `useHealthReprobe`：reprobe → 轮询 getHealth 至 checked_at 更新或 ~30s 超时 → 返回最新快照；暴露 `reprobeing` 状态
- [x] 3.3 `SettingsSectionAiHealth.vue`：健康卡片头部加「重新检测」按钮（AppButton），点击走 composable，完成后刷新快照与 autoStart；按钮 loading 态「检测中…」
- [x] 3.4 `AiHealthBanner.vue`：banner 加「重新检测」按钮，点击走 composable，完成后刷新 scheduler status（banner 可见性依赖 aiHealthy）并更新快照
- [x] 3.5 前端验证：`pnpm lint` + `pnpm exec nuxi typecheck` + `pnpm test:unit` + `pnpm build`（cmd.exe 执行 typecheck/build/test:unit）

## 4. 端到端验证 + 文档

- [x] 4.1 实机验证（代理污染修复后）：起后端 → 等 llama-server 加载 → 观察定时重探自动自愈（无需手动操作健康变为 true）；再验证手动 reprobe API 返回 triggered=true 且快照刷新
- [x] 4.2 前端验证（agent-browser）：设置页点「重新检测」→ banner 消失/健康徽标变绿
- [x] 4.3 更新活文档：`docs/reference/flow/scheduler.md`（健康门节：定时重探 + reprobe API + 前端入口 + 代理回环直连）、`docs/reference/api/ai-admin.md`（新增端点）、`docs/reference/configuration.md`（代理节补充回环直连说明）
- [x] 4.4 `openspec validate` 通过 + doc-impact verify/check-standards 检查
