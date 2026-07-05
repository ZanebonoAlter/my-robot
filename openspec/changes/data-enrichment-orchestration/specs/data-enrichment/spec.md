## ADDED Requirements

### Requirement: 板块数据源绑定

系统 SHALL 提供 `board_data_sources` 表，存储 SemanticBoard 与数据源的绑定关系。每行 SHALL 包含：`semantic_board_id`（外键）、`source_type`（etf_quote/exchange_rate/gdelt_event 等枚举）、`config`（板块级参数 jsonb，如 ETF 代码/关键词、GDELT 地理过滤）、`enabled`（默认 true）、时间戳。

`(semantic_board_id, source_type)` SHALL 受唯一约束（同板块同源只绑一次）。`source_type` SHALL 受 CHECK 约束为系统已注册的数据源类型。

建表 SHALL 通过显式迁移完成（开发执行规范 §10）。

#### Scenario: 同板块同源唯一

- **WHEN** 尝试对同一 board 插入两条 source_type=etf_quote
- **THEN** 系统 SHALL 因唯一约束拒绝第二条

#### Scenario: 板块级配置隔离

- **WHEN** 板块A 配 etf_quote 的 config={"keywords":["半导体"]}，板块B 配 etf_quote 的 config={"keywords":["军工"]}
- **THEN** 两个绑定 SHALL 独立存在，互不影响

### Requirement: 板块分析配置

系统 SHALL 在板块级提供分析配置，含：`enrichment_enabled`（循环B增强开关，默认 false）、`window_days`（循环B实时详情窗口，默认 14）、`context_layers`（解读员读取的上下文层，默认 `["week","month","year","all"]`）。循环 A（新闻汇总）SHALL NOT 受板块配置控制——它是新闻汇总基础设施，全局定时跑。

#### Scenario: 默认关闭增强

- **WHEN** 新建板块
- **THEN** `enrichment_enabled` 默认 false，循环B增强不跑

#### Scenario: context_layers 裁剪

- **WHEN** 板块配置 `context_layers=["week","month"]`（去掉 year/all）
- **THEN** 解读员 SHALL 只读 week + month，跳过未选的层

### Requirement: 数据源工具注册表

系统 SHALL 维护一个数据源工具注册表（`internal/dataenrichment/registry.go`），每个工具 SHALL 暴露 `Name`、`Description`、`InputSchema`、`Execute`。首批 SHALL 至少注册：`list_etf_by_keyword`（按关键词筛全市场 ETF）、`get_etf_quote`（实时行情）、`list_sectors`（行业板块清单）。

工具 Execute SHALL 返回完整命中结果（不截断条目数量），仅精简字段以控制 token。未注册的工具名调用 SHALL 返回错误。

#### Scenario: 工具返回完整命中

- **WHEN** `list_etf_by_keyword(keyword="半导体")` 命中 15 只 ETF
- **THEN** 返回 SHALL 包含全部 15 只的代码/名称/涨跌幅，不得截断条目数（避免 agent 误判死循环）

#### Scenario: 未知工具拒绝

- **WHEN** agent loop 调用未注册的工具名 `get_stock_price`
- **THEN** Execute SHALL 返回错误，列出可用工具名

### Requirement: 分层新闻汇总上下文（循环 A）

系统 SHALL 维护一个独立于分析的新闻汇总循环（循环 A），产出每个持久话题的分层上下文。新表 `topic_lifeline_context` SHALL 存 week/month/year/all 四粒度的新闻叙事汇总（含相关数据波动快照），每行 SHALL 带 `as_of_date`（汇总截止日，用于时效判断与检查自愈）。新闻汇总 SHALL 只基于话题的 sections（新闻原文），SHALL NOT 消费 `topic_enrichment_result` 或 `topic_enrichment_review`。

#### Scenario: 汇总独立于分析

- **WHEN** 循环 A 运行生成汇总
- **THEN** 输入 SHALL 为话题的 sections，SHALL NOT 读取任何增强结果或 review

#### Scenario: 时效可见

- **WHEN** 解读员/分析员读取 context
- **THEN** SHALL 能看到 `as_of_date`，以判断该汇总是否滞后于最新进展

### Requirement: 循环 A 触发与汇总算法

循环 A SHALL 定时触发（week 每周、month 每月、year 每年），SHALL 内置检查自愈机制（扫描 `as_of_date` 滞后或缺失的 topic 补生成），SHALL 支持手动重生成任意 granularity。

