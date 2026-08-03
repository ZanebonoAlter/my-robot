## Context

- 数据增强（`internal/dataenrichment/`）现有三角色编排：interpret（判形态+提炼研究方向）→ agent loop（查数据）→ analyze（出分层见解）+ review_judge（新旧对比），外加 lens（视角候选）。
- 当前 prompt 全部硬编码"A 股 / 产业分析师"框定：`interpretPrompt`（`orchestrator.go:319`）要求提炼"A 股有对应 ETF 的方向"；`agentLoopSystemPrompt`（`:404`）自称"A 股数据查询员"查 ETF 行情；`analyzePrompt`（`:752`）因果只有单跳"A→B logic"。
- 数据源注册表（`tool_registry.go` register）只有 `list_etf_by_keyword` / `get_etf_quote` / `list_sectors` + 内部导航 `list_boards`/`list_lanes`/`get_lane_detail` + `web_search`。`web_search` 是 `NoopWebSearcher`（`web_search.go`），`wire.go:60` 硬编码，全仓库无任何真实搜索后端实现（Tavily/Serper/博查 均无）。
- `exchange_rate` / `gdelt_event` 两个 `source_type`（`models.go:16-17`）从未实现（`board_config.go:39-40` 明示 `no tools implemented yet`），纯空枚举桩。
- `WebSearchResult` 只返回 `{title,url,snippet}`，无正文抓取工具；但 `reader` 域**已有** `readability_crawler`（`internal/reader/service/readability_crawler.go`），可复用。
- spec（`openspec/specs/data-enrichment/spec.md`）有两代主线残留：旧的"走向预测 direction/confidence/horizon/trigger_up/trigger_down"（orchestration 代）与本该接管的"形态+视角+分层见解"（causal-analysis-agent 代）并存。
- 目标风格已具象化并存档于 `docs/reference/research/内部看美国-分析基因.md`（B 站「内部看美国」UID 3546715770588065 的 7 条分析基因，用户已认可）。优先复刻 ②系统重定位 / ④多层机制拆解 / ⑤边界限定 / ⑦可核查证据链。

## Goals / Non-Goals

**Goals:**

- 让数据增强产出具备结构化深度（系统重定位 / 多层机制 / 历史类比 / 边界 / 可核查证据链），而非 A 股产业点评
- 接通真实外部数据源（博查 web_search + fetch_page 正文），消灭"web_search 没法用"
- 三角色去"A 股 / 产业"硬编码，领域自适应
- spec 与实现对齐，清除旧"走向预测"主线残留

**Non-Goals:**

- 不动循环 A（新闻汇总 `topic_lifeline_context`）——它只基于 sections 客观汇总，保持不变
- 不动 review_judge 的语义（新旧认知对比 + applied 不回写新闻记忆）——只让它适配新 schema
- 不动 agent loop 三防御（thinking 关闭 / 历史不截断 / 去重）——保留
- 不做多搜索服务商（博查为唯一实现；`WebSearcher` 接口预留未来换 Tavily 兜底，但本 change 不实现）
- 不做前端侦探墙重构——仅给 `CausalAnalysisReport` 加深度层渲染
- 不接结构化外部数据库（Windward/Kpler/海关 等是 web_search 检索对象，不是本 change 注册的内置工具）

## Decisions

1. **两层产物结构（事实层 + 深度层）**：保留现有 per-form 事实层结构（`fact_layer`/`timeline`/`veins`/`impact`），新增 form-agnostic 的 `depth` 块。理由：「内部看美国」本身就是"事实层+深度层"双层结构；additive 形状让前端可增量渲染、旧结果降级不崩。`depth` 对所有非 `sparse` 形态强制产出；`sparse` 仍只 `notice`+`summary`（诚实标注不动）。

2. **深度层字段 = 7 分析基因的直接映射**：`system_reframe`(②) / `mechanism_layers`(④) / `historical_analogy`(③) / `regime_shift`(⑥, 可选) / `boundary`(⑤) / `evidence_chain`(⑦)。`evidence_chain` 从 `source_type ∈ news|tool` 升级为 `source_type ∈ news|web|page`，`web`/`page` 带 `url`+`quote`+`institution`+`date`，落地可核查原文。

