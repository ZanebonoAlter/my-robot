## Context

换用 Qwythos-9B-Claude-Mythos-5-1M-MTP-Q6_K（Qwen3.5-9B 底座、1M 上下文、原生 tool calling）跑在本地 llama-server，期望「打标签不思考、生成日报思考」并修复日报头条混淆。本设计基于对运行中服务器（`http://127.0.0.1:8080`）的实测，结论已验证。

**启动命令（用户当前）**：`-c 524288 -ngl 999 --cache-type-k q8_0 --cache-type-v q8_0 --flash-attn on --jinja -np 2`，**未设全局 `--chat-template-kwargs`**，即模板走默认分支。

**当前代码状态（两个根因）：**

1. **思考开关语义错位**：`backend-go/internal/platform/airouter/openai_compatible.go:196-199` 中 `EnableThinking` 仅触发事后 `stripThinkTags`（剥除 `<think>...</think>`），**从不透传** `chat_template_kwargs`。全仓 grep 确认无任何 `chat_template_kwargs` 透传。因此模型是否思考完全由 CLI 全局参数决定，后端代码层无法按 capability 控制。
2. **日报头条混淆**：`buildHighlightsPrompt`（`daily_report_llm.go:87`）只喂 `- [ID:5] 标签名 (文章数:12)`，LLM 看不到任何事件内容；而 `collectBoardTags`（`daily_report_orchestrator.go:312-318`）其实已经 pluck 出每 tag 的文章 ID 列表，`models.Article` 也具备 `Title/AIContentSummary/FirecrawlContent/Content/Description` 全部字段——数据源头可获取，只是没喂给 prompt。

## Goals / Non-Goals

**Goals:**

- 修正 `AIProvider.EnableThinking` 为「真正开启模型思考（透传 `chat_template_kwargs.enable_thinking`）」，使后端能按 provider 控制 Qwythos 的思考行为。
- 通过「配两条 provider 指向同一服务」实现打标签（不思考）/ 日报（思考）差异化，无需新增代码层路由开关。
- 数据迁移统一清零，兜底语义反转。
- 日报 highlights/threads/cluster 的 prompt 注入代表性文章标题+摘要，根除头条混淆。

**Non-Goals:**

- 不做日报 tool calling / agent 循环（用户已选 context 注入方案，更稳、改动小）。
- 不下沉思考开关到 `ai_route_providers`（link 级）；维持 provider 级，靠配置实现差异化。
- 不改前端代码（`enable_thinking` 字段名不变；UI 文案调整不在本次 scope）。
- 不解析 `reasoning_content` 字段（服务器已把思考分离到 `content` 之外，保留现状）。

## Decisions

### D1. 用 per-request `chat_template_kwargs.enable_thinking` 控制思考（实测验证）

**依据（实测证据，2026-06-26 于运行中服务器）：**

- **证据1 服务器元信息**：`GET /health` → `{"status":"ok"}`；`GET /v1/models` → `Qwythos-9B-Claude-Mythos-5-1M-MTP-Q6_K.gguf`。
- **证据2 模板支持 per-request 开关**：`GET /props` 返回 `chat_template_caps = {supports_tools, supports_tool_calls, supports_parallel_tool_calls, supports_preserve_reasoning, ...}` 全为 true；模板第 153-160 行含 `enable_thinking` 分支：传 false 时预置空 `<think>\n\n</think>\n\n`（预关闭思考），否则开 `<think>\n`。
- **证据3 对照实验（决定性）**：同一 prompt「用一句话解释什么是向量数据库」，仅改请求体 `chat_template_kwargs.enable_thinking`：
  | 参数 | `reasoning_content` | `completion_tokens` | 结论 |
  |---|---|---|---|
  | `false` | 无（None） | 55 | 直答，适用于打标签 |
  | `true` | 有（2014 字） | 600（length 截断） | 思考，适用于日报 |
  → 证明 per-request 开关在 Qwythos 上稳定双向可控，且服务器把思考分离到独立 `reasoning_content` 字段，`content` 是干净答案。

