# Tasks: AI 模型健康门 + 启动自动拉起 + 模型类型区分

> 垂直切片，按依赖顺序 A→B→C→D。每切片独立可交付、可验证。尾部遵循开发执行规范 §11 归档门禁（测试 / 文档 / 验证三固定尾节，验证节每条=可执行命令+期望结果）。
>
> ⚠️ 前置：`analysispause`、`airouter`（含 TestConnection）、`aisettings`、`StartRuntime` 均已就绪。本 change 不依赖其他 active change。

## 1. 后端：provider 类型区分 + 路由绑定校验（A · ai-capability-routing）

- [x] 1.1 `models.AIProvider` 加 `ModelKind`（gorm `size:20;default:llm;index`）+ `StartCommand`（gorm `type:text`）。验收：AutoMigrate 加列成功（`go test ./internal/platform/database -run TestAutoMigrate` 或等效，列存在）
- [x] 1.2 版本化迁移 `20260802_0001_ai_provider_model_kind_backfill`：backfill 挂在 embedding 路由的 provider→embedding（幂等：仅更新仍为 llm 且确属 embedding 路由者）；查「同时挂 embedding+llm 路由」冲突 provider 并 `logging.Warnf` 列出。验收：testcontainer 幂等执行；构造冲突 fixture 断言告警日志
- [x] 1.3 `airouter.Store.UpsertRoute` 按 capability 类别校验 provider model_kind（embedding 路由只接 embedding，llm 路由只接 llm），违反返回明确错误。验收：`store_test.go` 覆盖 embedding 拒 llm / llm 拒 embedding / 同类放行
- [x] 1.4 `ai_handler`：`ListProviders` 透传 `model_kind` + `start_command_configured`（或原文）；`UpsertProviderRequest`/`UpdateProvider` 接受 `model_kind` + `start_command` + `clear_start_command`，缺省 model_kind=llm。验收：handler 测试覆盖传入/缺省/清除 start_command
- [x] 1.5 model_kind 合法值校验（llm/embedding），非法值拒绝。验收：单测覆盖

## 2. 后端：aihealth 包 — 启动健康检测 + 自动拉起 + 内存快照（B · ai-model-health）

- [x] 2.1 新包 `internal/platform/aihealth`：内存快照结构（routes 明细 + healthy + checked_at）+ mutex 守卫 + `Healthy()`（快照未就绪→false）+ `GetSnapshot()`。验收：单测覆盖未就绪/已就绪读取
- [x] 2.2 `RunStartupProbe(ctx, autoStart)`：遍历 enabled 且有 provider 的 routes，取每条第一个 provider，调 `airouter.TestConnection`；不通 && autoStart && 有 start_command → 脱离 exec（Windows DETACHED / *nix setsid）→ `pollProbe` 轮询≤45s；计算 healthy（宽松：embedding 主通 且 ≥1 llm 主通）；写快照。验收：单测（mock TestConnection + mock exec）覆盖 ①全通 ②emb不通无 start_command ③emb不通有 start_command+autoStart 拉起后通 ④autoStart=false 不 exec
- [x] 2.3 脱离执行 helper（Windows `cmd /c` + SysProcAttr DETACHED_PROCESS/CREATE_NEW_PROCESS_GROUP；*nix `sh -c` + Setsid），fire-and-forget（不 Wait）。验收：单测断言命令被构造（跨平台 build tag 分离，不真起进程）
- [x] 2.4 healthy 宽松判定纯函数 + 缺路由/缺 embedding/缺 llm 边界。验收：单测全覆盖

## 3. 后端：健康门 wiring + 启动集成 + API + aisettings（C · analysis-pause-control + ai-model-health）