3. **博查作为 web_search 唯一实现**：连通性实测（2026-08-04）博查 `api.bochaai.com` 221ms 国内最快且中文检索优；`WebSearcher` 接口（`Search(ctx,query)→[]WebSearchResult`）不变，新增 `BochaWebSearcher` 实现。key 照 airouter provider 模式（`configs/config.yaml` + 环境变量 `BOCHA_API_KEY`），`wire.go` 中 `key==""` 时回退 `NoopWebSearcher` 优雅降级。

4. **fetch_page 复用 reader readability**：新增 `fetch_page` 工具调 `internal/reader/service/readability_crawler`，返回 `{title,url,main_text(截断 N 字符)}`，喂深度层 `evidence_chain`。不新造爬虫。超时/反爬失败时返回错误 JSON 让 agent 自降级（沿用 registry 约定）。

5. **金融方向一刀切删除**：删 `list_etf_by_keyword`/`get_etf_quote`/`list_sectors` 三个工具及其注册；删 `SourceTypeETFQuote`/`SourceTypeExchangeRate`/`SourceTypeGDELTEvent` 三个枚举（后两者从未实现）；`board_config.go` 的 `ToolsForSourceType` 改为：默认 always-on 集 = 内部导航 + web_search + fetch_page，不再有 per-source_type 条件工具。`board_data_sources` 表与 `source_type` CHECK 机制保留为扩展点（未来接结构化源），但内置无金融 source_type。

6. **新增 `structural` 形态**：纯结构话题（如"人民币国际化进程""美元霸权演变"）无离散事件，归 `structural`，产出 = 结构演化叙述 + 深度层。形态枚举变为 5 个：`event_chain`/`theme_vein`/`single_point`/`structural`/`sparse`。

7. **三角色 prompt 去硬编码 + 深度指令注入**：interpret「资深产业分析师」→「结构化分析编辑」，提炼领域无关研究方向（历史机制/关键数据/可比案例）；agent「A 股查询员」→「研究助理/事实核查员」，工具集变 web_search+fetch_page+导航；analyze 强制产出 `depth` 块并**显式要求反过度解读**（`boundary` 非空）；lens 示例从市场事件题改结构/系统题。

8. **清除旧"走向预测"主线**：`分层上下文驱动的数据增强编排` Requirement 中分析员产出从 `direction`/`confidence`/`horizon`/`trigger_up`/`trigger_down` 改为深度层；分析认知对比 review 不再对比"预测方向兑现"，改为对比深度层见解的新增/推翻/确定性变化。

9. **读取时 depth 降级**：旧 `topic_enrichment_result`（JSON 无 depth）读取时 `depth` 视为可选，前端/后端解析缺失则不渲染深度层、不报错。

## Risks / Trade-offs

- **深度层增加 LLM 输出 token 成本** → 仅非 sparse 形态产出；`depth` 各字段给字数上限提示；`evidence_chain` 限条数。可观测性（`ai_call_logs`）保留，apply 后看实际 token 增量。
- **博查单点依赖** → `WebSearcher` 接口隔离，换/加服务商零成本；本 change 不实现兜底（YAGNI，等博查验证后再说）。
- **fetch_page 延迟与反爬** → 复用 reader 已有超时/降级（`fallback_crawler`）；单工具失败不阻断 agent loop（沿用 registry 错误 JSON 约定）。
- **prompt 重写风险（产出质量不可自动判定）** → 用「内部看美国」真实话题做 golden case 人工验收；analyze 输出 schema 加结构化校验（`depth` 必填字段），解析失败重试一次。
- **既有 ETF source 配置失效** → 用户已明确"老的方向不对"，接受 orphan；tasks 提供清理脚本/SQL 说明；`board_data_sources` 机制本身保留。
- **前端震动面（`CausalAnalysisReport.vue` 922 行）** → 深度层是 additive 渲染块，不改既有事实层渲染；typecheck/build 走 Windows cmd（项目铁律）。
- **`review_judge` 适配新 schema** → 它比的是自由 JSON 串，schema 变化不破坏其机制，但 apply 时需用新格式跑一次验证认知对比仍成立。