汇总算法：`week` SHALL 直接读"最近 7 天 sections"重算（例外，不走增量合并）；`month`/`year`/`all` SHALL 用「自上次汇总以来的增量 sections + 该 granularity 旧汇总」LLM 合并生成。各 granularity SHALL 平行维护自己的滚动窗口，不搞 week→month→year 层层金字塔合并（避免误差累积）。

#### Scenario: week 直接重算

- **WHEN** 每周定时刷新某 topic 的 week
- **THEN** SHALL 读最近 7 天全部 sections 重算，SHALL NOT 依赖旧 week 汇总

#### Scenario: month 增量合并

- **WHEN** 每月定时刷新某 topic 的 month
- **THEN** SHALL 读自上次汇总以来的增量 sections + 旧 month 汇总，LLM 合并生成新 month

#### Scenario: 检查自愈补漏

- **WHEN** 定时任务发现某 topic 的 month 从未生成（或宕机漏跑某周）
- **THEN** SHALL 补生成该缺口的汇总

### Requirement: 分层上下文驱动的数据增强编排

数据增强编排的入口 SHALL 是分层上下文（`topic_lifeline_context` + 14天窗口详情 + 历史 applied review），**不是单篇新闻，也不是单一 lifeline**。编排 SHALL 由三角色组成：

1. **解读员**：全层读分层上下文（按板块 `context_layers`，未生成的层跳过），提炼需补数据的产业主题（JSON）。
2. **查询员（agent loop）**：对每个主题用板块绑定数据源工具链式查询，支持换词（命中 0 时换宽泛词）。
3. **分析员**：结合分层上下文 + 实时数据，判断"最新进展在演进中的意义"（强化/转折/扩散），引用对应层作对比基准；`as_of_date` 滞后时以 14 天详情为准。

编排 SHALL 对单次 LLM 调用设 max_loops 上限（默认 6）防止无限循环。解读员 SHALL 读取历史 applied review 以避免重蹈已知偏差。

#### Scenario: 消费分层上下文

- **WHEN** 触发某 topic 的数据增强
- **THEN** 解读员输入 SHALL 含表1 context（按 context_layers）+ 14天窗口详情 + 历史 applied review，不得为单篇 article 原文

#### Scenario: agent loop 链式查询

- **WHEN** 查询员处理主题"光刻机"，`list_etf_by_keyword("光刻机")` 命中 0
- **THEN** 查询员 SHALL 换宽泛词（如"半导体"）重查，再对命中代码调 `get_etf_quote`

#### Scenario: 死循环防御

- **WHEN** 查询员尝试用相同参数重复调用同一工具
- **THEN** 系统 SHALL 拦截并返回"已调用过"提示，不执行重复调用

### Requirement: agent loop 的三个强制防御

数据增强 agent loop SHALL 内置三项防御（PoC 验证过的坑）：

1. **关闭 Qwen3 thinking**：对本地 Qwen3 模型，LLM 请求 SHALL 传 `chat_template_kwargs.enable_thinking=false`，避免 thinking 烧光 token 导致 content 空。
2. **历史结果不截断**：agent loop 累积的工具调用历史 SHALL 给完整结果，不得截断（截断会导致 agent 误判"没拿全"而死循环重查）。
3. **去重拦截**：相同工具名 + 相同参数的调用 SHALL 被拦截并返回提示。

#### Scenario: thinking 关闭

- **WHEN** agent loop 调用本地 Qwen3
- **THEN** 请求 payload SHALL 包含 `chat_template_kwargs: {enable_thinking: false}`

#### Scenario: 历史完整传递

- **WHEN** agent loop 进入第 N 轮
- **THEN** 喂给 LLM 的历史调用块 SHALL 包含前 N-1 步的完整工具结果（非截断摘要）

### Requirement: 分析认知循环 review judge

每次增强产出 result 后，系统 SHALL 触发 review judge（一次 LLM 半自动调用），对比上次 result 与本次 result，输出 JSON `{should_review, reason, deviation_summary, affected_context, confidence}`。`should_review=true` 时 SHALL 写入 `topic_enrichment_review` 表；`false` 时 SHALL 跳过（避免噪音）。review SHALL 关联 `prev_result_id` 与 `curr_result_id`。`deviation_summary` SHALL 支持 LLM 生成基底 + 人工调整。第一次增强（无 prev_result）SHALL 跳过 review judge。

#### Scenario: 半自动判断避免噪音

- **WHEN** review judge 判断 `should_review=false`（如仅置信度微调、无核心判断变化）
- **THEN** SHALL 不写 review 行

#### Scenario: 偏差记录

- **WHEN** result 从"原油承压"变为"原油强化"
- **THEN** review SHALL 记录 `deviation_summary` 说明为什么核心判断反转