- [x] 3.1 `analysispause.IsPaused()` 改为 `用户暂停 || !aihealth.Healthy()`；`PausedAt()` 不变。`gate_test.go` 更新覆盖：用户暂停/未暂停 × 健康/不健康/快照未就绪。验收：`go test ./internal/platform/analysispause` PASS
- [x] 3.2 `aisettings` +`auto_start_models`（默认 false）Load/Save（复用 loadConfigByKey/saveConfigByKey 模式，存为 `"true"`/`"false"` 纯值或 JSON）。验收：单测覆盖缺省 false / 读写
- [x] 3.3 `app.StartRuntime`：`resetStaleStates` 之后 `go aihealth.RunStartupProbe(ctx, autoStartFromAisettings)`（异步，快照未就绪期 worker 见 healthy=false 不跑）。验收：启动日志/集成断言 probe 被触发且不阻塞 worker 启动
- [x] 3.4 handler：`GET /api/ai/health`（快照+auto_start_models）、`PUT /api/ai/health/auto-start-models`（{enabled}）。路由注册 `ai` group 下。验收：handler 测试覆盖读/写
- [x] 3.5 `GetSchedulersStatus` 增量返回顶层 `ai_healthy` + 各路由主 provider 简明通断明细。验收：handler 测试断言新字段存在
- [x] 3.6 集成测试（testcontainer）：启动→probe→healthy 翻转→worker resume 链路（mock provider 端点）。验收：testcontainer PASS

## 4. 前端：模型表单 + 路由过滤 + 健康面板 + banner（D，依赖 1/3）

- [x] 4.1 `SettingsSectionAiProviders.vue`：表单加 `model_kind` 单选（llm/embedding，默认 llm）+ `start_command` 输入（占位符 `llama-server -m D:/models/qwen.gguf --port 8081`）+ 清除入口。API 封装 `client.ts`/`ai.ts` 加字段。验收：组件测试断言字段渲染与提交
- [x] 4.2 `SettingsSectionCapabilityRoutes.vue`：挂 provider 候选按 `model_kind` 过滤（embedding 路由只列 embedding provider）。验收：组件测试断言过滤
- [x] 4.3 设置页新增「AI 健康状态」面板（各路由主 provider 通断 + launched 徽标 + 上次检测时间）+ `auto_start_models` 开关（调 `PUT /api/ai/health/auto-start-models`）。验收：组件测试断言面板与开关交互
- [x] 4.4 `useSchedulerStatus` 消费 `ai_healthy` + 简明明细；新增全局 banner 组件：`!analysisPaused && !aiHealthy` → 显示「AI 模型未就绪（LLM/Embedding 未连通），分析暂停运行」+ 跳设置页；`analysisPaused=true` 时不显示。验收：组件测试断言两种条件
- [x] 4.5 全语义 token，复用 AppButton/AppDialog/AppInput，禁 `window.*`。验收：grep 无 `window.(alert|prompt|confirm)`（本次新增代码零命中；存量死代码 useGlobalSettings.ts 有一处历史遗留，未动）

## 5. 架构体检（§7 强制，每个子任务后）

- [x] 5.1 `codegraph impact`：`IsPaused`/`UpsertRoute`/`RunStartupProbe` 波及面无 HIGH/CRITICAL 忽略
- [x] 5.2 新增 handler grep 路由注册二次确认（codegraph 追不到 group.GET/PUT）：`/api/ai/health`、`/api/ai/health/auto-start-models` 注册确认
- [x] 5.3 传导链守卫：重跑 analysis-pause-control 既有 gate_test，确认暂停生效范围/优雅停/续跑语义未被健康维度破坏
- [x] 5.4 分层合规：aihealth 在 `internal/platform/aihealth`，不引入对 admin/dataenrichment 的循环依赖；前端 banner 在 `features/settings` 或 `app/components`，无循环

## 6. 数据兼容性（§10）

- [x] 6.1 AutoMigrate 加列（model_kind default llm、start_command 空）幂等，testcontainer 反复执行无错
- [x] 6.2 backfill 迁移幂等；历史 provider model_kind 默认 llm 无感
- [x] 6.3 「同时挂 embedding+llm 路由」冲突 provider 迁移告警 + 后续保存校验拦截（不静默改路由）
- [x] 6.4 回滚路径：DROP 两列可逆；健康门可通过 `analysispause` 还原旧逻辑独立 revert；aihealth 包可整包移除

## 7. 文档（§12.4 里程碑收尾统一更新）

