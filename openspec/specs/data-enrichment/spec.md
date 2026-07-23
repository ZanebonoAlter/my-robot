# Purpose

数据增强（data enrichment）：持久话题的认知分析能力。以分层上下文为输入，经「解读员 → 探索/查询 agent → 分析员 → review judge」角色链产出分析结果，并支持报告追问。本 capability 由两个 change 合并建立：`data-enrichment-orchestration`（骨架：三表分离 / agent loop 三防御 / 可观测性 / 循环A 新闻汇总 / 板块配置）+ `causal-analysis-agent`（分析主线：话题形态判断 / 视角选择 / 分层见解 / 见解依据与确定性 / 报告追问）。

> 变更溯源：data-enrichment-orchestration（2026-07，骨架）+ causal-analysis-agent（2026-07，分析主线接管）。后者推翻前者的「金融走向预测」与「演进定位」两版主线，重定为「探索判断 agent——形态随话题变 + 见解为核心」。

# Requirements

## Requirement: 板块数据源绑定

系统 SHALL 提供 `board_data_sources` 表，存储 SemanticBoard 与数据源的绑定关系。每行 SHALL 包含：`semantic_board_id`（外键）、`source_type`（etf_quote/exchange_rate/gdelt_event 等枚举）、`config`（板块级参数 jsonb，如 ETF 代码/关键词、GDELT 地理过滤）、`enabled`（默认 true）、时间戳。

`(semantic_board_id, source_type)` SHALL 受唯一约束（同板块同源只绑一次）。`source_type` SHALL 受 CHECK 约束为系统已注册的数据源类型。

建表 SHALL 通过显式迁移完成（开发执行规范 §10）。

### Scenario: 同板块同源唯一

- **WHEN** 尝试对同一 board 插入两条 source_type=etf_quote
- **THEN** 系统 SHALL 因唯一约束拒绝第二条

### Scenario: 板块级配置隔离

- **WHEN** 板块A 配 etf_quote 的 config={"keywords":["半导体"]}，板块B 配 etf_quote 的 config={"keywords":["军工"]}
- **THEN** 两个绑定 SHALL 独立存在，互不影响

## Requirement: 板块分析配置

系统 SHALL 在板块级提供分析配置，含：`enrichment_enabled`（循环B增强开关，默认 false）、`window_days`（循环B实时详情窗口，默认 14）、`context_layers`（解读员读取的上下文层，默认 `["week","month","year","all"]`）。循环 A（新闻汇总）SHALL NOT 受板块配置控制——它是新闻汇总基础设施，全局定时跑。

### Scenario: 默认关闭增强

- **WHEN** 新建板块
- **THEN** `enrichment_enabled` 默认 false，循环B增强不跑

### Scenario: context_layers 裁剪

- **WHEN** 板块配置 `context_layers=["week","month"]`（去掉 year/all）
- **THEN** 解读员 SHALL 只读 week + month，跳过未选的层

## Requirement: 数据源工具注册表

系统 SHALL 维护一个数据源工具注册表（`internal/dataenrichment/registry.go`），每个工具 SHALL 暴露 `Name`、`Description`、`InputSchema`、`Execute`。首批 SHALL 至少注册：`list_etf_by_keyword`（按关键词筛全市场 ETF）、`get_etf_quote`（实时行情）、`list_sectors`（行业板块清单）。

工具 Execute SHALL 返回完整命中结果（不截断条目数量），仅精简字段以控制 token。未注册的工具名调用 SHALL 返回错误。

### Scenario: 工具返回完整命中

- **WHEN** `list_etf_by_keyword(keyword="半导体")` 命中 15 只 ETF
- **THEN** 返回 SHALL 包含全部 15 只的代码/名称/涨跌幅，不得截断条目数（避免 agent 误判死循环）

### Scenario: 未知工具拒绝

- **WHEN** agent loop 调用未注册的工具名 `get_stock_price`
- **THEN** Execute SHALL 返回错误，列出可用工具名

## Requirement: 分层新闻汇总上下文（循环 A）

系统 SHALL 维护一个独立于分析的新闻汇总循环（循环 A），产出每个持久话题的分层上下文。新表 `topic_lifeline_context` SHALL 存 week/month/year 四粒度的新闻叙事汇总（含相关数据波动快照），**按 `period` 档案式存储**（每周期独立一行，历史可翻，不再滚动覆盖；UNIQUE 为 `(topic_id, granularity, period)`）。每行 SHALL 带 `as_of_date`（汇总截止日，用于时效判断与检查自愈）。新闻汇总 SHALL 只基于话题的 sections（新闻原文），SHALL NOT 消费 `topic_enrichment_result` 或 `topic_enrichment_review`。

