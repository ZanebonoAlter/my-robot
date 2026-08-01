## ADDED Requirements

### Requirement: 话题形态判断

系统 SHALL 在分析前判断持久话题的形态，归入 `event_chain`（事件链）/ `theme_vein`（主题脉络）/ `single_point`（单点影响）/ `sparse`（骨感）之一。判据 SHALL 综合 `hit_count`（丰满度）、`section` 数（聚合度）、`cluster_label` 发散度（线性 vs 平行）与内容语义。形态枚举 SHALL 可扩展，新增形态无需改架构。

#### Scenario: 高频线性因果归事件链型

- **WHEN** 话题 hit_count ≥ 10 且 section 时序呈线性因果演进（如"官宣→否认→条款"）
- **THEN** 系统 SHALL 判 `event_chain`，产出因果链 + 推演见解

#### Scenario: 单次命中归骨感型

- **WHEN** 话题 hit_count = 1（料严重不足）
- **THEN** 系统 SHALL 判 `sparse`，诚实标注信息不足，不产出推演见解

#### Scenario: AI 大主题归脉络型

- **WHEN** 话题 section 的 cluster_label 高度发散、无线性因果特征（如"产业范式转移"下多 AI 线索并行）
- **THEN** 系统 SHALL 判 `theme_vein`，产出平行线索 + 跨线索洞察

### Requirement: 分析视角候选与选择

系统 SHALL 采用「agent 提候选 + 用户选」模式（模式丙）：解读员 SHALL 输出多个**具体可讨论的视角候选**（问题式，如"美国为何反复横跳"，非"博弈论"等抽象标签），用户选定一个视角后，探索 agent 按该视角推演。第一版 SHALL 为单视角。视角来源 SHALL 抽象为可扩展接口（首批 agent 生成，预留外部源如视频评论员/研报）。

#### Scenario: agent 提具体视角候选

- **WHEN** 解读员完成形态判断
- **THEN** 系统 SHALL 输出 ≥ 2 个具体问题式视角候选，每个带视角名 + 一句话说明

#### Scenario: 用户选定视角驱动推演

- **WHEN** 用户从候选选一个视角（或自填）
- **THEN** 探索 agent SHALL 仅按该视角推演见解，不发散到未选视角

#### Scenario: 视角来源可扩展

- **WHEN** 未来接入外部视角源（如视频评论员）
- **THEN** 系统 SHALL 通过 LensSource 接口注入候选，不动核心编排

### Requirement: 探索 agent 工具集

系统 SHALL 向探索 agent 注册多级入口探索工具 + web_search 验证工具，替换纯金融工具（金融工具降为可选，仅金融话题注册）。多级入口工具 SHALL 支持分层下钻：`list_boards`（看版块全景）→ `list_lanes`（版块下泳道）→ `get_lane_detail`（泳道详情按需取）。agent SHALL 自主决定下钻深度与何时停止。

#### Scenario: 多级入口按需下钻

- **WHEN** 探索 agent 判断某版块可能与视角相关
- **THEN** agent SHALL 调 `list_lanes` 查该版块泳道，仅对相关泳道调 `get_lane_detail`，无关版块跳过

#### Scenario: web_search 双重用途

- **WHEN** 探索 agent 需验证事实节点或支撑推演中间环节
- **THEN** agent SHALL 调 `web_search`，结果纳入见解依据与确定性判断

#### Scenario: 金融工具按需注册

- **WHEN** 话题非金融形态
- **THEN** 系统 SHALL 不注册金融行情工具，agent 不可见

### Requirement: 分层见解产出

系统 SHALL 产出分层分析：**事实层**（梳理 + 验证，铺垫）+ **见解层**（推演/假设/提问/视角，★产出主体）。见解层 SHALL 发挥 AI 多层推演 + 跨领域联想 + 假设性提问优势。产出结构 SHALL 随形态变（事件链=因果链+推演见解 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 骨感=诚实标注）。

#### Scenario: 事实层与见解层分离

- **WHEN** 系统产出分析
- **THEN** 输出 SHALL 明确区分事实层（已验证 claim）与见解层（推演 insight），读者可辨哪是真哪是猜

#### Scenario: 见解随视角与形态变

- **WHEN** 同一话题换视角或换形态
- **THEN** 见解层产出 SHALL 不同（视角换→见解维度换；形态换→产出结构换）

### Requirement: 见解依据与确定性分级

