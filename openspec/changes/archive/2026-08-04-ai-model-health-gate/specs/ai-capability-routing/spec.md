## ADDED Requirements

### Requirement: Provider 模型类型区分（model_kind）

每个 `AIProvider` SHALL 声明其模型功能类型 `model_kind`，取值 `llm` 或 `embedding`，默认 `llm`。`model_kind` 表达模型功能（LLM 推理 / 向量嵌入），与表达协议的 `provider_type`（openai_compatible / ollama）正交，互不替代。provider 列表 API（`GET /api/ai/providers`）SHALL 返回每个 provider 的 `model_kind`；provider 创建/更新（`POST /api/ai/providers`、`PUT /api/ai/providers/:id`）SHALL 接受并持久化 `model_kind`，缺省按 `llm`。

#### Scenario: 新建 provider 默认 llm

- **WHEN** 用户创建一个 provider，未传 model_kind
- **THEN** 系统 SHALL 保存 model_kind=llm

#### Scenario: 显式声明 embedding 类型

- **WHEN** 用户创建 provider，传 model_kind=embedding
- **THEN** 系统 SHALL 保存 model_kind=embedding

#### Scenario: 列表返回 model_kind

- **WHEN** 调用 GET /api/ai/providers
- **THEN** 每个 provider 对象 SHALL 包含 model_kind 字段（llm 或 embedding）

#### Scenario: model_kind 与 provider_type 正交

- **WHEN** 一个 provider_type=openai_compatible、model_kind=embedding 的本地 llama.cpp embedding 服务
- **THEN** 系统 SHALL 接受该组合，两者互不冲突

### Requirement: 路由绑定的模型类型约束

系统 SHALL 在路由绑定 provider 时按 capability 类别校验 provider 的 `model_kind`：`embedding` capability 路由 SHALL 只绑定 `model_kind=embedding` 的 provider；其余（llm 类）capability 路由 SHALL 只绑定 `model_kind=llm` 的 provider。违反时系统 SHALL 拒绝保存并返回明确错误（指明冲突的 provider 名与期望类型）。embedding 任务 SHALL 且仅 SHALL 使用 embedding 类型的模型。

#### Scenario: embedding 路由拒绝 llm provider

- **WHEN** 保存 embedding capability 路由，绑定的某 provider model_kind=llm
- **THEN** 系统 SHALL 拒绝保存，返回错误（如「embedding 路由不能挂 llm 模型『X』」），路由绑定不变

#### Scenario: llm 路由拒绝 embedding provider

- **WHEN** 保存 summary capability 路由，绑定的某 provider model_kind=embedding
- **THEN** 系统 SHALL 拒绝保存，返回错误

#### Scenario: 同类绑定放行

- **WHEN** 保存 embedding 路由绑定 model_kind=embedding 的 provider，或 summary 路由绑定 model_kind=llm 的 provider
- **THEN** 系统 SHALL 接受保存

#### Scenario: 存量迁移后冲突告警

- **WHEN** 版本化迁移执行后，存在「同一 provider 同时绑定在 embedding 路由与 llm 路由」的存量数据
- **THEN** 迁移 SHALL 以日志告警列出冲突 provider，且后续任何一次该路由的保存 SHALL 被类型校验拦截，直至用户手动修正

### Requirement: Provider 本地启动命令（start_command）

每个 `AIProvider` MAY 声明一个可选的本地启动命令 `start_command`（文本，可空）。`start_command` 有值 SHALL 表达「该 provider 是需被托管的本地进程」（如 llama.cpp 的 `llama-server` 启动行）；为空 SHALL 表达「该 provider 是外部已托管服务，不 attempts 拉起」。provider 列表 API SHALL 返回 `start_command` 是否已配置（不回显原文以避免无谓泄露时，至少返回 `start_command_configured` 布尔）；provider 创建/更新 SHALL 接受并持久化 `start_command`。

#### Scenario: 配置本地启动命令

- **WHEN** 用户为一个 llama.cpp provider 填写 start_command=`llama-server -m D:/models/qwen.gguf --port 8081`
- **THEN** 系统 SHALL 保存该命令，标记该 provider 为可托管本地进程

#### Scenario: 外部服务留空

- **WHEN** 用户为一个云端 OpenAI 兼容 provider 不填 start_command
- **THEN** 系统 SHALL 保存空值，该 provider 不被视为本地进程

#### Scenario: 列表回显配置态

- **WHEN** 调用 GET /api/ai/providers
- **THEN** 每个 provider 对象 SHALL 至少包含表示 start_command 是否已配置的字段（start_command 原文或 start_command_configured 布尔）