### Scenario: 汇总独立于分析

- **WHEN** 循环 A 运行生成汇总
- **THEN** 输入 SHALL 为话题的 sections，SHALL NOT 读取任何增强结果或 review

### Scenario: 时效可见

- **WHEN** 解读员/分析员读取 context
- **THEN** SHALL 能看到 `as_of_date`，以判断该汇总是否滞后于最新进展

## Requirement: 循环 A 触发与汇总算法

循环 A SHALL 定时触发（week 每周、month 每月、year 每年），**每个周期各产一条独立 period 行（新周期不覆盖旧周期）**。SHALL 内置检查自愈机制（扫描缺失历史 period 的 topic，按 period 逐个补），SHALL 支持手动重生成任意 period，SHALL 归档清理超期行（week>8 周、month>12 月）。

汇总算法：**每个 period 独立汇总成一条**——读「该 period 范围内的 sections」一次性汇总，SHALL NOT 与旧汇总合并覆盖。`all`（`period='all'`）为例外，滚动单行，读全部历史 sections + 旧 all 汇总增量合并。各 period SHALL 平行独立维护，不搞 week→month→year 层层金字塔合并（避免误差累积）。

### Scenario: 每 period 独立汇总

- **WHEN** 每月定时刷新某 topic 的 month（period=2026-06）
- **THEN** SHALL 读 2026-06 范围内 sections 独立汇总成一条，SHALL NOT 覆盖 2026-05 的 month 汇总

### Scenario: 历史周期可翻

- **WHEN** 用户在前端按周期筛选查看 2026-05 的 month
- **THEN** 系统 SHALL 返回该 period 独立存的汇总行（未被新周期合并覆盖）

### Scenario: 检查自愈补遗漏 period

- **WHEN** 某 topic 从未生成过 2026-05 的 month 汇总
- **THEN** 检查自愈 SHALL 补生成该 period，`as_of_date` 顺序推进

### Scenario: 用户手动首次生成缺失 period

- **WHEN** 用户在前端①选一个该 topic 从未生成过的 period（如 2026-W25，表1无对应行）
- **THEN** 系统 SHALL 允许手动触发该 period 的汇总（调 `POST .../contexts/:granularity/regenerate?period=...`），通过 Upsert 首次插入该 period 行，SHALL NOT 要求用户等待检查自愈排队补；生成后该 period 立即出现在可翻阅列表中。此入口与「翻历史」并列，覆盖「已存在重算」和「从未生成首次生成」两种情况

## Requirement: 分层上下文驱动的数据增强编排

数据增强编排的入口 SHALL 是分层上下文（`topic_lifeline_context` + 14天窗口详情 + 历史 applied review），**不是单篇新闻，也不是单一 lifeline**。编排 SHALL 由三角色组成：

1. **解读员**：全层读分层上下文（按板块 `context_layers`，未生成的层跳过），提炼需补数据的产业主题（JSON）。
2. **查询员（agent loop）**：对每个主题用板块绑定数据源工具链式查询，支持换词（命中 0 时换宽泛词）。
3. **分析员**：结合分层上下文 + 实时数据，判断"最新进展在演进中的意义"（强化/转折/扩散），引用对应 period 作对比基准；`as_of_date` 滞后时以 14 天详情为准。每个板块 SHALL 输出走向预测（`direction`/`confidence`/`horizon`）+ 凭什么判断（`reasoning`：信号→机制）+ **原始依据（`evidence`：引用的 context 原话 + context 指针，供前端 tooltip 展示）** + 板块专属触发条件（`trigger_up`/`trigger_down`）。SHALL NOT 下沉到个股买卖建议。

编排 SHALL 对单次 LLM 调用设 max_loops 上限（默认 6）防止无限循环。解读员 SHALL 读取历史 applied review 以避免重蹈已知偏差。

### Scenario: 消费分层上下文

- **WHEN** 触发某 topic 的数据增强
- **THEN** 解读员输入 SHALL 含表1 context（按 context_layers）+ 14天窗口详情 + 历史 applied review，不得为单篇 article 原文

### Scenario: agent loop 链式查询