每条见解 SHALL 挂**文章依据**（新闻原文引用）+ **时间线依据**（事件时序节点），不悬空发散。每条见解 SHALL 标注确定性分级：`high`（已验证）/ `medium`（推演·有据）/ `low`（假设·情景）/ `question`（提问·指出条件非预言成败）。见解 SHALL 给推演逻辑（凭什么 A→B），关键中间环节 SHALL 可 web 验证。

#### Scenario: 见解必须挂依据

- **WHEN** 探索 agent 产出一条见解
- **THEN** 该见解 SHALL 引用 ≥ 1 篇文章依据或时间线节点，无依据的见解 SHALL 被拒绝

#### Scenario: 确定性分级标注

- **WHEN** 见解为推演而非已验证事实
- **THEN** 系统 SHALL 标注 `medium`/`low`/`question` 之一，前端按级视觉区分

#### Scenario: 提问式见解不预言成败

- **WHEN** 见解为 `question` 级
- **THEN** 系统 SHALL 指出决定成败的条件，而非预言成败结果

### Requirement: 骨感型诚实标注

对料严重不足的话题（如 hit_count = 1，占库 65%），系统 SHALL 诚实标注"信息不足"，不强行推演见解。系统 SHALL 将此类话题列入观察，持续命中后自动升级分析。知道何时不该分析 SHALL 视为核心判断力。

#### Scenario: 骨感型不硬推演

- **WHEN** 话题被判 `sparse` 形态
- **THEN** 系统 SHALL 产出"信息不足"标注 + 轻量摘要，不产出推演见解

#### Scenario: 持续命中后升级

- **WHEN** 原 sparse 话题持续命中、积累演进特征
- **THEN** 系统 SHALL 重新判形态并升级为对应分析

### Requirement: 分析认知对比

系统 SHALL 在每次分析后对照上次，记录 `new_findings`（新见解）/ `overturned`（推翻的旧见解）/ `confidence_shift`（确定性变化），取代旧的"定位变化对比"。review SHALL 不回写新闻记忆（表1），仅标记认知采纳。

#### Scenario: 记录新发现与推翻

- **WHEN** 本次分析完成
- **THEN** review SHALL 对比上次 insight_layer，输出新增/推翻/确定性变化

#### Scenario: 不回写新闻记忆

- **WHEN** review 标记认知采纳
- **THEN** 系统 SHALL 仅在表3 记录，不污染表1（新闻记忆保持客观）

### Requirement: 多形态分析报告

前端 SHALL 按话题形态渲染不同结构的分析报告（事件链=因果链+见解层 / 脉络=平行线索+跨线索洞察 / 单点=影响评估 / 骨感=信息不足标注）。报告 SHALL 展示形态判断依据、视角选择、时间线依据轴、双类引用（📰新闻/🔧工具）、见解确定性分级视觉。

#### Scenario: 按形态渲染不同结构

- **WHEN** 话题形态为 `theme_vein`
- **THEN** 报告 SHALL 渲染平行线索 + 跨线索洞察，不画因果链箭头

#### Scenario: 见解确定性视觉区分

- **WHEN** 报告展示见解层
- **THEN** 不同确定性（high/medium/low/question）SHALL 用不同视觉（颜色/标签）区分

#### Scenario: 视角选择可交互

- **WHEN** 用户触发分析
- **THEN** 报告入口 SHALL 展示 agent 视角候选，用户可选/自填

### Requirement: 报告追问交互

系统 SHALL 支持用户在分析报告页针对报告内容多轮追问。追问 agent SHALL 复用探索 agent 循环（多级入口 + web_search），上下文 SHALL 带报告本身（analysis + 依据 + 视角 + 形态）。回答 SHALL 带双类引用 + 确定性标注。追问会话 SHALL 持久化（多轮历史）。追问新依据 SHALL 支持用户手动沉淀回报告（标记来源），不自动改报告。

#### Scenario: 追问复用探索工具

- **WHEN** 用户问"这条见解还有别的证据吗"
- **THEN** 追问 agent SHALL 调 web_search/get_lane_detail 补充，回答带新依据引用

#### Scenario: 多轮追问持久化

- **WHEN** 用户在同一报告下连续追问
- **THEN** 系统 SHALL 保存多轮问答历史（topic_enrichment_qa），支持回看

#### Scenario: 追问发现手动沉淀

- **WHEN** 追问中产生有价值的新依据
- **THEN** 系统 SHALL 支持用户手动沉淀回报告（标记 source=qa），不自动改报告
