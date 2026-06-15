## 1. 后端：移除无效的 thinking 请求参数

- [x] 1.1 在 `backend-go/internal/platform/airouter/openai_compatible.go` 的 `Chat` 方法中，删除 `if !provider.EnableThinking { payload["reasoning_effort"] = "none" }` 这段代码块
- [x] 1.2 将 `content = stripThinkTags(content)` 改为按 `provider.EnableThinking` 条件执行：`true` 时调用 `stripThinkTags`，`false` 时保留原始 `content`
- [x] 1.3 验证：检查 `Chat` 发出的 payload 不再包含 `reasoning_effort` 字段（单测或日志确认）

## 2. 后端：放宽 API Key 校验

- [x] 2.1 在 `backend-go/internal/platform/airouter/store.go` 的 `UpsertProvider` 中，删除 `if provider.ProviderType != ProviderTypeOllama && provider.APIKey == ""` 校验分支
- [x] 2.2 在 `backend-go/internal/models/ai_models.go` 中，将 `AIProvider.APIKey` 的 GORM 标签从 `gorm:"type:text;not null"` 改为 `gorm:"type:text"`
- [x] 2.3 更新 `backend-go/internal/platform/airouter/store_test.go`：确认/新增用例「`openai_compatible` 空 key 可创建成功」

## 3. 后端：新增 clear_api_key 字段

- [x] 3.1 在 `backend-go/internal/admin/handler/ai_handler.go` 的 `UpsertProviderRequest` 结构体增加 `ClearAPIKey bool \`json:"clear_api_key"\``
- [x] 3.2 在 `UpdateProvider` handler 中，实现：`req.ClearAPIKey==true` 时 `provider.APIKey = ""`；否则保持现有「空串=沿用」逻辑
- [x] 3.3 注意：`UpsertProvider`（创建路径）不处理 `clear_api_key`（新建无 key 可清），仅 `UpdateProvider` 处理
- [x] 3.4 验证：新增/更新 `ai_handler_test.go` 用例覆盖「清除」「沿用」两种路径

## 4. 后端：编译与测试

- [x] 4.1 运行 `cd backend-go && go vet ./internal/platform/airouter/... ./internal/admin/handler/... ./internal/models/...`
- [x] 4.2 运行 `cd backend-go && golangci-lint run ./internal/platform/airouter/... ./internal/admin/handler/...`
- [x] 4.3 运行 `cd backend-go && go test ./internal/platform/airouter/... ./internal/admin/handler/...`
- [x] 4.4 运行 `cd backend-go && go build ./...`

## 5. 前端：移除 API Key 硬校验

- [x] 5.1 在 `front/app/features/ai/composables/useAIRouterSettings.ts` 的 `savePrimaryProvider` 中，删除 `!isOllama && !primaryProviderId.value && !primaryProviderForm.api_key` 校验分支
- [x] 5.2 在 `saveNewProvider` 中，删除 `!isOllama && !newProviderForm.api_key` 校验分支
- [x] 5.3 在 `testPrimaryProvider` 中，删除 `!isOllama && !primaryProviderForm.api_key` 校验分支
- [x] 5.4 在 `front/app/types` 中为 `AIProviderUpsertRequest` 增加 `clear_api_key?: boolean` 可选字段

## 6. 前端：编辑表单增加「清除密钥」按钮

- [x] 6.1 在 `useAIRouterSettings.ts` 的 `editProviderForm` reactive 对象增加 `clear_api_key: false` 字段
- [x] 6.2 在 `startEditingProvider` 中重置 `editProviderForm.clear_api_key = false`
- [x] 6.3 在 `cancelEditingProvider` 中重置 `editProviderForm.clear_api_key = false`
- [x] 6.4 在 `front/app/features/ai/components/AIRouterBackupProviders.vue` 的编辑表单中，增加「清除密钥」按钮：仅当当前编辑 provider 的 `api_key_configured===true` 时显示；点击置 `editProviderForm.clear_api_key = true`
- [x] 6.5 「清除密钥」按钮点击后给视觉反馈（如文案变「将清除已保存密钥」或按钮禁用），避免误触

## 7. 前端：Thinking 文案与提示调整

- [x] 7.1 在 `AIProviderManagement.vue` 中，将主模型 Thinking toggle 的文案从「启用 Thinking（推理模型的思考过程，会消耗额外 token）」调整为反映「输出清理标记」语义的文案（如「清理推理输出（剥离 `<think>` 标签，适用于 llama.cpp/Qwen3 等内嵌推理的模型）」）
- [x] 7.2 在 `AIRouterBackupProviders.vue` 的 Thinking toggle 同步文案调整
- [x] 7.3 可选：在 Thinking toggle 旁加简短提示「DeepSeek/OpenAI o 系等独立推理字段的模型无需开启」

## 8. 前端：验证

- [x] 8.1 运行 `cd front && pnpm lint`
- [x] 8.2 运行 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
- [x] 8.3 运行 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`

## 9. 文档

- [x] 9.1 确认 `docs/reference/` 下是否有 AI provider 配置文档需要同步（API Key 可空、enable_thinking 语义）
- [x] 9.2 如有相关文档，更新「API Key 必填」为「可选（本地无认证服务留空）」、「Thinking 开关」语义说明
