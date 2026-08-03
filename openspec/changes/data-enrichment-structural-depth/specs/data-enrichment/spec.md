## MODIFIED Requirements

### Requirement: 板块数据源绑定

系统 SHALL 提供 `board_data_sources` 表，存储 SemanticBoard 与数据源的绑定关系。每行 SHALL 包含：`semantic_board_id`（外键）、`source_type`（系统已注册的数据源类型枚举）、`config`（板块级参数 jsonb）、`enabled`（默认 true）、时间戳。`(semantic_board_id, source_type)` SHALL 受唯一约束（同板块同源只绑一次）。`source_type` SHALL 受 CHECK 约束为系统已注册的数据源类型。`source_type` 枚举 SHALL 可扩展（未来接入结构化外部源时新增），但内置 SHALL NOT 包含任何金融行情类源（`etf_quote` / `exchange_rate` / `gdelt_event` 等已被移除）。建表 SHALL 通过显式迁移完成（开发执行规范 §10）。

#### Scenario: 同板块同源唯一

- **WHEN** 尝试对同一 board 插入两条相同 source_type
- **THEN** 系统 SHALL 因唯一约束拒绝第二条

#### Scenario: 金融 source_type 已移除

- **WHEN** 尝试插入 `source_type='etf_quote'`
- **THEN** 系统 SHALL 因 CHECK 约束拒绝（枚举已不含金融源）

### Requirement: 数据源工具注册表

系统 SHALL 维护一个数据源工具注册表（`internal/dataenrichment/service/tool_registry.go`），每个工具 SHALL 暴露 `Name`、`Description`、`InputSchema`、`Execute`。注册表 SHALL 至少注册：内部多级导航工具（`list_boards` / `list_lanes` / `get_lane_detail`）、`web_search`（真实联网检索，见『web 搜索与正文抓取数据源』）、`fetch_page`（网页正文抓取）。注册表 SHALL NOT 注册任何 A 股 / ETF / 金融行情类工具（`list_etf_by_keyword` / `get_etf_quote` / `list_sectors` 已移除）。工具 Execute SHALL 返回完整命中结果（不截断条目数量），仅精简字段以控制 token。未注册的工具名调用 SHALL 返回错误并列出可用工具。

#### Scenario: 工具返回完整命中

- **WHEN** `web_search(query="...")` 命中多条结果
- **THEN** 返回 SHALL 包含全部命中，不得截断条目数（避免 agent 误判死循环）

#### Scenario: 金融工具不可见

- **WHEN** agent loop 尝试调用 `get_etf_quote`
- **THEN** Execute SHALL 返回"未知工具"错误，可用工具列表中 SHALL NOT 含任何金融行情工具

#### Scenario: 未知工具拒绝

- **WHEN** agent loop 调用未注册的工具名 `get_stock_price`
- **THEN** Execute SHALL 返回错误，列出可用工具名

### Requirement: 分层上下文驱动的数据增强编排

数据增强编排的入口 SHALL 是分层上下文（`topic_lifeline_context` + 14天窗口详情 + 历史 applied review），**不是单篇新闻，也不是单一 lifeline**。编排 SHALL 由三角色组成：

1. **解读员（结构化分析编辑）**：全层读分层上下文（按板块 `context_layers`，未生成的层跳过），提炼需补数据的**研究方向**（领域自适应：历史机制 / 关键数据 / 可比案例，不限于金融产业方向），输出 JSON。SHALL NOT 硬编码提炼"A 股 ETF 方向"。
2. **研究助理（agent loop）**：对每个研究方向用 `web_search` + `fetch_page` + 内部导航工具链式检索，搜集背景事实、历史 precedents、专家/一手分析，喂给分析员的深度层。
3. **分析员（结构化分析师）**：结合分层上下文 + 检索数据，按形态+视角产出**事实层 + 深度层**（见『分层见解产出』『分析深度层产出』）。SHALL 显式产出反过度解读边界（`boundary`）。

编排 SHALL 对单次 LLM 调用设 max_loops 上限（默认 6）防止无限循环。解读员 SHALL 读取历史 applied review 以避免重蹈已知偏差。**SHALL NOT 产出** 旧主线的走向预测（`direction` / `confidence` / `horizon` / `trigger_up` / `trigger_down` 字段已废弃）。

#### Scenario: 消费分层上下文

- **WHEN** 触发某 topic 的数据增强
- **THEN** 解读员输入 SHALL 含表1 context（按 context_layers）+ 14天窗口详情 + 历史 applied review，不得为单篇 article 原文

#### Scenario: 解读员领域自适应

- **WHEN** 解读员处理非金融结构话题（如"人民币国际化进程"）
- **THEN** 提炼的研究方向 SHALL 覆盖历史机制/关键数据/可比案例，SHALL NOT 强制提炼 A 股 ETF 方向

