## Why

全局设置里的"能力路由"面板配置了 `summary`（文章总结）、`digest_polish`（日报润色）等多个 provider 路由，但后端代码从未真正读取这两条路由——文章总结实际走 `article_completion`，日报生成寄生在 `topic_tagging`。结果是用户配置的"文章总结"和"日报润色"两条路由完全不生效，且日报无法独立配置 provider / 并发 / 温度，被迫与标签提取挤同一条路由。与此同时 `article_completion` 路由配置项在前端可见却与产品语义脱节（UI 上用户看到的只有"自动总结"，并无"正文补全"概念），`AISummaryResponse` 还保留着从未被填充的结构化字段（KeyPoints / Takeaways / Tags）。

## What Changes

- **接线 `summary`**：`summarizeContent` 的调用 capability 从 `article_completion` 改为 `summary`，让"文章总结"路由真正驱动文章自动总结。
- **接线 `digest_polish`**：新增 `CapabilityDigestPolish` 常量；日报生成的 4 处 LLM 调用（`daily_report_llm.go` ×3、`daily_report_cluster.go` ×1）从 `topic_tagging` 迁移到 `digest_polish`；`defaultConcurrency` 为 `digest_polish` 增加独立配额。
- **废弃 `article_completion`**（**BREAKING**）：删除 `CapabilityArticleCompletion` 常量与 `defaultConcurrency` 条目；前端移除 `article_completion` 路由标签与 `capabilityOrder` 条目。数据库中既有的 `ai_routes` 行不会报错，但该路由在面板上不再显示。
- **清理 `AISummaryResponse` 死代码**：结构体瘦身为仅保留 `Markdown` 字段，移除 `OneSentence` / `KeyPoints` / `Takeaways` / `Tags`（结构化提取由 `topic_tagging` 承担，摘要不需要重复）；连带移除 `formatAISummary` 中永远不可达的结构化拼接分支、`ParseSummaryMarkdown` 的 OneSentence 计算、以及只服务于该分支的 `markdownToPlainText`。`summarizeContent` 直接返回 `string`。

## Capabilities

### New Capabilities
- `ai-capability-routing`: AI 能力路由（capability）与业务用途的绑定契约，定义每个 capability 服务于哪个业务流程、其默认并发配额与可独立配置的维度。本次明确 `summary`→文章总结、`digest_polish`→日报生成、`article_completion`→废弃。

### Modified Capabilities
- `daily-report-system`: 日报生成的 AI 调用从复用 `topic_tagging` 路由改为使用独立的 `digest_polish` 路由，使日报可单独配置 provider、并发与温度，不再与标签提取共享配额。

## Impact

- **后端**：
  - `internal/platform/airouter/store.go`（新增 `CapabilityDigestPolish`、删除 `CapabilityArticleCompletion`）
  - `internal/platform/airouter/router.go`（`defaultConcurrency` 调整）
  - `internal/platform/airouter/fallback.go`（`AISummaryResponse` 瘦身、移除死函数）
  - `internal/reader/service/content_completion_service.go`（`summarizeContent` 改 capability + 简化返回类型、`formatAISummary` 简化）
  - `internal/topicgraph/service/daily_report_llm.go`、`daily_report_cluster.go`（4 处调用迁移到 `digest_polish`）
- **前端**：`app/features/ai/composables/useAIRouterSettings.ts`（移除 `article_completion` 标签与 `capabilityOrder` 条目）
- **测试**：`content_completion_service_test.go`、`daily_report_*_test.go`、`router_test.go`、`store_test.go`、`fallback` 相关测试同步调整
- **数据库**：无 schema 变更；既有 `article_completion` 路由行残留但不影响运行（可选清理指南见 design）