- **WHEN** 查询员处理主题"光刻机"，`list_etf_by_keyword("光刻机")` 命中 0
- **THEN** 查询员 SHALL 换宽泛词（如"半导体"）重查，再对命中代码调 `get_etf_quote`

### Scenario: 死循环防御

- **WHEN** 查询员尝试用相同参数重复调用同一工具
- **THEN** 系统 SHALL 拦截并返回"已调用过"提示，不执行重复调用

## Requirement: agent loop 的三个强制防御

数据增强 agent loop SHALL 内置三项防御（PoC 验证过的坑）：

1. **关闭 Qwen3 thinking**：对本地 Qwen3 模型，LLM 请求 SHALL 传 `chat_template_kwargs.enable_thinking=false`，避免 thinking 烧光 token 导致 content 空。
2. **历史结果不截断**：agent loop 累积的工具调用历史 SHALL 给完整结果，不得截断（截断会导致 agent 误判"没拿全"而死循环重查）。
3. **去重拦截**：相同工具名 + 相同参数的调用 SHALL 被拦截并返回提示。

### Scenario: thinking 关闭

- **WHEN** agent loop 调用本地 Qwen3
- **THEN** 请求 payload SHALL 包含 `chat_template_kwargs: {enable_thinking: false}`

### Scenario: 历史完整传递

- **WHEN** agent loop 进入第 N 轮
- **THEN** 喂给 LLM 的历史调用块 SHALL 包含前 N-1 步的完整工具结果（非截断摘要）

## Requirement: 分析认知循环 review judge

每次增强产出 result 后，系统 SHALL 触发 review judge（一次 LLM 半自动调用），**对照上次预测方向 vs 期间实际走势**，逐板块结算兑现度（`verdict`：`[{sector, predicted_dir, actual, mark: hit|part|miss}]`），输出 JSON `{should_review, reason, verdict, deviation_summary, affected_context, confidence}`。`should_review=true` 时 SHALL 写入 `topic_enrichment_review` 表；`false` 时 SHALL 跳过（避免噪音）。review SHALL 关联 `prev_result_id` 与 `curr_result_id`。`deviation_summary` SHALL 支持 LLM 生成基底 + 人工调整。第一次增强（无 prev_result）或 prev 无 `direction` 字段（旧格式）SHALL 跳过或降级为描述对比。

**用户手动批注**：除 review_judge 自动生成外，用户 SHALL 能在 CRUD 界面手动创建 review（对某 result 批注）。手动批注的 review：`source='manual'`，`prev_result_id` 可空，`deviation_summary` 由用户填写，`applied` 默认 true。

### Scenario: 半自动判断避免噪音

- **WHEN** review judge 判断 `should_review=false`（如仅置信度微调、无核心判断变化）
- **THEN** SHALL 不写 review 行

### Scenario: 兑现度结算

- **WHEN** 上次预测"原油上行(高)"，期间实际 +4.2%
- **THEN** review.verdict SHALL 记录该板块 mark=hit（兑现），并附 deviation_summary 说明

### Scenario: 用户手动批注

- **WHEN** 用户在前端对某 result 手动写一条批注
- **THEN** SHALL 创建 review（source=manual，prev_result_id 可空，applied 默认 true），不依赖 review_judge

## Requirement: review 不回写新闻记忆

review 的 `applied` 标记 SHALL NOT 触发对 `topic_lifeline_context` 的任何写操作。表1 SHALL 永远只随循环 A 新闻汇总变（保持新闻事实客观）。`applied=true` 仅表示"该认知已被用户采纳"，下次增强解读员 SHALL 读取历史 `applied=true` 的 review 作为输入，以避免重蹈已知偏差。

### Scenario: 采纳不污染记忆

- **WHEN** 用户对某 review 点"采纳"（`applied=true`）
- **THEN** `topic_lifeline_context` SHALL NOT 被修改

### Scenario: 读历史 review 避错

- **WHEN** 下次增强解读员读取输入
- **THEN** SHALL 包含历史 `applied=true` 的 review（如"上次因 X 误判黄金跌"）

## Requirement: 仅手动触发（不挂日报管线）

数据增强（循环 B）SHALL **仅由用户手动触发**（CRUD 界面"重新分析某话题"），SHALL NOT 自动挂载到日报管线。理由：不是所有板块对金融数据有影响（如"开发工具"板块），自动增强无意义且浪费成本。

