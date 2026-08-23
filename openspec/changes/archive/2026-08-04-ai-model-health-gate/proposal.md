## Why

现状缺口（本地跑 llama.cpp 的单用户视角）：

1. **分析闸 fail-open，模型没好也照跑**：`analysispause.IsPaused()` 只读用户开关（`analysis_paused`），读失败还 fail-open。LLM/Embedding 没配好、或本地 llama.cpp 没拉起时，分析类调度（总结/打标/日报/生命线/firecrawl…）照样 lease，狂刷 provider 调用错误日志、空耗资源。
2. **本地模型进程每次重启要手动起**：用户跑 llama.cpp（`llama-server`），每次后端/机器重启都得手动敲命令把进程拉起来，否则探测永远不通、分析一直空转报错。
3. **provider 没有 llm/embedding 类型区分**：`AIProvider` 只有 `provider_type`（openai_compatible/ollama = 协议），没有「这个模型是 LLM 还是 Embedding」的属性，模型类型只能靠它挂在哪条 capability 路由反推；embedding 任务路由可能误挂 LLM provider，运行时才报错。

## What Changes

三块，按依赖顺序切片：

### A. provider 类型区分 + 路由绑定约束 —— ai-capability-routing

`AIProvider` 加 `model_kind`（llm/embedding，默认 llm）+ `start_command`（可空，=本地进程启动命令）。路由绑定时强校验：`embedding` capability 路由只接 `model_kind=embedding` 的 provider，其余（llm 类）路由只接 `model_kind=llm`。**embedding 任务必须用 embedding 模型**（硬约束，保存时拦截）。

### B. 启动健康检测 + 本地模型自动拉起 —— ai-model-health（新 capability）

后端启动时遍历每条 enabled 且有 provider 的路由，探其第一个（priority 最高）provider（复用 `airouter.TestConnection` GET /models）。新增配置 `auto_start_models`（aisettings，默认 off）= 自动拉起总开关：

- 探活不通 && 该 provider 有 `start_command` && 开关 on → fire-and-forget 执行启动命令（Windows `cmd /c` / *nix `sh -c`，脱离父进程）→ 轮询探活最多 ~45s。
- 开关 off → 只检测、不 exec（保守，默认不替用户起进程）。
- **只针对每个 route 的第一个 provider**（不穷举所有 provider）。

健康快照存内存，供 enablement 判定与 API 查询。**触发只在后端启动**（不做周期复检、不做前端「一键拉起」按钮——运行中 llama.cpp 崩了要到下次重启才被重新拉起，已知代价）。

### C. 健康门硬执行 —— analysis-pause-control

`analysispause.IsPaused()` 升级为 `用户暂停 || !健康`。**健康（宽松判定）= embedding 路由主 provider 可达 且 ≥1 条 llm 路由主 provider 可达**。启动时健康=false，探活通过后才置 true → 天然「默认关，检测到才开」。**用户开关、按钮、API 全不动**（手动 start 不健康也不拒绝，只是 worker 因健康门不跑）；前端在「意图=运行 但 !健康」时顶部 banner 提示去配置/拉起模型。

## Capabilities

### Modified Capabilities

- `analysis-pause-control`：有效暂停态引入健康维度（`有效暂停 = 用户暂停 || !健康`）；用户开关语义/持久化/API/按钮**全部不变**；新增「健康门硬执行」+「前端健康未就绪提示」。
- `ai-capability-routing`：provider 新增 `model_kind` / `start_command`；路由绑定按 `model_kind` 校验（embedding 任务必须用 embedding 模型）。

### Added Capabilities

- `ai-model-health`（新）：启动健康检测、本地模型自动拉起（`auto_start_models` 总开关）、宽松健康判定、健康快照与查询 API。

## Impact

- **后端**
  - `models.AIProvider`：+`ModelKind`（默认 llm，AutoMigrate 加列）+`StartCommand`（text，可空，AutoMigrate 加列）。
  - 版本化迁移：backfill `model_kind`（挂在 embedding 路由的 provider → embedding，其余 llm）；检测「同时挂在 embedding+llm 路由」的冲突 provider 并告警。
  - `airouter.Store.UpsertRoute`：按 capability 类别校验 provider model_kind，违反拒绝。
  - 新包 `internal/platform/aihealth`：启动探活 + 自动拉起 + 内存快照 + `Healthy()`。
  - `analysispause.IsPaused()`：改为 `用户暂停 || !aihealth.Healthy()`；`PausedAt()` 仍返回用户暂停时间戳（不变）。
  - `app.StartRuntime`：启动时异步触发 `aihealth.RunStartupProbe(autoStart)`（探活期间 worker 见 healthy=false 自然不跑）。
  - `aisettings`：+`auto_start_models`（默认 false）读写。
  - 新 handler：`GET /api/ai/health`（快照 + auto_start_models）、`PUT /api/ai/health/auto-start-models`；`/schedulers/status` 增量返回 `ai_healthy` + 各路由主 provider 通断。
  - `ai_handler`：provider CRUD 透传 `model_kind` / `start_command`。
- **前端**
  - 模型管理表单（`SettingsSectionAiProviders.vue`）：+`model_kind` 单选（llm/embedding）+`start_command` 输入框（可空，占位符给 llama-server 示例）。
  - 能力路由（`SettingsSectionCapabilityRoutes.vue`）：挂 provider 时按 `model_kind` 过滤候选。
  - 设置页新增「AI 健康状态」面板（各路由主 provider 通断 + 是否被后端拉起 + 上次检测时间）+ `auto_start_models` 开关。
  - `useSchedulerStatus` 消费 `ai_healthy`；意图=运行 但 !健康 时顶部 banner「AI 模型未就绪（LLM/Embedding 未连通），分析暂停运行」+ 跳设置页。
- **数据兼容**：`model_kind` 默认 llm（历史 provider 无感）；`start_command` 空；路由绑定校验可能拦下存量错误配置（embedding 路由误挂 llm provider）→ 迁移告警 + 需手动修正。
- **安全**：`start_command` 执行是注入面；单用户本地工具可接受，文档明示风险。
- **AI 成本**：健康检测用 `GET /models`（不发推理请求、零 token）。

## 依赖与执行顺序

A（类型区分 + 路由校验）→ B（健康检测 + 自动拉起 + 快照）→ C（健康门 wiring + 前端提示）。A 是 B/C 的基础：健康判定与拉起都按「路由主 provider + model_kind」进行。三块可在同一 change 内切片交付，C 依赖 B 的 `Healthy()` 与快照 API。