**为何选 per-request 而非 CLI 全局开关**：CLI `--chat-template-kwargs` 是全局一刀切（且在近期 llama.cpp 中被标记 deprecated，见 upstream Discussion #23351），无法满足「同服务、两 capability 差异化」。per-request 透传是唯一可行路径。

**为何不选 `--reasoning-budget 0`**：该参数是全局的，且与 `--jinja` 模板的 `enable_thinking` 是两套机制；per-request 的 `chat_template_kwargs` 更贴合「按 provider 配置」的本设计。

> **勘误（2026-06-27，部署后实测发现）**：本节最初要求「`EnableThinking=false` 时请求体 MUST NOT 包含 `chat_template_kwargs`，由服务端模板默认分支决定，对 Qwythos 即不强制开思考」——这个「即不强制开思考」的前提是错的。Qwen3/Qwythos 模板在该 kwarg **缺失**时走默认分支即**开启思考**（证据2 实测：传 false 才预置空 `<think></think>` 关闭，「否则开 `<think>`」包含「不传」）。因此「不发参数」= 开思考，导致打标签 provider（migration 清零成 false）实际仍在思考。修正：请求体**始终显式发送** `chat_template_kwargs.enable_thinking`（true/false 都发），per-request 钉死，不赌模板默认。对应 spec.md 的 Requirement 已同步修正。

### D2. 差异化靠「两条 provider 指向同一服务」，不下沉开关到 link 级

`ai_route_providers` 是多对多关联，同一个 provider 记录可被多 capability 的 route 共用。若打标签与日报共用一条 provider 记录，provider 级 `EnableThinking` 会让两者绑死。考虑过的方案对比：

| 方案 | 改动量 | 灵活性 | 决定 |
|---|---|---|---|
| 下沉到 `ai_route_providers`（link 级字段） | 中（加列+AutoMigrate+handler+airouter 读取） | 最干净，共用 provider 不冲突 | 未选（过度设计） |
| 按 capability 在代码内自动决定（digest_polish 默认思考） | 小 | 不够灵活，切换需改代码 | 未选 |
| **配两条 provider 指向同服务**（用户选定） | 极小（零结构改动，纯配置） | 满足当前需求 | **选定** |

选定理由：当前只有一台本地 Qwythos，差异化需求明确且固定（打标签关、日报开）。配两条 provider 记录（均指向 `http://127.0.0.1:8080`，仅 `enable_thinking` 不同）即可，不引入 DB 结构变更。若未来 provider 数量增长或多租户场景，再评估下沉 link 级。

### D3. 语义反转用迁移统一清零兜底

`EnableThinking` 旧含义「剥标签」与新含义「开启思考」方向相反。若 DB 中存在旧 `true` 值（本意为剥标签），改完会意外变成「开启思考」。处理方案对比：

| 方案 | 风险 | 决定 |
|---|---|---|
| 保留旧值（不处理） | 旧 true 误开启思考，拖慢打标签 | 未选 |
| 布尔取反迁移 | 实际很少有人专门设过此字段，收益小、反转有风险 | 未选 |
| **统一清零迁移**（用户选定） | 发布后需手动在后台为日报 provider 开思考 | **选定** |

新增幂等 versioned migration `UPDATE ai_providers SET enable_thinking = false`（可重复执行）。发布后运维在后台按需开启日报 provider 的思考。

### D4. 日报 context 注入：给 TagInput 加 ArticleContext，复用现有文章 ID 查询点

**依据（证据5）**：`collectBoardTags` 第 312-318 行（主路径）与第 359-364 行（fallback）已对每个 tag 跑一次查询 pluck 出当日文章 ID 列表，存入 `articleIDSets`。`Article` 模型字段齐全。因此注入文章标题/摘要只需把这两处 `Pluck(article_id)` 改成 `Select article_id, title, <摘要>`，无需新增 join 路径。