- SHALL 只对 `enrichment_enabled=true` 的板块允许触发。
- 增强结果 SHALL 写入独立的 `topic_enrichment_result` 表（快照不可变，不含 report_id），**不得修改** `daily_report_sections.persistent_topic_id` 或任何 topic 的 status/lifecycle。
- 增强失败 SHALL 只记日志告警，不影响其他（手动触发天然隔离）。

### Scenario: 仅手动触发

- **WHEN** 用户在 CRUD 界面对某 topic 点"重新分析"
- **THEN** SHALL 立即跑一次增强，不依赖任何日报管线调度

### Scenario: 只读不污染主数据

- **WHEN** 数据增强完成
- **THEN** `daily_report_sections` 与 `board_persistent_topics` 的所有字段 SHALL 不被增强流程修改

## Requirement: 数据增强全程可观测与可追溯

数据增强的所有 LLM 调用 SHALL 经 airouter 并带 `Operation` 与统一 `SessionID`，写入 `ai_call_logs`。`Operation` SHALL 至少包含：`data_enrichment.interpret` / `data_enrichment.tool_use` / `data_enrichment.analyze` / `data_enrichment.review_judge` / `data_enrichment.summarize_context`。

可观测基础（`ai_call_logs` 表 + `airouter/store.go` 写日志 + `SessionIDFromContext`）SHALL 视为已就绪——不阻塞本 change 开工。

**可追溯（变更大、易出错，强化）**：所有切片的输入输出 SHALL 可查、可重建：

- 每次 LLM 调用的输入（messages）+ 输出（content）→ `ai_call_logs`。
- 每次工具调用的参数 + 返回摘要 + 耗时 → `topic_enrichment_result.tool_calls`（jsonb）。
- 编排元数据（本次增强读了哪些 context 层 + 各层 `as_of_date`、14 天详情的 section 范围、引用的历史 review id 列表）→ `topic_enrichment_result.input_snapshot`（jsonb）。
- 三表（context/result/review）SHALL 持久化全部中间结论。

一次循环 B 增强内的全部 LLM 调用 SHALL 能通过 `session_id` 在 `GET /api/ai/call-logs` 重建（含解读 + N 次工具循环 + 分析 + review_judge）。

### Scenario: 按 session 回放增强

- **WHEN** 查询 `GET /api/ai/call-logs?session_id=data_enrichment_7_<uuid>`
- **THEN** SHALL 返回该 topic 增强期间的全部 LLM 调用（解读 + N 次工具循环 + 分析 + review_judge），按时间正序

### Scenario: input_snapshot 记录编排上下文

- **WHEN** 一次增强完成
- **THEN** `topic_enrichment_result.input_snapshot` SHALL 记录读了哪些 context 层、各层 as_of、section 范围、引用的 review id

## Requirement: 板块 tab 「认知工作台」界面

板块详情页 SHALL 新增「数据增强」tab（与"板块内容/日报/文章"并列），按**用户认知任务流**组织（不再四张表平铺 CRUD），含四步：

- **① 最近怎么了（新闻记忆 / context）**：SHALL 提供**周期筛选器**（粒度下拉 + period 翻页）翻历史 + 结构化叙事段落 inline 编辑。
- **② 会往哪走（走向预测 / result.sectors）**：SHALL 提供**板块可展开卡片**（凭什么判断[信号→机制] + 支撑数据 + 板块专属触发条件）+ **证据链 tooltip**（悬停显示 `evidence.quote` 原话，**原地展示不跳转**）。
- **③ 猜得准吗（预测兑现复盘 / review）**：SHALL 提供时间轴 + 逐条兑现结算（hit/part/miss）+ 采纳。
- **④ 数据源/参数（board_data_sources）**：折叠高级区。

界面 SHALL 禁用后端术语（granularity/session_id/evolution_assessment 等），用人话。SHALL 提供完整交互态（loading/empty/error）+ 双主题（editorial/dark）。数据契约 SHALL 设计为侦探墙重构时可直接复用。

### Scenario: 周期筛选翻历史

- **WHEN** 用户在①选粒度"月"+ 翻到 period 2026-05
- **THEN** SHALL 展示该 period 独立存的新闻汇总（非当前周期覆盖）

### Scenario: 证据链 tooltip 不跳转

- **WHEN** 用户在②悬停某板块的判断信号
- **THEN** SHALL 原地 tooltip 显示引用的新闻原话（来自 evidence.quote），SHALL NOT 跳转到其他区域

### Scenario: 兑现度复盘可见

- **WHEN** 用户打开③
- **THEN** SHALL 看到每条历史预测对照实际走势的 hit/part/miss 结算

