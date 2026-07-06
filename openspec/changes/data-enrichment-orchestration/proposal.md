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
- 新表 `topic_lifeline_context`（granularity + **period** + `as_of_date`），**period 档案式存储**（每周期独立一条，历史可翻，不再滚动覆盖）。
- 定时触发（周/月/年各产独立 period 行）+ 检查自愈（补缺失历史 period）+ 归档清理（week>8周 / month>12月）+ 手动重生成任意 period。
- 每个 period 独立汇总一条（all 例外，滚动单行）。

### C. 演进版数据增强编排（循环 B 核心）
消费**分层上下文**（表1 + 14天详情 + 历史 applied review），不是单 lifeline：
1. 解读员：全层读（按 context_layers），提炼需补数据的产业方向。
2. 查询员（agent loop）：链式查询，带去重/不截断/thinking关闭三防御。
3. 分析员：分层上下文 + 实时数据 → evolution_assessment + **sectors（含走向 direction/置信度/horizon/凭什么 reasoning/原始依据 evidence/板块触发条件）** + causal_chain。

### D. 分析认知循环：review judge（预测兑现度复盘，新增）
- 新表 `topic_enrichment_review`（prev↔curr result + **verdict 兑现度结算** + deviation_summary）。
- 增强后 LLM **对照上次预测方向 vs 期间实际走势**，逐板块结算 hit/part/miss（不再是"描述风格对比"）。
- **不回写表 1**——分析认知独立迭代，新闻记忆保持客观；applied 仅标记"认知采纳"，下次增强会读。

### E. 仅手动触发（不挂日报管线）
- 循环 B 仅手动触发（CRUD 界面"重新分析"）——不是所有板块对金融有影响（如"开发工具"板块），自动挂管线无意义且浪费成本。
- 设计余量：将来可加回日报管线自动触发作为可选开关。

### F. 前端：板块 tab 下「认知工作台」（新增）
板块详情页新增「数据增强」tab，按**用户认知任务流**组织（不再四张表平铺 CRUD）：①最近怎么了（周期筛选翻历史）→②会往哪走（板块走向预测，可展开看凭什么+触发条件）→③猜得准吗（预测兑现复盘）→④个股深度辩论（接 FinGenius，手动触发，6 角色辩论+投票）→⑤数据源/参数。**证据链 tooltip 悬停显示新闻原话，不跳转**；术语全翻译成人话。原型 `prototype/enrichment-workbench.html` 已验证交互方向（v3，含④个股辩论四态）。侦探墙重构推后单列 change，契约设计为可复用。

### F'. 个股深度辩论（外部 FinGenius，GPL v3 合规边界，2026-07-06 新增）

**两段式分工**：Syntopica 给板块方向 + 代表标的池（不带买卖建议，守合规边界），个股博弈交给外部 FinGenius（6 角色 agent：舆情/游资/风控/技术/筹码/大单，多轮辩论+投票）。

**GPL v3 合规（核心原则：进程隔离，绝不碰源码）**：
- Syntopica 把 FinGenius 当**独立黑盒 HTTP 服务**调用（`fingenius_client.go`），**不复制/翻译/链接**任何 FinGenius 源码进本仓库。两边各自独立进程、独立仓库、独立 LICENSE。
- HTTP 调用独立服务是松耦合，**不构成 GPL 衍生作品**（不传染）。Syntopica 维持 GPL v3（本就是）。
- 合规义务：README/docs 致谢 FinGenius + 提供上游仓库链接（见 design §11 决策⑥）。

**职责切分（两个独立开源项目）**：
- **本 change**（Syntopica）：实现 `fingenius_client.go`（HTTP 异步客户端）+ `stock_debate_result` 表 + **LLM 提炼环节**（把 FinGenius 文本输出转三档 stance）+ 独立触发端点 + 前端④区块。
- **另起项目**（FinGenius 服务）：FastAPI 服务壳改造，暴露 `POST /analyze`（提交）+ `GET /task/{id}`（轮询）。**不在本 change 范围**。

