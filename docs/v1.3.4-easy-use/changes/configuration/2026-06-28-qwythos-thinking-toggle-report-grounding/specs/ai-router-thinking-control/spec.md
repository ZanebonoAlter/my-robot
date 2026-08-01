## ADDED Requirements

### Requirement: Provider 配置控制模型思考开关

`AIProvider.EnableThinking` 字段 SHALL 表达「是否让被调用的模型进行推理思考」，由 airouter 在 OpenAI 兼容请求体中透传 `chat_template_kwargs.enable_thinking` 实现。无论 `EnableThinking` 取 true 还是 false，请求体 MUST 始终包含 `chat_template_kwargs.enable_thinking` 字段并取该布尔值——即 true 时 `{"enable_thinking": true}`，false 时 `{"enable_thinking": false}`。

> 为何 false 也要显式发：Qwen3/Qwythos 的 chat template 在该 kwarg **缺失**时走默认分支（开启思考），只有在显式发 `enable_thinking=false` 时才预置空 `<think></think>` 关闭思考。因此「不发参数」≠「关思考」，而是 = 开思考。per-request 钉死才能让开关真正生效。

airouter SHALL NOT 对响应做基于 `EnableThinking` 的事后 `<think>` 标签剥除（服务器已将思考内容分离到 `reasoning_content` 字段，`content` 为干净答案）。

#### Scenario: EnableThinking 为 true 时透传开思考参数

- **WHEN** 一个 `EnableThinking=true` 的 provider 被 airouter 调用
- **THEN** 发往该 provider 的 HTTP 请求体中包含 `chat_template_kwargs.enable_thinking` 等于 `true`

#### Scenario: EnableThinking 为 false 时显式发送关思考参数

- **WHEN** 一个 `EnableThinking=false` 的 provider 被 airouter 调用
- **THEN** 发往该 provider 的 HTTP 请求体中包含 `chat_template_kwargs.enable_thinking` 等于 `false`（必须显式发送，否则模型会回退到模板默认即开思考）

#### Scenario: 不再做基于开关的事后标签剥除

- **WHEN** provider 返回的 `content` 字段中不含 `<think>` 标签（思考已分离到 reasoning_content）
- **THEN** airouter 直接返回该 `content`，不调用 `stripThinkTags`

### Requirement: 思考开关语义反转的数据迁移

发布时 SHALL 执行幂等迁移，将 `ai_providers.enable_thinking` 全部置为 `false`，以兜底旧语义（事后剥标签）反转为新语义（开启思考）带来的误开启风险。该迁移 MUST 可重复执行。

#### Scenario: 迁移将所有 provider 的思考开关清零

- **WHEN** 迁移执行
- **THEN** `ai_providers` 表中所有行的 `enable_thinking` 列变为 `false`

#### Scenario: 迁移幂等可重复执行

- **WHEN** 迁移在已执行过的库上再次执行
- **THEN** 不报错，所有行 `enable_thinking` 仍为 `false`