#### Scenario: 分析员产出深度层而非走向预测

- **WHEN** 分析员产出非 sparse 形态结果
- **THEN** 结果 SHALL 含深度层（`depth` 块），SHALL NOT 含 `direction`/`trigger_up`/`trigger_down` 字段

#### Scenario: 死循环防御

- **WHEN** 研究助理尝试用相同参数重复调用同一工具
- **THEN** 系统 SHALL 拦截并返回"已调用过"提示，不执行重复调用

### Requirement: 话题形态判断

系统 SHALL 在分析前判断持久话题的形态，归入 `event_chain`（事件链）/ `theme_vein`（主题脉络）/ `single_point`（单点影响）/ `structural`（结构演化）/ `sparse`（骨感）之一。判据 SHALL 综合 `hit_count`（丰满度）、`section` 数（聚合度）、`cluster_label` 发散度（线性 vs 平行）与内容语义。`structural` 形态适用于**无离散事件、呈长时段结构演化**的话题（如"人民币国际化进程""美元霸权演变"）。形态枚举 SHALL 可扩展，新增形态无需改架构。

#### Scenario: 高频线性因果归事件链型

- **WHEN** 话题 hit_count ≥ 10 且 section 时序呈线性因果演进（如"官宣→否认→条款"）
- **THEN** 系统 SHALL 判 `event_chain`，产出因果链 + 推演见解

#### Scenario: 长时段结构演化归结构型

- **WHEN** 话题为持续性结构命题、无单一离散事件驱动（如"人民币国际化"）
- **THEN** 系统 SHALL 判 `structural`，产出结构演化叙述 + 深度层

#### Scenario: 单次命中归骨感型

- **WHEN** 话题 hit_count = 1（料严重不足）
- **THEN** 系统 SHALL 判 `sparse`，诚实标注信息不足，不产出推演见解与深度层

### Requirement: 探索 agent 工具集

系统 SHALL 向探索 agent（研究助理）注册多级入口探索工具 + `web_search` 联网检索 + `fetch_page` 正文抓取，**不注册任何金融行情工具**（金融工具已彻底移除，不再"降为可选"）。多级入口工具 SHALL 支持分层下钻：`list_boards`（看版块全景）→ `list_lanes`（版块下泳道）→ `get_lane_detail`（泳道详情按需取）。agent SHALL 自主决定下钻深度、检索查询与何时停止。

#### Scenario: 多级入口按需下钻

- **WHEN** 探索 agent 判断某版块可能与视角相关
- **THEN** agent SHALL 调 `list_lanes` 查该版块泳道，仅对相关泳道调 `get_lane_detail`，无关版块跳过

#### Scenario: web_search 与 fetch_page 配合取证

- **WHEN** 探索 agent 需验证事实节点或抓取一手原文支撑深度层
- **THEN** agent SHALL 调 `web_search` 检索，对关键命中调 `fetch_page` 取正文，结果纳入深度层 `evidence_chain`

#### Scenario: 金融工具彻底不可见

- **WHEN** 话题为任意形态（含金融相关）
- **THEN** 系统 SHALL NOT 注册金融行情工具，agent 全程不可见

### Requirement: 分层见解产出

系统 SHALL 产出分层分析：**事实层**（梳理 + 验证，铺垫）+ **见解层**（推演/假设/提问/视角，★产出主体）+ **深度层**（非 sparse 形态强制，见『分析深度层产出』）。见解层 SHALL 发挥 AI 多层推演 + 跨领域联想 + 假设性提问优势。产出结构 SHALL 随形态变（事件链=因果链+推演见解 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 结构=演化叙述 / 骨感=诚实标注）。

#### Scenario: 事实层与见解层分离

- **WHEN** 系统产出分析
- **THEN** 输出 SHALL 明确区分事实层（已验证 claim）与见解层（推演 insight），读者可辨哪是真哪是猜

#### Scenario: 非 sparse 形态强制深度层

- **WHEN** 系统产出 `event_chain`/`theme_vein`/`single_point`/`structural` 形态分析
- **THEN** 输出 SHALL 含完整深度层 `depth` 块；`sparse` 形态 SHALL NOT 产出深度层

### Requirement: 多形态分析报告

前端 SHALL 按话题形态渲染不同结构的分析报告（事件链=因果链+见解层 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 结构=演化叙述 / 骨感=信息不足标注）。报告 SHALL 展示形态判断依据、视角选择、时间线依据轴、引用（📰新闻/🌐网页/📄正文）、见解确定性分级视觉。报告 SHALL 渲染**深度层**区块（系统重定位 / 多层机制 / 历史类比 / 范式转折 / 边界限定 / 可核查证据链），证据链 SHALL 支持点击查看原文 URL。

