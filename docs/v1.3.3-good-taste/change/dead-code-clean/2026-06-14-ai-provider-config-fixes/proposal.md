## Why

AI 模型全局配置中存在两个正确性缺陷：`enable_thinking` 开关在请求侧发送的 `reasoning_effort:"none"` 是非法枚举且对 llama.cpp/DeepSeek 均无意义，等于一个不工作的死开关；而 OpenAI 兼容格式被三处硬校验强制要求 API Key，导致本地 llama.cpp 这类无 key 服务无法配置。两者同属「AI provider 配置正确性」，且改动面高度重叠（`AIProvider` 模型、`openai_compatible.go`、`ai_handler.go`、前端面板与 composable），合并为一个 change 处理。

## What Changes

### Thinking 适配

- **移除** `openai_compatible.go` 中 `reasoning_effort:"none"` 的请求侧写入（非法枚举，对目标后端无意义）。
- **重新定义** `enable_thinking` 语义为「输出清理标记」：仅控制是否对响应内容执行 `<think>` 标签剥离。**尽力而为语义**——不保证请求侧能关闭思考（DeepSeek reasoner 永远思考，llama.cpp 由模型决定），仅保证输出干净。
- `enable_thinking=true` 时调用 `stripThinkTags`；`enable_thinking=false` 时跳过剥离。DeepSeek 的 `reasoning_content` 独立字段因不污染 `content`，自动无害。

### API Key 可选

- **移除** `store.go` 中 `ProviderType != Ollama && APIKey == ""` 的强制校验，允许 `openai_compatible` 类型空 key。
- **移除** `useAIRouterSettings.ts` 三处 `!ollama && !api_key` 前端硬校验（savePrimaryProvider / saveNewProvider / testPrimaryProvider）。
- client 层已有 `if provider.APIKey != "" { set Authorization }`，空 key 不发认证头，llama.cpp 直接可用，无需改动。
- **BREAKING（DB 标签）**：`AIProvider.APIKey` 的 `not null` GORM 标签去除，表意为「可空」。空串合法，已存数据不受影响。

### 清空密钥语义（修复更新时的二义性）

- `UpsertProviderRequest` 新增 `clear_api_key bool` 显式字段。
- `UpdateProvider` 处理逻辑：`clear_api_key=true` 时将 `APIKey` 置空；否则沿用现状（空串 = 保持不变）。
- 前端编辑备用 provider 表单新增「清除密钥」按钮（仅 edit 场景，仅当 `api_key_configured=true` 时显示）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `ai-capability-routing`: 新增 provider 密钥可空性与密钥清除的需求；新增 `enable_thinking` 的语义契约（输出清理标记，尽力而为，不保证请求侧关闭思考）。

## Impact

- **后端**：`internal/platform/airouter/openai_compatible.go`（删 reasoning_effort、调整 stripThinkTags 调用条件）；`internal/platform/airouter/store.go`（删 APIKey 强制校验）；`internal/admin/handler/ai_handler.go`（新增 `clear_api_key` 字段处理）；`internal/models/ai_models.go`（GORM 标签 `not null` 去除）。相应测试同步更新。
- **前端**：`front/app/features/ai/composables/useAIRouterSettings.ts`（删三处校验、表单增加 `clear_api_key`）；`front/app/features/ai/components/AIRouterBackupProviders.vue`（编辑表单增加「清除密钥」按钮）；`front/app/features/ai/components/AIProviderManagement.vue`（主模型 Thinking 文案/提示调整）；相关类型定义（`AIProviderUpsertRequest`）。
- **DB**：`ai_providers.api_key` 列约束语义变化（`NOT NULL` 在 PostgreSQL 层面：空串本就可写入，无需迁移，但 GORM AutoMigrate 的标签需对齐）。已有数据不受影响。
- **无 API 路径变更**：所有端点路径不变，仅 DTO 增加可选字段、校验放宽。
