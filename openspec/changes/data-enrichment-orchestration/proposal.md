## Why

Syntopica 的产品主轴是**持久话题的泳道演进**：一个 persistent topic 每天被日报 section 归属、按天排成一条线（`identity` 轨保证泳道连续，`similarity` 轨给时间线状态）。**泳道本身就是"新闻被串起来"的成品。**

数据增强该做的，是在这条串好的线之上帮用户判断**"现在走到哪了"**——而不是脱离泳道做"未来涨跌预测 + 兑现打分"。

早期 PoC（`tests/data_enrichment_poc/`）验证了正确方向：**数据源是 agent 可调用的工具**。但本 change 初版在 PoC 基础上"升级"时，没有脱离 PoC 的金融骨架，反而把金融做深了（加了个股辩论）。PoC 验证的是"agent 能查数据"，不是"金融是主线"——这个结论被偷换了。具体三大错位：

```
错位①  触发：手动单板块    ←→  应为 话题级跨版块（一个话题命中多条泳道）
错位②  范围：锁死单板块金融  ←→  应为 多类数据源，金融只是其一
错位③  目标：预测涨跌+兑现  ←→  应为 演进定位（强化/转折/扩散/衰减）+ 证据链
```

用户反馈确认了方向调整（2026-07-09）：

- **主线拉回演进定位**：分析员输出从"板块走向"改为"话题演进定位"，金融降为可选数据源视角。对"美伊局势"能查原油黄金佐证，对"Rust 1.x 发布""AI 模型军备"这类无涨跌语义的话题也能做演进定位。
- **触发改话题级跨版块**：保持手动，但从单板块扩到话题级——agent 跨该话题命中的多条泳道聚合信号。
- **分析员 schema 全改演进定位**：`direction/trigger/sectors` → `position`(强化/转折/扩散/衰减) + 跨泳道信号 + 证据链；review 从"涨跌兑现 hit/part/miss"改为"定位变化对比"。
- **FinGenius 个股辩论保留但隐藏**：代码冻结，前端④默认折叠，标"金融可选模块·独立于演进主线"。

### 保留的骨架（不因重定位丢弃）

PoC + 初版验证过、与定位正交的部分原样保留：

- **三表分离**：新闻记忆（表1，客观）/ 当下定位（表2，快照）/ 反思变化（表3，增量）——抽象对，只换填入的内容。
- **循环 A（新闻背景汇总）**：纯新闻按 period 档案式汇总，与演进定位主线正交，**独立产物、独立入口**，不改。
- **tool_registry + HTTPFetcher + agent loop 三防御**：通用 agent 工具调用骨架，金融工具只是首批注册的实现。
- **可观测性**（Operation / SessionID / input_snapshot / tool_calls 落库）：与金融无关，保留。

## What Changes

### A. 板块↔数据源配置层（data-enrichment）
- 新表 `board_data_sources`（版块↔source + 版块级 config）。
- 数据源工具注册表（Go）：list_etf_by_keyword / get_etf_quote / list_sectors（首批金融工具，留扩展位，未来可加非金融 skill）。
- 版块配置扩展：数据源绑定 + `enrichment_enabled` + `window_days`(默认14) + `context_layers`。

### B. 分层新闻汇总上下文（循环 A，独立产物，不改）
独立循环，纯新闻驱动，不碰分析——**这是循环 B 的基础，但前端独立入口**：
- 新表 `topic_lifeline_context`（granularity + **period** + `as_of_date`），period 档案式存储。
- 定时触发（周/月/年各产独立 period 行）+ 检查自愈 + 归档清理 + 手动重生成。
- 前端：独立 tab/区块，周期筛选翻历史 + inline 编辑（已验证，对应旧 `enrichment-workbench.html` ①区块）。

