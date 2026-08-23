## Why

数据增强产出现在缺一种"有深度的输出"——例如《现代世界体系》或 B 站「内部看美国」那种**结构化因果剖析**（单点事件 → 底层机制 → 大系统 → 历史对照 → 边界限定 → 可核查证据链）。根因有二：(1) 整条三角色流水线（interpret / agent / analyze / lens）被硬编码为"A 股产业分析师"，数据源只有 ETF 行情，对非金融的结构性命题天然水土不服；(2) 唯一能解锁外部证据的 `web_search` 是 `NoopWebSearcher`（`wire.go:60` 硬编码，从未接真实后端），导致证据源严重不足。配套问题：因果推理只有"单跳 A→B"，没有多层系统级因果梯子；spec 里还残留旧的"走向预测"主线（direction/confidence/horizon），与本该接管的"形态+视角+分层见解"主线并存、不一致。

## What Changes

- **BREAKING**：移除 A 股金融数据方向——删除 `list_etf_by_keyword` / `get_etf_quote` / `list_sectors` 工具，删除 `etf_quote` / `exchange_rate` / `gdelt_event` 三个 `source_type` 枚举（`exchange_rate` / `gdelt_event` 本就未实现——`board_config.go:39-40` 明确 `no tools implemented yet`，零浪费）
- 接入**真实 web_search 后端（博查 Bocha）**，替换 `NoopWebSearcher`；`WebSearcher` 接口保持不变，博查为首个实现，未来可换 Tavily 兜底
- 新增 **`fetch_page` 工具**，复用 `reader` 域已有 `readability_crawler` 抓正文，把证据链从 snippet 升级到"可核查原文"
- 三角色 prompt 去"A 股 / 产业分析师"硬编码 → **领域自适应的结构化分析**
- analyze 产物新增**「深度层」`depth` 块**：`system_reframe`（系统重定位）/ `mechanism_layers`（多层机制拆解）/ `historical_analogy`（历史类比）/ `regime_shift`（范式转折，可选）/ `boundary`（反过度解读边界）/ `evidence_chain`（可核查证据链）；所有非 `sparse` 形态强制产出
- 形态分类新增 **`structural`**（纯结构话题，如"人民币国际化进程"，无离散事件）
- 视角（lens）生成器示例从市场事件题改为**结构 / 系统题**（"X 为何反复发生""X 背后底层结构"）
- 清除 spec 中残留的旧"走向预测"主线（`direction` / `confidence` / `horizon` / `trigger_up` / `trigger_down`），与"形态+视角+分层见解+深度层"主线对齐
- 前端 `CausalAnalysisReport.vue` 渲染深度层
- 配置：博查 key **界面可配**（`ai_settings` 表 `bocha_config` + 设置页表单，照 Firecrawl），DB 优先、`configs/config.yaml` + 环境变量 `BOCHA_API_KEY` 兜底；动态读（界面改即时生效）；无 key 时优雅降级回 Noop

## Capabilities

### New Capabilities

> 无独立新 capability。web 源（博查 web_search + fetch_page）与深度层均并入既有 `data-enrichment` capability，作为其新增 Requirement，与现有「探索 agent 工具集」「分层见解产出」等 Requirement 同域。

### Modified Capabilities

- `data-enrichment`：核心改动域。修改的 Requirement——①「数据源工具注册表」（删 ETF 工具强制要求，改要求 web_search 真实现 + fetch_page）②「板块数据源绑定」（删金融 source_type 枚举）③「分层上下文驱动的数据增强编排」（去产业硬编码，分析员产出从走向预测改为深度层）④「话题形态判断」（新增 structural 形态）⑤「分析视角候选与选择」（视角示例结构化）⑥「分层见解产出」（加深度层）⑦「多形态分析报告」（渲染深度层）。新增 Requirement——「web 搜索与正文抓取数据源」「分析深度层产出」。

## Impact

- **后端**：`orchestrator.go`（4 个 Analysis 结构体加 Depth + interpret/agent/analyze/lens prompt 重写）、`web_search.go`（新增 `BochaWebSearcher`）、`tool_registry.go`（删 ETF 工具 + 注册 fetch_page）、新增 `fetch_page.go`、`wire.go`（接博查 + 注入 fetch_page 依赖）、`board_config.go`（`ToolsForSourceType` 重做）、`repository/models.go`（SourceType 枚举精简）、`lens_source.go`（视角示例重写）。新增博查 HTTP client + 配置读取（照 Firecrawl `ai_settings` 模式，DB 优先 + env/config.yaml 兜底 + 动态读）
- **前端**：`app/api/boardEnrichment.ts`（加 depth 类型）、`app/features/tags/components/CausalAnalysisReport.vue`（渲染深度层，922 行）、`BoardEnrichmentPanel.vue`（微调）
- **数据库 / 数据迁移**：`board_data_sources` 表 schema 不变（机制保留为扩展点），但既有 `source_type ∈ {etf_quote, exchange_rate, gdelt_event}` 的行变为无效孤儿——提供清理说明；旧 `topic_enrichment_result` 无 depth 字段，读取时 depth 视为可选降级渲染（不崩）
- **配置**：设置页增博查配置表单（`ai_settings` 表 `bocha_config`，照 Firecrawl）；`configs/config.yaml` 博查段 + 环境变量 `BOCHA_API_KEY` 保留为兜底
- **依赖**：不引入新三方依赖（博查走标准 HTTP，fetch_page 复用 reader 既有 readability）
- **部署影响**：合并后用户可见行为变化——增强分析从"A 股产业点评"变为"结构化深度剖析"；需用户在**设置界面**配博查 key（无 key 则 web_search 降级、深度层仍产但证据链弱）；旧 ETF source 配置失效需清理
