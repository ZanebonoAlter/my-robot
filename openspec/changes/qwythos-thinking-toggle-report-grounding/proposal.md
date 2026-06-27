## Why

换用 Qwythos-9B-Claude-Mythos-5-1M（Qwen3.5-9B 底座、1M 上下文、原生 tool calling）后，本地 llama-server 上能否做到「打标签不思考、生成日报思考」，取决于请求体是否透传 `chat_template_kwargs.enable_thinking`。实测证明该 per-request 开关稳定生效，但后端 `airouter` 从未透传它——`AIProvider.EnableThinking` 字段当前只做「事后剥除 `<think>` 标签」，模型是否思考完全由 CLI 全局参数决定，无法按 capability 差异化。同时日报头条长期混淆：`buildHighlightsPrompt` 只喂「标签名 + 文章数」，LLM 看不见事件内容；换 1M 上下文模型后，把代表性文章标题/摘要直接注入 prompt 即可根除该问题。

## What Changes

- **修正 `AIProvider.EnableThinking` 语义**：从「事后剥除 think 标签」改为「真正开启模型思考」——在 `openai_compatible.go` 的请求体里透传 `chat_template_kwargs.enable_thinking=true`；移除事后 `stripThinkTags` 调用（服务器已把思考分离到 `reasoning_content`，`content` 本就干净）。
- **数据迁移统一清零**：新增幂等 versioned migration `UPDATE ai_providers SET enable_thinking = false`，兜底语义反转风险（旧 `true` 含义是「剥标签」，反转后清零最安全，避免误开启思考拖慢打标签）。
- **日报 context 注入**：`TagInput` 新增 `ArticleContext` 字段，在 `collectBoardTags` 两处文章 ID 查询点顺带取出代表性文章标题+摘要（每 tag 前 2-3 篇、每篇截断 ~200 字），填进 highlights/threads/cluster 三个 prompt，修复头条「看不见事件详情导致混淆」。
- **差异化靠配置实现**：不新增代码层路由开关；运维侧在后台配两条 provider 指向同一本地服务（`qwythos-think` / `qwythos-nothink`），分别挂到 `digest_polish` 与 `topic_tagging` route。

## Capabilities

### New Capabilities
- `ai-router-thinking-control`: provider 级控制模型思考开关（透传 `chat_template_kwargs.enable_thinking`）及其语义反转的数据迁移。详见 `specs/ai-router-thinking-control/spec.md`。

### Modified Capabilities
<!-- 日报 context 注入是对 daily-report 行为的增强，但项目无既有 openspec/specs/daily-report 规范文件，故作为新增行为随本 change 描述，不单列 delta spec。 -->

## Impact

- **后端代码**：
  - `internal/platform/airouter/openai_compatible.go`（透传 enable_thinking、移除剥标签调用）
  - `internal/platform/database/postgres_migrations.go`（新增清零迁移）
  - `internal/models/ai_models.go`（`EnableThinking` 字段注释说明新语义，列名/类型不变）
  - `internal/topicgraph/repository/daily_report_models.go`（`TagInput` 加 `ArticleContext`）
  - `internal/topicgraph/service/daily_report_orchestrator.go`（`collectBoardTags` 两处查询点补文章标题/摘要）
  - `internal/topicgraph/service/daily_report_llm.go`、`daily_report_cluster.go`（三处 prompt 注入）
- **API / 数据兼容性**：`enable_thinking` JSON 字段名不变（仅语义变），前端无破坏；`ArticleContext` 为内存结构体非持久化，无 DDL；migration 幂等可重复执行。
- **运维**：需在后台手动新建并挂载两条 provider 记录（代码外操作，交付说明列出步骤）。
- **前端**：无代码改动（后台 provider 表单字段名不变；UI 文案调整不在本次 scope）。
