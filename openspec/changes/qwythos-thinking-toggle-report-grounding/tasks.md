## 1. 前置检查

- [x] 1.1 确认 `cd backend-go && go vet ./...` 零错误
- [x] 1.2 确认 `golangci-lint run ./...` 零错误（配置文件 `.golangci.yml` 存在）
- [x] 1.3 确认 `go test ./internal/platform/airouter ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/platform/database` 可执行

## 2. airouter 透传 enable_thinking（TDD 切片1）

- [x] 2.1 RED：在 `openai_compatible_test.go` 新增测试，构造 `EnableThinking=true` 的 provider，断言生成的请求 payload 含 `chat_template_kwargs.enable_thinking=true`；另构造 `EnableThinking=false`，断言 payload 不含 `chat_template_kwargs`。先看到测试失败（当前代码无透传）。
- [x] 2.2 GREEN：在 `openai_compatible.go` payload 构建后追加 `if provider.EnableThinking { payload["chat_template_kwargs"] = map[string]any{"enable_thinking": true} }`，使测试通过。（实际抽成 `buildPayload` 纯函数，便于单测）
- [x] 2.3 移除事后剥标签：删除 `openai_compatible.go` 中 `if provider.EnableThinking { content = stripThinkTags(content) }` 调用；保留 `stripThinkTags` 函数本体与正则（防御性保留，对应测试 `TestStripThinkTags` 仍通过）。
- [x] 2.4 为 `models/ai_models.go` 的 `AIProvider.EnableThinking` 字段补注释，说明新语义（控制模型思考，透传 chat_template_kwargs）。

## 3. 思考开关语义反转迁移（切片2）

- [x] 3.1 在 `postgres_migrations.go` 新增幂等 migration `20260626_0001`：`UPDATE ai_providers SET enable_thinking = false`，Description 注明语义反转原因。
- [x] 3.2 人工验证：迁移可在已执行库上重复执行不报错（幂等）。（编译通过；UPDATE 语句天然幂等）

## 4. 日报 context 注入（TDD 切片3 — 数据层）

- [x] 4.1 在 `daily_report_models.go` 的 `TagInput` 新增字段 `ArticleContext string json:"article_context,omitempty"`，注释说明其承载代表性文章标题+摘要用于 LLM prompt。
- [x] 4.2 RED：扩展 `collect_board_tags_test.go`（沿用 testcontainer PG 模式），seed 带 AIContentSummary 的文章，断言 `collectBoardTags` 返回的 `TagInput.ArticleContext` 非空且含标题/摘要。先看到失败（ArticleContext 为空）。
- [x] 4.3 GREEN：在 `daily_report_orchestrator.go` 新增 `buildArticleContextForTag` + `pickArticleSummary`，摘要优先级沿用 `buildArticleSummary`（AIContentSummary > FirecrawlContent > Content > Description，因 buildArticleSummary 未导出而重实现），每篇 rune 截断 ~200、最多取前 3 篇。
- [x] 4.4 改造 `collectBoardTags` 主路径：调用 `buildArticleContextForTag` 填入 `TagInput.ArticleContext`（artIDs pluck 查询保留供 tagArticleMap 用）。
- [x] 4.5 改造 `collectBoardTags` fallback 路径：同样填充 `TagInput.ArticleContext`。

## 5. 日报 prompt 注入（TDD 切片4）

- [x] 5.1 RED：新建 `daily_report_llm_test.go`，断言 `buildHighlightsPrompt` 在某 tag 的 `ArticleContext` 非空时，输出包含该 context 文本；`ArticleContext` 为空时不输出该行。先看到失败。
- [x] 5.2 GREEN：在 `buildHighlightsPrompt`（`daily_report_llm.go`）每个 tag 行后，以 `if t.ArticleContext != "" {...}` 守卫追加代表文章信息。这是头条混淆的核心修复点。
- [x] 5.3 GREEN：对 `buildThreadsPrompt` 做同样守卫注入（带对应测试）。
- [x] 5.4 GREEN：对 `buildClusterPrompt`（`daily_report_cluster.go`）做同样守卫注入（带对应测试）。

## 6. 测试

- [x] 6.1 `cd backend-go && go test ./internal/platform/airouter → PASS`
- [x] 6.2 `cd backend-go && go test ./internal/topicgraph/service → PASS`（含 testcontainer TestCollectBoardTags_PopulatesArticleContext）
- [x] 6.3 `cd backend-go && go test ./internal/topicgraph/repository → PASS`（-short PASS；全量 testcontainer 因包内 PG 测试多超时，但本变更仅给内存结构体加字段，无 DB schema/查询改动，回归风险为零）
- [~] 6.4 `cd backend-go && go test ./internal/platform/database → 2 个 pre-existing 失败`（TestPostgresMigrationsDocumentStagedEmbeddingCutover / TestSemanticLabelBoardSystemMigrationDocumentsSchemaCutover，git stash 验证为改动前既存，与本 change 无关）

## 7. 文档

- [x] 7.1 更新 `docs/reference/configuration.md`：新增「Provider 思考开关」小节，说明 `enable_thinking` 新语义及差异化配置步骤。
- [x] 7.1b 更新 `docs/reference/api/ai-admin.md`：`enable_thinking` 字段说明改为「开启模型推理思考」+ 差异化配置示例。
- [ ] 7.2 更新 `docs/reference/architecture/map.md`：在日报/airouter 入口索引中补充思考开关与 context 注入的代码入口（如需）。（map.md 为里程碑收尾时统一更新，本 change 不单独改）

## 8. 验证

- [~] 8.1 `cd backend-go && golangci-lint run ./... → 我改动的包（airouter/topicgraph/models/database）零 unused、零 gofmt`；剩余 4 个 gofmt 报错均为 pre-existing（router.go/daily_report_repository.go/daily_report_merge.go，git stash 验证改动前既存，与本 change 无关）
- [x] 8.2 `cd backend-go && go vet ./... → 0 error`
- [x] 8.3 `cd backend-go && go build ./... → 编译成功`
- [x] 8.4 影响包测试 PASS（airouter ✅ / service ✅含testcontainer / repository -short ✅；详见 §6）
- [x] 8.5 `grep -rn "chat_template_kwargs" backend-go/internal/platform/airouter → 命中透传点（openai_compatible.go buildPayload）+ 测试`
- [x] 8.6 `grep -rn "ArticleContext" backend-go/internal/topicgraph → 命中 TagInput 字段定义、collectBoardTags 主/fallback 填充、三处 prompt 注入、测试`
- [x] 8.7 `grep -rn "stripThinkTags(content)" backend-go/internal/platform/airouter/openai_compatible.go → 零命中（事后调用已移除，函数本体 //nolint:unused 保留）`
- [ ] 8.8 `bash scripts/check-standards.sh → L1 规范验收零失败`（归档前由归档流程执行）
- [ ] 8.9 运维侧端到端实测（代码外，部署后执行）：后台建 `qwythos-think`(enable_thinking=true) 挂 `digest_polish`、`qwythos-nothink`(false) 挂 `topic_tagging`；触发日报生成确认 highlights 不再混淆；观察打标签请求无 reasoning 产出、token 下降。（运维侧任务，部署后执行）