**字段设计**：`TagInput` 新增 `ArticleContext string`（内存结构体，非持久化）。取每 tag 前 2-3 篇代表性文章，摘要优先级沿用 `article_tagger.go` 的 `buildArticleSummary`（`AIContentSummary > FirecrawlContent > Content > Description`），每篇按 rune 截断 ~200 字，拼成 `《标题1》摘要... ; 《标题2》摘要...`。抽辅助函数 `buildArticleContextForTag(tagID, startOfDay, endOfDay) string` 避免两处重复。

**为何 context 注入而非 tool calling**：tool calling 需在 airouter 加 tools 字段、解析 tool_calls、实现 agent 调用-执行-回填循环，改动大且 9B 模型做 agent 可靠性存疑；context 注入改动小、稳定可控，1M 上下文绰绰有余。用户已选定 context 注入。

**prompt 注入点**：`buildHighlightsPrompt`（`daily_report_llm.go:87`，头条混淆核心修复）、`buildThreadsPrompt`（`:200`）、`buildClusterPrompt`（`daily_report_cluster.go:146`），均加 `if t.ArticleContext != "" {...}` 守卫，避免空字段污染 prompt。无需改 `GenerateHighlights`/`GenerateClusterThreads`/`ClusterTags` 的签名（字段经 `[]TagInput` 自动透传）。

## Risks / Trade-offs

- **[风险] `enable_thinking=false` 在某些 llama.cpp 版本/模型上有已知不生效 bug**（upstream #20409 / #20182）。
  → 缓解：已在本服务器对 Qwythos 实测，false 稳定生效（证据3）。部署后运维侧观察打标签请求是否产生 reasoning（应无、token 下降）。
- **[风险] context 注入增加 prompt token 体积**，可能推高日报生成耗时与显存。
  → 缓解：每 tag 限 2-3 篇、每篇截断 ~200 rune；1M 上下文 + 已有 `-c 524288` 余量充足。首版取保守值，必要时下调。
- **[权衡] 配两条 provider 指向同服务**增加了配置维护成本（两条记录指向同一 8080）。
  → 接受：当前单机单模型场景下成本可控，且零代码结构改动。交付说明列出明确配置步骤。
- **[风险] 移除事后 `stripThinkTags` 调用后，若某 provider 仍返回混入 `<think>` 的 content**（如非标准服务器），答案会带思考标签。
  → 缓解：Qwythos 在 llama-server 上已确认 `content` 干净（证据3）；保留 `stripThinkTags` 函数本体与单元测试作为防御性工具，仅 Chat 不再调用。若未来接入其他服务器出现残留，可再挂回。

## Migration Plan

1. **代码与迁移同批发布**：airouter 透传逻辑 + `postgres_migrations.go` 新增 `20260626_0001` 清零迁移，随服务启动时 `RunMigrations` 执行。
2. **回滚**：迁移为纯 `UPDATE`（幂等），回滚即重新部署旧代码；`enable_thinking` 列本身不变。若需恢复旧语义，撤销 openai_compatible.go 改动即可（但旧语义本就是 bug 性质，不建议回滚）。
3. **运维配置（代码外，发布后执行）**：
   - 后台新建 provider `qwythos-think`：`base_url=http://127.0.0.1:8080`、`model=<Qwythos 文件名>`、`enable_thinking=true`。
   - 后台新建 provider `qwythos-nothink`：同 base_url/model、`enable_thinking=false`。
   - `digest_polish` route 挂 `qwythos-think`；`topic_tagging` route 挂 `qwythos-nothink`。
4. **验证（端到端）**：触发一次日报生成，确认 highlights 不再混淆（能看到事件细节）；观察打标签请求无 reasoning 产出、token 下降。

## Open Questions

- 无。三处设计抉择（D2 配置差异化、D3 清零迁移、D4 context 注入）均已与用户确认。