<!-- doc-impact: api（/api/ai/health、/api/ai/health/auto-start-models、/api/ai/providers 增 model_kind/start_command、/schedulers/status 增 ai_healthy）、database（ai_providers 增 model_kind/start_command 列 + 20260802_0001 backfill 迁移）、architecture（新增 internal/platform/aihealth 包 + StartRuntime 健康检测/自动拉起装配 + map.md 入口）、configuration（auto_start_models + AIProvider.model_kind/start_command）、flow（scheduler 暂停总闸引入健康门维度、ai-summary provider 类型区分）、standard/backend（aihealth 包测试范式）。apply 启动时以 doc-impact.sh suggest 实际预勾选为准；若 suggest 命中其他 flow/standard，在此补对应更新） -->

- [x] 7.1 `docs/reference/api/`：补 `/api/ai/health`、`/api/ai/health/auto-start-models`；`/api/ai/providers` 补 model_kind/start_command；`/schedulers/status` 补 ai_healthy
- [x] 7.2 `docs/reference/database/`：ai_providers 补 model_kind、start_command 列 + backfill 迁移说明
- [x] 7.3 `docs/reference/flow/`（analysis-pause 相关）：暂停判定补「有效暂停 = 用户暂停 || NOT 健康」+ 健康门硬执行；aihealth 启动检测 + 自动拉起链路
- [x] 7.4 `docs/reference/configuration.md`：补 auto_start_models 配置项（默认 false、语义、安全提示）
- [x] 7.5 `docs/reference/standard/`（backend）：无新测试范式（aihealth 沿用 testcontainer/SQLite 既有模式 + 可注入接缝 probeFn/launchFn），无对应文档文件，跳过

## 8. 测试（§11.2）

> 归档前重跑，零失败。后端 go 命令须 cmd.exe；前端 typecheck/build/test 须 cmd，lint 可 WSL。

- [x] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/airouter ./internal/platform/analysispause ./internal/platform/aihealth ./internal/admin/handler -short"` → PASS
- [x] T.2 testcontainer 集成：AutoMigrate 加列 + backfill 幂等 + 路由绑定类型校验 + 启动 probe healthy 翻转链路 → PASS
- [x] T.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全过（含 model_kind/start_command 表单 + 路由过滤 + 健康面板 + banner 条件 + auto_start_models 开关）
- [x] T.4 `grep -rnE "window.(alert|prompt|confirm)" front/app` → 零命中

## 9. 验证（§11.2，归档前实测）

- [x] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [x] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/platform/... ./internal/admin/... ./internal/app/..."` → VET_OK
- [x] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/platform/airouter ./internal/platform/analysispause ./internal/platform/aihealth ./internal/admin/... ./internal/app/..."` → 0 issues
- [x] V.4 `cd front && pnpm lint` → 0 error（lint WSL 可跑）
- [x] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [x] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [x] V.7 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验）
- [ ] V.8 浏览器视觉验收（cmd 起后端 + 前端）：① provider 表单可设 model_kind + start_command ② embedding 路由挂 provider 按 kind 过滤 ③ 不配模型时分析不跑 + 顶部 banner 提示 ④ 配好模型重启后分析自动恢复（无需手动点）⑤ 开 auto_start_models + 填 start_command，停掉 llama.cpp 重启后端 → 进程被自动拉起、健康翻绿 ⑥ 设置页健康面板展示各路由通断 + launched 徽标

## 10. 健康探活健壮性 + 恢复时重新探活 + 顶部栏健康指示（bug 修复，用户重启实测发现）

- [x] 10.1 [后端·A] `aihealth.RunStartupProbe`：`store.ListRoutes()` 失败时重试（3 次，~2s 退避），仍失败才置 `Healthy:false`；别让一次瞬态 DB 连接错误（如 Windows socket 耗尽 WSAENOBUFS / 端口冲突）把健康门焊死
- [x] 10.2 [后端·B] `SetAnalysisPause` handler：恢复（`paused=false`）时异步触发 `go aihealth.RunStartupProbe(...)`，让"点启动分析"能重新探活、自愈健康门
- [x] 10.3 [前端] 顶部栏 `AppHeaderView` 加常驻 AI 健康状态指示（健康=绿、不健康=红/琥珀，`mdi:heart-pulse`），点击跳 `/settings?section=ai-health`；用 `useSchedulerStatus` 的 `aiHealthy`
- [x] 10.4 [spec] 同步更新：ai-model-health（探活重试/韧性）、analysis-pause-control（恢复触发重新探活 + 顶部栏常驻健康指示）