### C. 演进定位编排（循环 B 核心，★重定位）
消费**分层上下文**（表1 + 14天详情 + 历史 applied review），**入口在话题级、跨该话题命中的多条泳道**：
1. 解读员：全层读（按 context_layers），提炼需补数据的方向。
2. 查询员（agent loop）：链式查询，带去重/不截断/thinking关闭三防御。**工具集按 enabled 版块动态注册**——只有 enabled 版块绑定的数据源工具才进 agent loop 可用集。
3. 分析员：分层上下文 + 实时数据 → **演进定位（position: 强化/转折/扩散/衰减）+ 跨泳道信号聚合（signals）+ 证据链（evidence）** + 可选 financial_view。

★ 关键变化：分析员不再输出 `sectors[].direction/confidence/horizon/trigger`（涨跌语义），改为话题通用的演进定位。

### D. 分析认知循环：review judge（定位变化对比，★重定位）
- 新表 `topic_enrichment_review`（prev↔curr result + **position_change 定位迁移** + change_summary）。
- 增强后 LLM **对照上次演进定位 vs 本次**，记录定位迁移（如 reinforcing→turning）+ 凭什么。
- **不再做 hit/part/miss 涨跌兑现**——那是金融专属语义，套不进非金融话题。
- **不回写表 1**——分析认知独立迭代，新闻记忆保持客观；applied 仅标记"认知采纳"，下次增强会读。

### E. 仅手动触发，话题级跨版块（★修正）
- 循环 B 仅手动触发（话题管理界面"演进分析"），不挂日报管线。
- **入口在话题级**：点一个话题，agent 跨它命中的多条泳道聚合信号，不再锁死单板块。
- `enrichment_enabled` 跨版块语义：话题命中的版块里，只要有一个 enabled 就允许触发；但只有 enabled 版块的工具才注册进 agent loop。

### F. 前端：演进分析报告（循环 B，★重定位 · 报刊式）
演进分析的产物是**一篇带引用的分析报告**，不是 dashboard 功能拼盘。已用原型 `prototype/evolution-report.html` 验证方向（报刊式长文，非 1/2/3 编号分块）。

**报告结构（原型已收敛）**：
- **报刊式长文**：单栏、衬线、drop-cap、双线 masthead。段落自然流动，不用编号分块。
- **演进定位**：导语一句话交代 + 文末小结条，不抢正文戏。
- **跨泳道关联写进正文叙事**：用泳道名标签（如"美伊冲突""霍尔木兹海峡安全""原油供需平衡""黄金避险配置"——持久话题泳道名，不是粗 SemanticBoard 大类）+ 传导机制自然语言。
- **双类引用（知识库式）**：
  - 📰 **新闻** `[1][2]`（红）：来自订阅源入库的新闻（循环A sections）。hover 看原话 + 来源 + 泳道。
  - 🔧 **工具查证** `[T1][T2]`（蓝）：agent 自主调用 opencli skill 查到的（web-search / market-quote / browser）。hover 看 skill 名 + query + 结果。
  - 文末资料来源分两组：新闻报道 / 工具查证。
- **行情降级为内联佐证**：不设固定金融区，行情只是 agent 可能调的 skill 之一，融进正文句子低调展示。
- **工作流黑盒**：历史汇总→脉络→搜索→佐证→分析这套流程用户无感，文末"关于这份报告"轻量交代。

**新闻背景（循环A）独立处理**：不在报告里揉新闻背景折叠区。循环A 的 period 汇总用独立 tab/区块（周期筛选+inline编辑），与报告分属两个独立产物、两个独立入口。

### F'. 个股深度辩论（外部 FinGenius，★降级 · 冻结 · 独立于演进主线）

**降级处理（2026-07-09）**：FinGenius 个股涨跌辩论与演进定位主线本质冲突（主线说"不预测涨跌"，辩论说"精确预测个股涨跌"）。经用户决策，**保留代码但降级隐藏**：