**异步任务 + Syntopica 侧提炼（两个关键技术决策，详见 design §11 决策⑥）**：
- **异步任务**：FinGenius 单次分析数分钟（6 agent × 多步 LLM + 辩论），同步 HTTP 必超时。用「POST 提交拿 task_id → GET 轮询拿结果」模式，`FINGENIUS_POLL_INTERVAL`/`FINGENIUS_MAX_WAIT` 可配。原型④加载态正好展示轮询过程。
- **Syntopica 侧 LLM 提炼**：FinGenius 原始输出是「6 段文本 + 二档投票(bullish/bearish)」，不直接提供三档 stance。FinGenius 服务端零加工（守 GPL 边界），Syntopica 用一次轻量 LLM 调用（`debate_distill` Operation）把文本提炼成原型④要的三档（up/down/flat + 一句话 note）。提炼失败降级显示 FinGenius 原始文本。

**触发与降级**：手动触发（前端④「开始辩论」按钮，不串进循环B主流程）；FinGenius 不可用或轮询超时 → 辩论区块显出错态，**不阻塞**板块方向预测（个股辩论是可选增强）。

详细设计见 `design.md` §3.4（分析员 symbols 扩展）、§4.2b（stock_debate_result 表）、§6（debate_distill Operation）、§11 决策⑥（GPL 合规边界 + 异步客户端契约 + LLM 提炼规则 + 配置 + 降级 + FinGenius 服务端改造提示）。

## Capabilities

### Added Capabilities

- `data-enrichment`：板块↔数据源配置、分层新闻汇总（循环A，period 档案式）、演进版三角色增强（循环B，含走向预测+证据链）、review judge 预测兑现度复盘、**手动触发**、板块tab 认知工作台
- `data-enrichment`（2026-07-06 扩展）：**个股深度辩论**——分析员给板块代表标的池，对标的发起 FinGenius 6 角色辩论（外部 HTTP 服务，GPL v3 进程隔离，不碰源码），辩论结果独立存表、前端④区块按板块分组展示

## Impact

- **后端**
  - 新表：`board_data_sources`、`topic_lifeline_context`、`topic_enrichment_result`、`topic_enrichment_review`、**`stock_debate_result`**（FinGenius 辩论结果，2026-07-06 新增）。
  - 新 domain：`internal/dataenrichment/`（tool registry + 三角色编排 + 循环A汇总 + review judge + handler + **`fingenius_client.go` HTTP 客户端**）。
  - 复用：`GetTopicLifeline`（14天详情渲染）、`airouter.Router.Chat`（必带 Operation/SessionID）。
  - 定时任务：循环A的周/月/年刷新 + 检查自愈。
- **前端**：板块 tab 新增「数据增强」认知工作台（周期筛选/板块展开/tooltip 证据链/兑现复盘 + **④个股深度辩论四态**）。侦探墙重构推后单列 change。
- **AI 成本**：
  - 循环B单 topic：解读1 + 查询员每主题1-3 + 分析1 + review_judge1 ≈ 5-8 次 LLM 调用，全层读 ~2.5-3k token（实测校准，见 design §8）。
  - 循环A：每个 granularity 刷新 1 次 LLM 调用（summarize_context）。
  - **个股辩论**：成本在 FinGenius 侧（6 agent × 多轮辩论），Syntopica 只付 HTTP 调用开销；手动触发，不自动跑。
- **依赖关系**：
  - 可观测基础（`ai_call_logs` 表 + `airouter/store.go` + `SessionIDFromContext`@`daily_report_watch.go:29`）**代码层已落地**，不阻塞开工。原 proposal 声明强依赖 ai-call-logging-schema 前置就绪，该声明已过时——功能已落地。
  - **FinGenius 外部服务**（2026-07-06 新增）：个股辩论④环节依赖用户另起 FinGenius HTTP 服务（GPL v3 独立项目）。**本 change 只实现 Syntopica 侧客户端，不提供 FinGenius 服务端**。FinGenius 不可用时，④区块显出错态，①②③⑤完全可用——个股辩论是可选增强，非核心闭环。
- **数据兼容**：5 张新表独立，历史数据无影响。
- **GPL v3 合规**：Syntopica 与 FinGenius 均 GPL v3。进程隔离调用不传染，本仓库不引入任何 FinGenius 源码。合规义务（致谢 + 上游链接）记入 README/docs（见 design §11 决策⑥）。
- **部署后影响**：本 change 上线后，用户在板块「数据增强」tab 可配置数据源、按周期翻看新闻汇总、看板块走向预测（带证据链）、复盘预测兑现度；**手动发起个股深度辩论（需另起 FinGenius 服务，否则④显出错态，不影响①②③⑤）**；需用户手动开 `enrichment_enabled` 才会跑增强（默认关，避免成本）。历史 topic 无 context，循环A定时任务首次跑会批量补生成历史 period。
