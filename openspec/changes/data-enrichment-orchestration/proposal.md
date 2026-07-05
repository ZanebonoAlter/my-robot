## Why

Syntopica 的分析管线（标签→向量→板块→日报→持久话题演进）**只认带标题+正文+时间的 article**。新闻外的实时数据（行情、指标）无法纳入分析——article schema 撑不住数值，"标签聚类成叙事"也产不出有意义的板块判断。

用户希望分析时能按需补充实时数据（A 股 ETF 行情）佐证板块影响判断。早期排除两个错误方向：

1. **数据源当订阅流灌进 article 表**：数值撑不住 schema，且制造"第二个话题泳道"与持久话题演进重复。
2. **单篇新闻做孤立影响分析**：持久话题核心价值是"事件演进"，孤立单点无意义。

PoC（`tests/data_enrichment_poc/`）验证正确方向：**数据源是 agent 可调用的工具**。本 change 在 PoC 基础上进一步升级为**持久话题的认知闭环系统**——不只是"补数据判断"，而是维护两类隔离的记忆：

- **新闻记忆**（循环 A，纯新闻汇总，客观，只随新闻变）
- **分析认知**（循环 B，增强 + review，主观，自我迭代）

两者通过表 1 单向连接，互不污染。持久话题分析被定位为**不断自我修正的认知过程**，而非一次性快照。

PoC 关键结论：
- ✅ 三角色架构（解读员→查询员 agent loop→分析员）在 Qwen3-9B 本地跑通，10 主题 0 出错。
- ✅ agent loop 链式查询正常，会换词（光刻机→半导体，避险→黄金）。
- ⚠️ 必须消费持久话题演进脉络而非单篇新闻。

## What Changes

### A. 板块↔数据源配置层（data-enrichment）
- 新表 `board_data_sources`（board↔source + 板块级 config）。
- 数据源工具注册表（Go，对标 PoC TOOL_REGISTRY）：list_etf_by_keyword / get_etf_quote / list_sectors。
- 板块配置扩展（tab 式）：数据源绑定 + `enrichment_enabled` + `window_days`(默认14) + `context_layers`。

### B. 分层新闻汇总上下文（循环 A，新增）
独立循环，纯新闻驱动，不碰分析：
- 新表 `topic_lifeline_context`（week/month/year/all + `as_of_date`）。
- 定时触发（周/月/年）+ 检查自愈（补漏）+ 手动重生成。
- week 直接重算（例外）；month/year/all 增量 + 旧汇总合并。

### C. 演进版数据增强编排（循环 B 核心）
消费**分层上下文**（表1 + 14天详情 + 历史 applied review），不是单 lifeline：
1. 解读员：全层读（按 context_layers），提炼需补数据的产业方向。
2. 查询员（agent loop）：链式查询，带去重/不截断/thinking关闭三防御。
3. 分析员：分层上下文 + 实时数据 → evolution_assessment/sectors/causal_chain。

### D. 分析认知循环：review judge（新增）
- 新表 `topic_enrichment_review`（prev↔curr result 对比 + deviation_summary）。
- 增强后 LLM 半自动对比判断（JSON + 理由，非字段 diff），值得才生成。
- **不回写表 1**——分析认知独立迭代，新闻记忆保持客观；applied 仅标记"认知采纳"，下次增强会读。

### E. 仅手动触发（不挂日报管线）
- 循环 B 仅手动触发（CRUD 界面"重新分析"）——不是所有板块对金融有影响（如"开发工具"板块），自动挂管线无意义且浪费成本。
- 设计余量：将来可加回日报管线自动触发作为可选开关。

### F. 前端：板块 tab 下 CRUD 界面（新增）
板块详情页新增「数据增强」栏，三表 CRUD（查看/编辑/触发）。**第一版 CRUD 先行验证功能**，侦探墙重构推后单列 change。数据契约设计为侦探墙可复用。

## Capabilities

### Added Capabilities

- `data-enrichment`：板块↔数据源配置、分层新闻汇总（循环A）、演进版三角色增强（循环B）、review judge 认知循环、**手动触发**、板块tab CRUD

## Impact

- **后端**
  - 新表：`board_data_sources`、`topic_lifeline_context`、`topic_enrichment_result`、`topic_enrichment_review`。
  - 新 domain：`internal/dataenrichment/`（tool registry + 三角色编排 + 循环A汇总 + review judge + handler）。
  - 复用：`GetTopicLifeline`（14天详情渲染）、`airouter.Router.Chat`（必带 Operation/SessionID）。
  - 定时任务：循环A的周/月/年刷新 + 检查自愈。
- **前端**：板块 tab 新增「数据增强」CRUD 栏。侦探墙重构推后单列 change。
- **AI 成本**：
  - 循环B单 topic：解读1 + 查询员每主题1-3 + 分析1 + review_judge1 ≈ 5-8 次 LLM 调用，全层读 ~2.5-3k token（实测校准，见 design §8）。
  - 循环A：每个 granularity 刷新 1 次 LLM 调用（summarize_context）。
- **依赖关系**：可观测基础（`ai_call_logs` 表 + `airouter/store.go` + `SessionIDFromContext`@`daily_report_watch.go:29`）**代码层已落地**，不阻塞开工。原 proposal 声明强依赖 ai-call-logging-schema 前置就绪，该声明已过时——功能已落地。
- **数据兼容**：4 张新表独立，历史数据无影响。
- **部署后影响**：本 change 上线后，用户在板块「数据增强」tab 可配置数据源、查看新闻汇总/增强结果/review；需用户手动开 `enrichment_enabled` 才会跑增强（默认关，避免成本）。历史 topic 无 context，循环A定时任务首次跑会批量补生成。
