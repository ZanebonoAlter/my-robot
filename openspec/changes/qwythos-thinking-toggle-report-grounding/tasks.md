## 1. 前置检查

- [ ] 1.1 确认 `cd backend-go && go vet ./...` 零错误
- [ ] 1.2 确认 `golangci-lint run ./...` 零错误（配置文件 `.golangci.yml` 存在）
- [ ] 1.3 确认 `go test ./internal/platform/airouter ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/platform/database` 可执行

## 2. airouter 透传 enable_thinking（TDD 切片1）

- [ ] 2.1 RED：在 `openai_compatible_test.go` 新增测试，构造 `EnableThinking=true` 的 provider，断言生成的请求 payload 含 `chat_template_kwargs.enable_thinking=true`；另构造 `EnableThinking=false`，断言 payload 不含 `chat_template_kwargs`。先看到测试失败（当前代码无透传）。
- [ ] 2.2 GREEN：在 `openai_compatible.go` payload 构建后追加 `if provider.EnableThinking { payload["chat_template_kwargs"] = map[string]any{"enable_thinking": true} }`，使测试通过。
- [ ] 2.3 移除事后剥标签：删除 `openai_compatible.go` 中 `if provider.EnableThinking { content = stripThinkTags(content) }` 调用；保留 `stripThinkTags` 函数本体与正则（防御性保留，对应测试 `TestStripThinkTags` 仍通过）。
- [ ] 2.4 为 `models/ai_models.go` 的 `AIProvider.EnableThinking` 字段补注释，说明新语义（控制模型思考，透传 chat_template_kwargs）。

## 3. 思考开关语义反转迁移（切片2）

- [ ] 3.1 在 `postgres_migrations.go` 新增幂等 migration `20260626_0001`：`UPDATE ai_providers SET enable_thinking = false`，Description 注明语义反转原因。
- [ ] 3.2 人工验证：迁移可在已执行库上重复执行不报错（幂等）。

## 4. 日报 context 注入（TDD 切片3 — 数据层）

- [ ] 4.1 在 `daily_report_models.go` 的 `TagInput` 新增字段 `ArticleContext string json:"article_context,omitempty"`，注释说明其承载代表性文章标题+摘要用于 LLM prompt。
- [ ] 4.2 RED：在 `daily_report_orchestrator_test.go` 新增测试，断言辅助函数 `buildArticleContextForTag` 对给定 tag 与文章返回形如 `《标题》摘要...` 的拼接文本，且单篇摘要被截断到 ~200 rune、最多取 2-3 篇。先看到失败（函数不存在）。
- [ ] 4.3 GREEN：在 `daily_report_orchestrator.go` 新增 `buildArticleContextForTag`，摘要优先级沿用 `buildArticleSummary`（AIContentSummary > FirecrawlContent > Content > Description），每篇 rune 截断 ~200、最多取前 3 篇。
- [ ] 4.4 改造 `collectBoardTags` 主路径（`:312-318`）：将 pluck 文章 ID 改为同时 SELECT 标题与摘要字段，调用 `buildArticleContextForTag` 填入 `TagInput.ArticleContext`。
- [ ] 4.5 改造 `collectBoardTags` fallback 路径（`:359-364`）：同样填充 `TagInput.ArticleContext`。

## 5. 日报 prompt 注入（TDD 切片4）

- [ ] 5.1 RED：在 `daily_report_llm_test.go`（或 orchestrator 同包测试）新增测试，断言 `buildHighlightsPrompt` 在某 tag 的 `ArticleContext` 非空时，输出包含该 context 文本；`ArticleContext` 为空时不输出该行。先看到失败。
- [ ] 5.2 GREEN：在 `buildHighlightsPrompt`（`daily_report_llm.go:87`）每个 tag 行后，以 `if t.ArticleContext != "" {...}` 守卫追加代表文章信息。这是头条混淆的核心修复点。
- [ ] 5.3 GREEN：对 `buildThreadsPrompt`（`daily_report_llm.go:200`）做同样守卫注入。
- [ ] 5.4 GREEN：对 `buildClusterPrompt`（`daily_report_cluster.go:146`）做同样守卫注入。

## 6. 测试

- [ ] 6.1 `cd backend-go && go test ./internal/platform/airouter → PASS`
- [ ] 6.2 `cd backend-go && go test ./internal/topicgraph/service → PASS`
- [ ] 6.3 `cd backend-go && go test ./internal/topicgraph/repository → PASS`
- [ ] 6.4 `cd backend-go && go test ./internal/platform/database → PASS`

## 7. 文档

- [ ] 7.1 更新 `docs/reference/configuration.md`：说明 `enable_thinking` provider 字段的新语义（开启模型思考），及「配两条 provider 指向同服务实现打标签/日报差异化」的运维配置步骤。
- [ ] 7.2 更新 `docs/reference/architecture/map.md`：在日报/airouter 入口索引中补充思考开关与 context 注入的代码入口（如需）。

## 8. 验证

- [ ] 8.1 `cd backend-go && golangci-lint run ./... → 0 error`
- [ ] 8.2 `cd backend-go && go vet ./... → 0 error`
- [ ] 8.3 `cd backend-go && go build ./... → 编译成功`
- [ ] 8.4 `cd backend-go && go test ./internal/platform/airouter ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/platform/database → PASS`
- [ ] 8.5 `grep -rn "chat_template_kwargs" backend-go/internal/platform/airouter → 命中透传点（openai_compatible.go）`
- [ ] 8.6 `grep -rn "ArticleContext" backend-go/internal/topicgraph → 命中 TagInput 字段定义、collectBoardTags 填充、三处 prompt 注入`
- [ ] 8.7 `grep -rn "stripThinkTags(content)" backend-go/internal/platform/airouter/openai_compatible.go → 零命中（已移除事后调用，函数本体保留）`
- [ ] 8.8 `bash scripts/check-standards.sh → L1 规范验收零失败`
- [ ] 8.9 运维侧端到端实测（代码外，部署后执行）：后台建 `qwythos-think`(enable_thinking=true) 挂 `digest_polish`、`qwythos-nothink`(false) 挂 `topic_tagging`；触发日报生成确认 highlights 不再混淆；观察打标签请求无 reasoning 产出、token 下降。
