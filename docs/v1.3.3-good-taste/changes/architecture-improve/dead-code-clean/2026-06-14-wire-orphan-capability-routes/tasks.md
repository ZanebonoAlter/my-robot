## 1. 新增 capability 常量与并发配额

- [x] 1.1 在 `internal/platform/airouter/store.go` 新增 `CapabilitySummary Capability = "summary"` 与 `CapabilityDigestPolish Capability = "digest_polish"` 常量
- [x] 1.2 在 `internal/platform/airouter/router.go` 的 `defaultConcurrency` 中新增 `CapabilityDigestPolish: 2`；`CapabilitySummary` 沿用兜底值 3 或按需显式设定
- [x] 1.3 验证：`go build ./internal/platform/airouter/...` 通过

## 2. 接线 summary（文章总结）

- [x] 2.1 在 `internal/reader/service/content_completion_service.go` 的 `summarizeContent` 中，将 `Capability: airouter.CapabilityArticleCompletion` 改为 `Capability: airouter.CapabilitySummary`
- [x] 2.2 同步调整 `summarizeContent` 的返回类型为 `string`（直接返回 `result.Content`），移除 `AISummaryResponse` 包装（依赖任务 4 完成，可先保留签名待 4 收尾）
- [x] 2.3 调整 `CompleteArticleWithMetadata` 中对 `summarizeContent` 返回值的使用（`formatAISummary(summary)` → 直接赋值/trim）
- [x] 2.4 验证：`go test ./internal/reader/service/...`（content_completion 相关）

## 3. 接线 digest_polish（日报生成）

- [x] 3.1 在 `internal/topicgraph/service/daily_report_llm.go` 将 3 处 `Capability: airouter.CapabilityTopicTagging`（GenerateHighlights、GenerateNarrative、第三个调用）改为 `Capability: airouter.CapabilityDigestPolish`
- [x] 3.2 在 `internal/topicgraph/service/daily_report_cluster.go` 将聚类调用 `Capability: airouter.CapabilityTopicTagging` 改为 `Capability: airouter.CapabilityDigestPolish`
- [x] 3.3 验证：`go test ./internal/topicgraph/service/...`（daily_report 相关）

## 4. 清理 AISummaryResponse 死代码

- [x] 4.1 在 `internal/platform/airouter/fallback.go` 将 `AISummaryResponse` 结构体瘦身为仅保留 `Markdown string` 字段
- [x] 4.2 移除 `ParseSummaryMarkdown` 函数（仅服务死分支）
- [x] 4.3 移除 `markdownToPlainText` 函数（仅服务已删的 `OneSentence` 计算）
- [x] 4.4 同步简化 `AIService.SummarizeArticle` 的返回（不再构造 `AISummaryResponse`，直接返回 Markdown 字符串或按 2.2 收口后由调用方处理）
- [x] 4.5 在 `content_completion_service.go` 简化 `formatAISummary`（移除 `Markdown` 为空时的结构化拼接分支）或内联删除
- [x] 4.6 验证：`go build ./internal/platform/airouter/... ./internal/reader/service/...`

## 5. 废弃 article_completion

- [x] 5.1 确认全仓库无生产代码引用 `CapabilityArticleCompletion`（`grep -rn CapabilityArticleCompletion --include="*.go" internal/`）
- [x] 5.2 删除 `internal/platform/airouter/store.go` 中的 `CapabilityArticleCompletion` 常量
- [x] 5.3 删除 `internal/platform/airouter/router.go` 的 `defaultConcurrency` 中 `CapabilityArticleCompletion` 条目
- [x] 5.4 更新测试中引用 `CapabilityArticleCompletion` 的构造（改为 `CapabilitySummary` 或 `CapabilityDigestPolish`）
- [x] 5.5 验证：`go build ./...` 与 `go test ./internal/platform/airouter/... ./internal/reader/service/...`

## 6. 前端调整

- [x] 6.1 在 `app/features/ai/composables/useAIRouterSettings.ts` 的 `routeLabels` 中移除 `article_completion: '正文补全'`
- [x] 6.2 在同文件 `capabilityOrder` 数组中移除 `'article_completion'`
- [x] 6.3 验证：`pnpm lint`（WSL 可用）；typecheck/build 走 Windows cmd

## 7. 文档与收尾

- [x] 7.1 在 `docs/` 知识库记录能力路由绑定变更（summary/digest_polish 接线、article_completion 废弃）
- [x] 7.2 在 design.md 记录的手动清理 SQL（删除 `ai_routes` 中 `capability='article_completion'` 残留行）作为可选运维步骤说明，不强制执行
- [x] 7.3 全量验证：`go test ./internal/platform/airouter/... ./internal/reader/service/... ./internal/topicgraph/service/...` 与 `go build ./...`
