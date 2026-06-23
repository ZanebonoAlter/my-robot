## ADDED Requirements

### Requirement: Provider API Key 可空

系统 SHALL 允许 `openai_compatible` 类型的 provider 以空 API Key 配置并保存，以支持 llama.cpp / vLLM / LM Studio 等无认证的本地 OpenAI 兼容服务。系统 SHALL NOT 对 `openai_compatible` 类型强制要求非空 API Key；仅 `ollama` 类型本就无需 key。client 层 SHALL 在 API Key 为空时省略 `Authorization` 请求头。

#### Scenario: 无 key 的 openai_compatible provider 可创建
- **WHEN** 用户创建一个 `provider_type=openai_compatible`、`api_key` 为空的 provider
- **THEN** 系统 SHALL 成功保存该 provider，且 SHALL NOT 返回「api_key is required」错误

#### Scenario: 无 key provider 被 chat 调用时不发送认证头
- **WHEN** 路由对某 `api_key` 为空的 `openai_compatible` provider 发起 chat 请求
- **THEN** 发往该 provider BaseURL 的 HTTP 请求 SHALL NOT 包含 `Authorization` 头

#### Scenario: 前端不再强制校验 openai_compatible 的 key
- **WHEN** 用户在主模型或备用模型表单中保存一个 `provider_type=openai_compatible` 且 API Key 为空的配置
- **THEN** 前端 SHALL NOT 以「需要填写 API Key」阻断保存

### Requirement: 显式清除已保存的 API Key

系统 SHALL 提供显式清除已保存 API Key 的能力，通过 `clear_api_key` 布尔字段表达，消除「空串=沿用」语义下无法清空已存 key 的二义。当 `clear_api_key=true` 时，系统 SHALL 将该 provider 的 `APIKey` 置空；当 `clear_api_key` 为假或缺省且请求中的 `api_key` 为空时，系统 SHALL 沿用已保存的 key 不变。

#### Scenario: 清除已存 key
- **WHEN** 更新一个已存 provider，请求携带 `clear_api_key=true`
- **THEN** 系统 SHALL 将该 provider 的 `APIKey` 设为空字符串

#### Scenario: 不清除时沿用已存 key
- **WHEN** 更新一个已存 provider，请求未携带 `clear_api_key`（或为假）且 `api_key` 为空
- **THEN** 系统 SHALL 保持该 provider 原有的 `APIKey` 不变

#### Scenario: 前端提供清除密钥入口
- **WHEN** 用户编辑一个 `api_key_configured=true` 的备用 provider
- **THEN** 前端 SHALL 显示「清除密钥」操作入口；触发后 SHALL 以 `clear_api_key=true` 提交更新

### Requirement: enable_thinking 为输出清理标记（尽力而为）

`enable_thinking` 字段 SHALL 仅控制响应内容的 `<think>` 标签清理行为，SHALL NOT 声称在请求侧控制模型是否思考。`enable_thinking=true` 时系统 SHALL 对 chat 响应内容执行 `stripThinkTags`；`enable_thinking=false` 时 SHALL 跳过该清理。系统 SHALL NOT 向 provider 发送任何声称「关闭思考」的请求参数（包括但不限于 `reasoning_effort:"none"`）。

#### Scenario: 开启 thinking 时清理 think 标签
- **WHEN** 某 `enable_thinking=true` 的 provider 返回包含 `<think>...</think>` 的内容
- **THEN** 系统 SHALL 从返回给调用方的 `ChatResult.Content` 中剥离该标签

#### Scenario: 关闭 thinking 时保留原始内容
- **WHEN** 某 `enable_thinking=false` 的 provider 返回包含 `<think>...</think>` 的内容
- **THEN** 系统 SHALL 将原始内容（含标签）原样返回给调用方

#### Scenario: 不发送关闭思考参数
- **WHEN** 对任意 provider 发起 chat 请求
- **THEN** 请求 payload SHALL NOT 包含 `reasoning_effort` 字段（无论 `enable_thinking` 取值）

#### Scenario: DeepSeek 的独立 reasoning 字段无害
- **WHEN** provider 将思考内容放在独立的 `reasoning_content` 字段（如 DeepSeek）而非内嵌 `content`
- **THEN** 系统 SHALL 仅取 `content` 字段作为结果，`reasoning_content` SHALL 被忽略且不影响 `enable_thinking` 的清理行为
