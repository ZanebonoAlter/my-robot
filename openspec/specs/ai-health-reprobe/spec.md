# ai-health-reprobe Specification

## Purpose
TBD - created by archiving change ai-health-reprobe. Update Purpose after archive.
## Requirements
### Requirement: 健康快照定时重探（自动自愈）

系统 SHALL 在后端启动时启动一个后台定时重探器：健康快照判定 not healthy 时，SHALL 按固定间隔（默认 60 秒）周期性重新探测（复用 RunStartupProbe 全流程，含自动拉起、45 秒轮询窗口、10 分钟拉起冷却、全局探测互斥），并在快照判定 healthy 后 SHALL 停止定时重探（保持已健康状态，避免无谓探测）。定时重探 SHALL NOT 改变健康判定口径、SHALL NOT 在探测 in-flight 时叠加探测（仍由全局互斥保证）。重探器 SHALL 仅在快照 not healthy 时活动，SHALL 永不"主动"把健康快照打回 not healthy。

#### Scenario: 慢加载模型加载完成后自动自愈

- **WHEN** 后端启动探测时本地 llama-server 仍在加载（45 秒轮询窗口内始终返回 502），快照判定 not healthy，模型在启动后数分钟加载完成
- **THEN** 定时重探器 SHALL 在后续间隔探测中探测成功，快照 SHALL 自动更新为 healthy，重探器 SHALL 随后停止

#### Scenario: 探测 in-flight 时定时触发被跳过

- **WHEN** 定时重探间隔到达，但一次探测（启动探测或手动重探）仍在进行中
- **THEN** 定时触发的探测 SHALL 被跳过（不并发、不重复拉起），由进行中的探测完成后更新快照

#### Scenario: 快照已健康时不再定时探测

- **WHEN** 健康快照判定 healthy
- **THEN** 定时重探器 SHALL 保持空闲，SHALL NOT 发起任何探测

#### Scenario: 定时重探仍遵守拉起冷却

- **WHEN** 定时重探发现某 provider 不可达，且该 provider 在 10 分钟冷却窗口内已被成功拉起
- **THEN** 重探 SHALL NOT 再次执行其 start_command，SHALL 仅继续轮询已拉起进程

### Requirement: 本地回环请求绕过全局出站代理

系统 SHALL 在配置了全局出站代理（`http_proxy_url`）时，对回环地址目标（hostname 为 `localhost`、`127.0.0.0/8` 网段、`::1`，或 hostname 为空）的请求 SHALL 绕过代理直连；对非回环目标 SHALL 继续走代理。该行为 SHALL 应用于所有经 httpclient 发出的请求（LLM 探测/推理、feed 抓取、Firecrawl 等），使得本地托管模型（llama-server）不被代理拦截。

#### Scenario: 本地探测不走代理

- **WHEN** 全局代理已配置，健康探测请求目标是 http://localhost:8080/v1/models
- **THEN** 请求 SHALL 直连 8080，SHALL NOT 发往代理

#### Scenario: 回环 IP 同样直连

- **WHEN** 全局代理已配置，请求目标是 http://127.0.0.1:8081/v1/models 或 http://[::1]:8081/v1/models
- **THEN** 请求 SHALL 直连，SHALL NOT 发往代理

#### Scenario: 外部请求仍走代理

- **WHEN** 全局代理已配置，请求目标是外部站点（如 https://rsshub.example.com）
- **THEN** 请求 SHALL 继续经代理转发

#### Scenario: 未配置代理时不受影响

- **WHEN** 未配置全局代理
- **THEN** 所有请求 SHALL 保持直连（不引入回环直连逻辑之外的行为变化）

### Requirement: 手动重探 API

系统 SHALL 提供 `POST /api/ai/health/reprobe` 端点：触发一次异步健康重探（复用 RunStartupProbe 全流程），立即返回（不等待探测完成）。若已有探测 in-flight，SHALL 返回跳过结果（幂等，不排队、不并发）。触发后快照由异步探测完成时更新，前端轮询 GET /api/ai/health 获取最新结果。本端点 SHALL NOT 依赖用户是否点过「恢复分析」。

#### Scenario: 手动触发重探成功

- **WHEN** 调用 POST /api/ai/health/reprobe，无探测 in-flight
- **THEN** 系统 SHALL 异步启动一次 RunStartupProbe 并返回 200（含 triggered=true）

#### Scenario: 探测 in-flight 时手动触发被跳过

- **WHEN** 调用 POST /api/ai/health/reprobe，恰有一次探测 in-flight
- **THEN** 系统 SHALL 返回 200（含 triggered=false、skipped=true），SHALL NOT 并发探测

#### Scenario: 手动重探不依赖恢复分析

- **WHEN** 分析处于暂停状态（用户暂停），调用 POST /api/ai/health/reprobe
- **THEN** 系统 SHALL 照常触发重探，SHALL NOT 改变分析暂停状态

#### Scenario: 手动重探仍遵守拉起冷却

- **WHEN** 手动重探发现某 provider 不可达，且该 provider 在 10 分钟冷却窗口内已被成功拉起
- **THEN** 重探 SHALL NOT 再次执行其 start_command，SHALL 仅继续轮询已拉起进程