- **代码冻结**：`fingenius_client.go` / `stock_debate_result` 表 / `debate_distill` Operation / `DebateSection.vue` / 36 个测试，原样保留不动，不再作为主线发展。
- **定位隔离**：标注"金融可选模块 · 独立于演进定位主线"。
- **前端折叠**：`DebateSection.vue` 默认折叠，需用户主动展开；仅当话题命中金融版块时才提示可用。
- **GPL v3 合规**：不变（进程隔离调用独立 HTTP 服务，详见 design §11 决策⑥）。

## Capabilities

### Added Capabilities

- `data-enrichment`：版块↔数据源配置、分层新闻汇总（循环A，period 档案式）、**演进定位三角色增强（循环B，含演进定位+跨泳道信号+证据链）**、review judge 定位变化对比、手动触发（话题级跨版块）、演进分析报告（报刊式 + 双类引用）
- `data-enrichment`（金融可选模块 · 冻结）：**个股深度辩论**——FinGenius 6 角色辩论（外部 HTTP 服务，GPL v3 进程隔离），结果独立存表、前端④默认折叠。**不再作为主线发展**。

## Impact

- **后端**
  - 新表：`board_data_sources`、`topic_lifeline_context`、`topic_enrichment_result`、`topic_enrichment_review`、`stock_debate_result`（FinGenius，冻结）。
  - 新 domain：`internal/dataenrichment/`（tool registry + 三角色编排 + 循环A汇总 + review judge + handler + `fingenius_client.go`）。
  - ★ **重定位改动**：`orchestrator.go`（分析员 prompt + 输出 schema 改演进定位 + 工具按版块动态注册）、`review_judge.go`（定位变化对比，替涨跌兑现）、触发跨版块化。
  - 复用：`GetTopicLifeline`（14天详情渲染）、`airouter.Router.Chat`（必带 Operation/SessionID）。
  - 定时任务：循环A的周/月/年刷新 + 检查自愈。
- **前端**：
  - ★ **演进分析报告**（循环B产物）：报刊式长文 + 双类引用（📰新闻/🔧工具）+ 泳道名标签 + 跨泳道关联叙事。原型 `prototype/evolution-report.html` 已验证。
  - **新闻背景**（循环A产物）：独立 tab/区块，周期筛选+inline编辑（已验证，保留）。
  - FinGenius ④ 折叠降级。
- **AI 成本**：
  - 循环B单 topic：解读1 + 查询员每主题1-3 + 分析1 + review_judge1 ≈ 5-8 次 LLM 调用，全层读 ~2.5-3k token。
  - 循环A：每个 granularity 刷新 1 次 LLM 调用（summarize_context）。
  - 个股辩论：成本在 FinGenius 侧；手动触发，冻结不发展。
- **依赖关系**：
  - 可观测基础（`ai_call_logs` 表 + `airouter/store.go` + `SessionIDFromContext`）**代码层已落地**，不阻塞。
  - **FinGenius 外部服务**（冻结）：个股辩论④环节依赖用户另起 FinGenius HTTP 服务。不可用时④显出错态，不影响演进定位主线。
- **数据兼容**：5 张新表独立，历史数据无影响。★ 重定位涉及的 `topic_enrichment_result` / `topic_enrichment_review` 字段语义变更（sectors→position/signals/evidence，verdict→position_change）——表结构基本不动（jsonb），旧数据需清空重跑（重定位前的 result/review 是涨跌语义，不可复用）。
- **GPL v3 合规**：Syntopica 与 FinGenius 均 GPL v3。进程隔离调用不传染，本仓库不引入任何 FinGenius 源码。
- **部署后影响**：本 change 上线后，用户在话题管理界面点"演进分析"，agent 跨该话题命中的多条泳道聚合信号，产出一篇带引用的演进分析报告（报刊式）；循环A的新闻背景在独立 tab 可按周期翻看/编辑；**手动发起个股深度辩论（需另起 FinGenius 服务，否则④显出错态并默认折叠，不影响主线）**；需用户手动开 `enrichment_enabled` 才会跑增强（默认关）。历史 topic 无 context，循环A定时任务首次跑会批量补生成历史 period；重定位前的旧 result/review（涨跌语义）需清空重跑。
