# ai-model-health Delta Specification

## MODIFIED Requirements

### Requirement: 启动时模型健康检测

系统 SHALL 在后端启动时执行一次模型健康检测：遍历每条 enabled 且已绑定 provider 的 AI 路由，取其 priority 最高（第一个）provider，复用 `airouter.TestConnection`（GET `{base_url}/models`，零推理 token）探测可达性，结果写入内存健康快照。检测 SHALL 只针对每条路由的第一个 provider，SHALL NOT 穷举该路由的 fallback provider。检测 SHALL 在后端启动时触发一次；当健康快照判定 not healthy 时，系统 SHALL 由后台定时重探器（见 ai-health-reprobe 能力）周期性复检直至快照 healthy，期间 SHALL 复用同一全局探测互斥与拉起冷却约束。

#### Scenario: 启动时探测每条路由主 provider

- **WHEN** 后端启动，存在 enabled 的 summary 路由（主 provider A）与 embedding 路由（主 provider B）
- **THEN** 系统 SHALL 探测 A 与 B 的可达性，并将两条结果写入健康快照

#### Scenario: 仅探主 provider 不探 fallback

- **WHEN** 某 embedding 路由绑定了主 provider A 与 fallback provider B
- **THEN** 启动健康检测 SHALL 只探测 A，SHALL NOT 探测 B

#### Scenario: 无 provider 的路由跳过

- **WHEN** 某 enabled 路由未绑定任何 provider
- **THEN** 启动健康检测 SHALL 跳过该路由（不产生快照条目，也不视为「down」条目）

#### Scenario: ListRoutes 瞬态失败时重试

- **WHEN** 启动健康检测查询路由列表（store.ListRoutes）失败（如瞬态 DB 连接错误、socket 耗尽、端口冲突）
- **THEN** 系统 SHALL 重试若干次（默认 3 次、~2s 退避），仅当反复失败才记 NOT 健康；避免单次瞬态错误永久焊死健康门，使用户点「恢复」亦无法自愈

#### Scenario: 快照不健康时定时复检直至自愈

- **WHEN** 启动探测后快照判定 not healthy（如本地模型加载超时），模型在启动后数分钟才加载完成
- **THEN** 系统 SHALL 由定时重探器周期性复检，模型可达后快照 SHALL 自动更新为 healthy，无需用户干预

#### Scenario: 快照健康后不再周期性复检

- **WHEN** 健康快照判定 healthy
- **THEN** 系统 SHALL NOT 周期性复检（保持已健康状态，避免无谓探测）

### Requirement: 健康快照与查询 API

系统 SHALL 在内存维护一份健康快照（各路由主 provider 的可达性明细 + 整体 healthy + 上次检测时间），并提供 `GET /api/ai/health` 返回该快照与 `auto_start_models` 当前值。系统 SHALL 提供 `PUT /api/ai/health/auto-start-models`（body `{enabled:bool}`）更新 `auto_start_models` 并返回新值。系统 SHALL 提供 `POST /api/ai/health/reprobe` 手动触发异步重探（幂等，探测 in-flight 时跳过，不改变分析暂停状态）。`/schedulers/status` 顶层 SHALL 增量返回 `ai_healthy` 布尔与各路由主 provider 的简明通断明细，供前端无需额外请求即可渲染提示。

#### Scenario: 查询健康快照

- **WHEN** 调用 GET /api/ai/health
- **THEN** 返回 { healthy, checked_at, auto_start_models, routes:[{route_name, capability, primary_provider, model_kind, reachable, launched_by_backend, last_checked, error}] }

#### Scenario: 设置自动拉起总开关

- **WHEN** 调用 PUT /api/ai/health/auto-start-models，body { enabled: true }
- **THEN** 系统 SHALL 持久化 auto_start_models=true 并返回 { enabled: true }

#### Scenario: 手动触发异步重探

- **WHEN** 调用 POST /api/ai/health/reprobe，无探测 in-flight
- **THEN** 系统 SHALL 异步启动一次 RunStartupProbe 并立即返回（triggered=true），快照由探测完成时更新

#### Scenario: 探测 in-flight 时手动重探跳过

- **WHEN** 调用 POST /api/ai/health/reprobe，恰有一次探测 in-flight
- **THEN** 系统 SHALL 返回 skipped=true，SHALL NOT 并发探测

#### Scenario: 调度器状态携带健康摘要

- **WHEN** 调用 GET /schedulers/status
- **THEN** 响应顶层 SHALL 包含 ai_healthy 布尔与各路由主 provider 的简明通断明细