### Scenario: 契约为侦探墙铺路

- **WHEN** 设计 API 形状
- **THEN** 数据结构 SHALL 可被侦探墙重构直接复用

## Requirement: 话题形态判断

系统 SHALL 在分析前判断持久话题的形态，归入 `event_chain`（事件链）/ `theme_vein`（主题脉络）/ `single_point`（单点影响）/ `sparse`（骨感）之一。判据 SHALL 综合 `hit_count`（丰满度）、`section` 数（聚合度）、`cluster_label` 发散度（线性 vs 平行）与内容语义。形态枚举 SHALL 可扩展，新增形态无需改架构。

### Scenario: 高频线性因果归事件链型

- **WHEN** 话题 hit_count ≥ 10 且 section 时序呈线性因果演进（如"官宣→否认→条款"）
- **THEN** 系统 SHALL 判 `event_chain`，产出因果链 + 推演见解

### Scenario: 单次命中归骨感型

- **WHEN** 话题 hit_count = 1（料严重不足）
- **THEN** 系统 SHALL 判 `sparse`，诚实标注信息不足，不产出推演见解

### Scenario: AI 大主题归脉络型

- **WHEN** 话题 section 的 cluster_label 高度发散、无线性因果特征（如"产业范式转移"下多 AI 线索并行）
- **THEN** 系统 SHALL 判 `theme_vein`，产出平行线索 + 跨线索洞察

## Requirement: 分析视角候选与选择

系统 SHALL 采用「agent 提候选 + 用户选」模式（模式丙）：解读员 SHALL 输出多个**具体可讨论的视角候选**（问题式，如"美国为何反复横跳"，非"博弈论"等抽象标签），用户选定一个视角后，探索 agent 按该视角推演。第一版 SHALL 为单视角。视角来源 SHALL 抽象为可扩展接口（首批 agent 生成，预留外部源如视频评论员/研报）。

### Scenario: agent 提具体视角候选

- **WHEN** 解读员完成形态判断
- **THEN** 系统 SHALL 输出 ≥ 2 个具体问题式视角候选，每个带视角名 + 一句话说明

### Scenario: 用户选定视角驱动推演

- **WHEN** 用户从候选选一个视角（或自填）
- **THEN** 探索 agent SHALL 仅按该视角推演见解，不发散到未选视角

### Scenario: 视角来源可扩展

- **WHEN** 未来接入外部视角源（如视频评论员）
- **THEN** 系统 SHALL 通过 LensSource 接口注入候选，不动核心编排

## Requirement: 探索 agent 工具集

系统 SHALL 向探索 agent 注册多级入口探索工具 + web_search 验证工具，替换纯金融工具（金融工具降为可选，仅金融话题注册）。多级入口工具 SHALL 支持分层下钻：`list_boards`（看版块全景）→ `list_lanes`（版块下泳道）→ `get_lane_detail`（泳道详情按需取）。agent SHALL 自主决定下钻深度与何时停止。

### Scenario: 多级入口按需下钻

- **WHEN** 探索 agent 判断某版块可能与视角相关
- **THEN** agent SHALL 调 `list_lanes` 查该版块泳道，仅对相关泳道调 `get_lane_detail`，无关版块跳过

### Scenario: web_search 双重用途

- **WHEN** 探索 agent 需验证事实节点或支撑推演中间环节
- **THEN** agent SHALL 调 `web_search`，结果纳入见解依据与确定性判断

### Scenario: 金融工具按需注册

- **WHEN** 话题非金融形态
- **THEN** 系统 SHALL 不注册金融行情工具，agent 不可见

## Requirement: 分层见解产出

系统 SHALL 产出分层分析：**事实层**（梳理 + 验证，铺垫）+ **见解层**（推演/假设/提问/视角，★产出主体）。见解层 SHALL 发挥 AI 多层推演 + 跨领域联想 + 假设性提问优势。产出结构 SHALL 随形态变（事件链=因果链+推演见解 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 骨感=诚实标注）。

### Scenario: 事实层与见解层分离

- **WHEN** 系统产出分析
- **THEN** 输出 SHALL 明确区分事实层（已验证 claim）与见解层（推演 insight），读者可辨哪是真哪是猜

### Scenario: 见解随视角与形态变

- **WHEN** 同一话题换视角或换形态
- **THEN** 见解层产出 SHALL 不同（视角换→见解维度换；形态换→产出结构换）