#### Scenario: 按形态渲染不同结构

- **WHEN** 话题形态为 `theme_vein`
- **THEN** 报告 SHALL 渲染平行线索 + 跨线索洞察，不画因果链箭头

#### Scenario: 深度层区块可见

- **WHEN** 报告展示非 sparse 形态分析
- **THEN** 报告 SHALL 渲染深度层各字段（系统重定位/多层机制/历史类比/边界），证据链条目 SHALL 带可点击 URL

#### Scenario: 见解确定性视觉区分

- **WHEN** 报告展示见解层
- **THEN** 不同确定性（high/medium/low/question）SHALL 用不同视觉（颜色/标签）区分

## ADDED Requirements

### Requirement: web 搜索与正文抓取数据源

系统 SHALL 提供真实的 web 检索与正文抓取能力作为数据增强的通用外部数据源：`web_search` 工具 SHALL 接入真实搜索后端（首个实现为博查 Bocha，`api.bochaai.com`），返回 `[{title,url,snippet}]`；`fetch_page` 工具 SHALL 复用 `reader` 域 `readability_crawler` 抓取网页正文，返回 `{title,url,main_text}`。`web_search` SHALL 通过 `WebSearcher` 接口注入实现，便于未来替换/增加服务商而不动编排。搜索后端 API key SHALL 通过配置（`configs/config.yaml` + 环境变量 `BOCHA_API_KEY`）注入，SHALL NOT 硬编码。当 key 未配置或后端不可达时，`web_search` / `fetch_page` SHALL 返回错误 JSON 让 agent 自降级（沿用 registry 单工具失败约定），SHALL NOT 阻断 agent loop。

#### Scenario: web_search 返回真实结果

- **WHEN** agent 调 `web_search(query="1973 石油危机 资本回流")` 且博查 key 已配置
- **THEN** SHALL 返回真实搜索命中 `[{title,url,snippet}]`，SHALL NOT 返回"not configured"

#### Scenario: fetch_page 抓正文

- **WHEN** agent 调 `fetch_page(url="https://...")`
- **THEN** SHALL 返回该页 readability 提取的 `main_text`（超长截断），供深度层 `evidence_chain` 引用原文

#### Scenario: 无 key 优雅降级

- **WHEN** `BOCHA_API_KEY` 未配置
- **THEN** `web_search` SHALL 返回"web_search 未配置"错误 JSON，agent loop SHALL 继续运行（不阻断），深度层仍产出但证据链弱

#### Scenario: 搜索后端可替换

- **WHEN** 未来将博查换为 Tavily
- **THEN** 改动 SHALL 局限于新增一个 `WebSearcher` 实现 + wire 注入，SHALL NOT 触碰编排与 analyze prompt

### Requirement: 分析深度层产出

非 sparse 形态（`event_chain` / `theme_vein` / `single_point` / `structural`）的分析结果 SHALL 强制产出深度层 `depth` 块，映射结构化深度分析基因：`system_reframe`（一句话把事件放进哪个大系统讲）/ `mechanism_layers`（多层子机制拆解，每层给 `deep_logic` 深层逻辑 + 依据）/ `historical_analogy`（历史案例 + 机制类比 + 何处不同）/ `regime_shift`（范式转折判断，无则 null）/ `boundary`（显式反过度解读：什么还不能下结论）/ `evidence_chain`（可核查证据链，`source_type ∈ news|web|page`，web/page 带 `url`+`quote`+`institution`+`date`）。`boundary` SHALL 非空（强制反过度解读）。`sparse` 形态 SHALL NOT 产出深度层。

#### Scenario: 非 sparse 强制完整深度层

- **WHEN** 分析员产出 `structural` 形态结果
- **THEN** `depth` 块 SHALL 含 `system_reframe` / `mechanism_layers` / `historical_analogy` / `boundary` 非空，`evidence_chain` 至少 1 条

#### Scenario: 反过度解读强制

- **WHEN** 分析员产出任意非 sparse 结果
- **THEN** `depth.boundary` SHALL 非空，显式标注不能下结论的边界，SHALL NOT 空泛

#### Scenario: 证据链可核查

- **WHEN** 深度层引用外部依据
- **THEN** `evidence_chain` 中 web/page 类条目 SHALL 带 `url` + `quote` 原文摘录 + 来源机构 + 日期，可供前端点击核查

#### Scenario: 范式转折可缺省

- **WHEN** 话题无范式转折迹象
- **THEN** `depth.regime_shift` SHALL 为 null（不强行编造）

#### Scenario: sparse 不产深度层

- **WHEN** 形态为 `sparse`
- **THEN** 结果 SHALL 只含 `notice` + `summary`，SHALL NOT 含 `depth` 块