### Requirement: review 不回写新闻记忆

review 的 `applied` 标记 SHALL NOT 触发对 `topic_lifeline_context` 的任何写操作。表1 SHALL 永远只随循环 A 新闻汇总变（保持新闻事实客观）。`applied=true` 仅表示"该认知已被用户采纳"，下次增强解读员 SHALL 读取历史 `applied=true` 的 review 作为输入，以避免重蹈已知偏差。

#### Scenario: 采纳不污染记忆

- **WHEN** 用户对某 review 点"采纳"（`applied=true`）
- **THEN** `topic_lifeline_context` SHALL NOT 被修改

#### Scenario: 读历史 review 避错

- **WHEN** 下次增强解读员读取输入
- **THEN** SHALL 包含历史 `applied=true` 的 review（如"上次因 X 误判黄金跌"）

### Requirement: 日报管线只读覆盖层 + 手动触发

数据增强 SHALL 作为日报管线的可选只读步骤，遵循 TopicWatch 范式：

- SHALL 在日报 `SaveReport` 事务**外**执行（报告存完后跑），失败 SHALL 只记录告警不阻断日报生成。
- SHALL 只对 `enrichment_enabled=true` 的板块的活跃 topic 执行。
- 增强结果 SHALL 写入独立的 `topic_enrichment_result` 表（快照不可变），**不得修改** `daily_report_sections.persistent_topic_id` 或任何 topic 的 status/lifecycle。
- SHALL 支持手动触发（CRUD 界面"重新分析某话题"），不依赖日报管线。

#### Scenario: 失败不阻断日报

- **WHEN** 数据增强编排因 LLM 超时或数据源不可用失败
- **THEN** 日报 SHALL 正常生成保存，增强失败只记 Warnf 日志

#### Scenario: 只读不污染主数据

- **WHEN** 数据增强完成
- **THEN** `daily_report_sections` 与 `board_persistent_topics` 的所有字段 SHALL 不被增强流程修改

#### Scenario: 手动触发

- **WHEN** 用户在 CRUD 界面对某 topic 点"重新分析"
- **THEN** SHALL 立即跑一次增强，不依赖日报管线调度

### Requirement: 数据增强全程可观测

数据增强的所有 LLM 调用 SHALL 经 airouter 并带 `Operation` 与统一 `SessionID`，写入 `ai_call_logs`。`Operation` SHALL 至少包含：`data_enrichment.interpret` / `data_enrichment.tool_use` / `data_enrichment.analyze` / `data_enrichment.review_judge` / `data_enrichment.summarize_context`。工具调用（非 LLM）SHALL 记录工具名/参数/返回摘要/耗时，存入 `topic_enrichment_result.tool_calls`（jsonb）。

可观测基础（`ai_call_logs` 表 + `airouter/store.go` 写日志 + `SessionIDFromContext`）SHALL 视为已就绪——不阻塞本 change 开工。

一次循环 B 增强内的全部 LLM 调用 SHALL 能通过 `session_id` 在 `GET /api/ai/call-logs` 重建（含解读 + N 次工具循环 + 分析 + review_judge）。

#### Scenario: 按 session 回放增强

- **WHEN** 查询 `GET /api/ai/call-logs?session_id=data_enrichment_7_<uuid>`
- **THEN** SHALL 返回该 topic 增强期间的全部 LLM 调用（解读 + N 次工具循环 + 分析 + review_judge），按时间正序

### Requirement: 板块 tab CRUD 界面

板块详情页 SHALL 新增「数据增强」tab（与"板块内容/日报/文章"并列），提供三表 CRUD：

- **表1 `topic_lifeline_context`**：查看 week/month/year/all + 手动重生成某 granularity + 人工编辑 content。
- **表2 `topic_enrichment_result`**：查看（含 LLM 调用 trace，点 session_id 可回放）+ 手动触发增强。
- **表3 `topic_enrichment_review`**：查看认知演进史 + 人工调整 deviation_summary + 采纳（applied）。

CRUD 的数据契约（API 形状）SHALL 设计为侦探墙重构时可直接复用，避免替换时返工。侦探墙（`TopicDetectiveWall.client.vue`）重构不在本 change 范围。

#### Scenario: 三表 CRUD 可用

- **WHEN** 用户打开某板块的「数据增强」tab
- **THEN** SHALL 能查看/编辑/触发三表内容

#### Scenario: 契约为侦探墙铺路

- **WHEN** 设计 CRUD 的 API 形状
- **THEN** 数据结构 SHALL 可被侦探墙重构直接复用