## Requirement: 见解依据与确定性分级

每条见解 SHALL 挂**文章依据**（新闻原文引用）+ **时间线依据**（事件时序节点），不悬空发散。每条见解 SHALL 标注确定性分级：`high`（已验证）/ `medium`（推演·有据）/ `low`（假设·情景）/ `question`（提问·指出条件非预言成败）。见解 SHALL 给推演逻辑（凭什么 A→B），关键中间环节 SHALL 可 web 验证。

### Scenario: 见解必须挂依据

- **WHEN** 探索 agent 产出一条见解
- **THEN** 该见解 SHALL 引用 ≥ 1 篇文章依据或时间线节点，无依据的见解 SHALL 被拒绝

### Scenario: 确定性分级标注

- **WHEN** 见解为推演而非已验证事实
- **THEN** 系统 SHALL 标注 `medium`/`low`/`question` 之一，前端按级视觉区分

### Scenario: 提问式见解不预言成败

- **WHEN** 见解为 `question` 级
- **THEN** 系统 SHALL 指出决定成败的条件，而非预言成败结果

## Requirement: 骨感型诚实标注

对料严重不足的话题（如 hit_count = 1，占库 65%），系统 SHALL 诚实标注"信息不足"，不强行推演见解。系统 SHALL 将此类话题列入观察，持续命中后自动升级分析。知道何时不该分析 SHALL 视为核心判断力。

### Scenario: 骨感型不硬推演

- **WHEN** 话题被判 `sparse` 形态
- **THEN** 系统 SHALL 产出"信息不足"标注 + 轻量摘要，不产出推演见解

### Scenario: 持续命中后升级

- **WHEN** 原 sparse 话题持续命中、积累演进特征
- **THEN** 系统 SHALL 重新判形态并升级为对应分析

## Requirement: 分析认知对比

系统 SHALL 在每次分析后对照上次，记录 `new_findings`（新见解）/ `overturned`（推翻的旧见解）/ `confidence_shift`（确定性变化），取代旧的"定位变化对比"。review SHALL 不回写新闻记忆（表1），仅标记认知采纳。

### Scenario: 记录新发现与推翻

- **WHEN** 本次分析完成
- **THEN** review SHALL 对比上次 insight_layer，输出新增/推翻/确定性变化

### Scenario: 不回写新闻记忆

- **WHEN** review 标记认知采纳
- **THEN** 系统 SHALL 仅在表3 记录，不污染表1（新闻记忆保持客观）

## Requirement: 多形态分析报告

前端 SHALL 按话题形态渲染不同结构的分析报告（事件链=因果链+见解层 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 骨感=信息不足标注）。报告 SHALL 展示形态判断依据、视角选择、时间线依据轴、双类引用（📰新闻/🔧工具）、见解确定性分级视觉。

### Scenario: 按形态渲染不同结构

- **WHEN** 话题形态为 `theme_vein`
- **THEN** 报告 SHALL 渲染平行线索 + 跨线索洞察，不画因果链箭头

### Scenario: 见解确定性视觉区分

- **WHEN** 报告展示见解层
- **THEN** 不同确定性（high/medium/low/question）SHALL 用不同视觉（颜色/标签）区分

### Scenario: 视角选择可交互

- **WHEN** 用户触发分析
- **THEN** 报告入口 SHALL 展示 agent 视角候选，用户可选/自填

## Requirement: 报告追问交互

系统 SHALL 支持用户在分析报告页针对报告内容多轮追问。追问 agent SHALL 复用探索 agent 循环（多级入口 + web_search），上下文 SHALL 带报告本身（analysis + 依据 + 视角 + 形态）。回答 SHALL 带双类引用 + 确定性标注。追问会话 SHALL 持久化（多轮历史）。追问新依据 SHALL 支持用户手动沉淀回报告（标记来源），不自动改报告。

### Scenario: 追问复用探索工具

- **WHEN** 用户问"这条见解还有别的证据吗"
- **THEN** 追问 agent SHALL 调 web_search/get_lane_detail 补充，回答带新依据引用

### Scenario: 多轮追问持久化

- **WHEN** 用户在同一报告下连续追问
- **THEN** 系统 SHALL 保存多轮问答历史（topic_enrichment_qa），支持回看

### Scenario: 追问发现手动沉淀

- **WHEN** 追问中产生有价值的新依据
- **THEN** 系统 SHALL 支持用户手动沉淀回报告（标记 source=qa），不自动改报告
