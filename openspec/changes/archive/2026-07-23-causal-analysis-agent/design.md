## Context

`data-enrichment-orchestration` 的演进定位主线跑下来像新闻总结。数据库调研（2026-07-20，538 持久话题）推翻了"演进定位/因果传导"的普适假设：

- **主流是 AI/技术话题**（产业范式转移、Agent架构…），形态是「大主题下平行线索」，无因果链。
- **65% 话题只命中 1 次**，料严重不足。
- **形态天差地别**（地缘=线性因果 / AI=平行脉络 / 技术=单点单簇），没有一种固定框架能套所有话题。

故本 change 把分析目标从「演进定位」改为「**探索判断 agent——形态随话题变 + 见解为核心**」。核心公式：**见解 = 事实 × 视角**。事实梳理只是铺垫，见解（推演/假设/提问/视角，挂文章+时间线依据）才是产出主体，发挥 AI 多层推演+跨领域联想优势。

**复用骨架**（data-enrichment-orchestration 已交付，不动）：三表分离（记忆/快照/反思）、agent loop 三防御（去重/不截断/thinking 关闭）、可观测性（Operation/SessionID/input_snapshot/tool_calls）、循环A（新闻背景独立产物）、tool registry 骨架。

**取代部分**：演进定位主线（position 四档/signals）、定位变化对比 review、前端演进报告。

## Goals / Non-Goals

**Goals:**

- 形态判断：agent 按话题元数据（hit_count/section 数/cluster_label 发散度）判四形态（事件链/主题脉络/单点影响/骨感），形态可扩展。
- 视角机制（模式丙）：agent 提候选**具体视角**（可讨论的问题，非抽象标签）→ 用户选 → 按视角推演。视角来源可扩展（agent 生成 + 预留外部源接口）。
- 见解层：分层（事实层验证 + 见解层推演），确定性分级（已验证·高/推演·中/假设·低/提问），每条见解挂**文章依据 + 时间线依据**。
- 探索 agent 循环：多级入口工具（list_boards/list_lanes/get_lane_detail）+ web_search，边查边想边记，按形态/视角控深度。
- 编排：解读员（形态判断 + 视角候选）+ 探索 agent 循环（方案乙）。
- review 重定义：分析新发现/推翻对比（替定位变化）。
- 报告追问交互：复用探索 agent 循环，多轮对话，带报告上下文，可手动沉淀追问发现。
- 诚实姿态：骨感型诚实标注信息不足（不硬推演）；见解层敢推演但必须挂依据。

**Non-Goals:**

- 视频语料/评论员视角源（**后续独立 change**，本 change 仅预留接口）。
- 多视角交叉（第一版单视角先验证效果）。
- 跨版块静态话题关联建模（用 agent 动态探索绕开，对应 data-enrichment §11.1.1 暂缓）。
- 自动触发（手动触发，不挂日报管线）。
- FinGenius 个股辩论（冻结，独立于主线）。
- 涨跌预测/兑现打分（演进定位已删，因果归因不做笃定结论）。

## Decisions

### D1. 编排：解读员（形态判断+视角候选）+ 探索 agent 循环

```
话题入口
  → [解读员] 全层读 context+14天详情 → 输出 {形态判断, 视角候选[], 探索方向}
  → [用户选视角]（模式丙，UI 一步）
  → [探索 agent 循环] 多级入口翻版块/泳道 + web_search，按选定视角推演见解挂依据
  → [产出] {form, lens, analysis(事实层+见解层)}
  → [review] 对照上次，记新发现/推翻
```

**选择理由**：解读员给"起点"（省 agent 从零瞎摸）+ 做形态判断/视角候选（这两步需要全层 context，放探索 agent 里会重复消耗）；探索 agent 拿工具自主跑（多级下钻+web 验证），是分析主引擎。

**备选**：① 三角色流水线（旧，解读→查询→分析）——查询员只查金融、分析员被动等数据，与"探索+见解"目标冲突。② 单 agent 全自动（解读员也融进去）——全层 context 在 agent loop 里每轮重复消耗 2.5k token，6 轮就贵；且形态判断需要全局视角，适合一次性给而非循环里算。

### D2. 形态判断：LLM + 元数据，四形态枚举可扩展

解读员输出 `form ∈ {event_chain, theme_vein, single_point, sparse}`，判据：`hit_count`（丰满度）+ `section 数`（聚合度）+ `cluster_label 发散度`（线性 vs 平行）+ 内容语义。

**选择理由**：纯规则判不准（cluster_label 发散度需语义理解）；纯 LLM 无元数据约束易飘。元数据做硬约束（如 hit=1 直接 sparse）+ LLM 做语义判断（线性因果 vs 平行线索），分层最稳。

**可扩展**：form 是枚举，未来加新形态（如周期型/争议型）只需扩枚举 + 对应 analysis 结构 + 前端渲染分支，不动架构。

**备选**：固定四形态硬编码——不可扩展，违背"形态随话题变"的本质。

### D3. 视角机制：模式丙（agent 候选 + 用户选），视角来源可扩展

解读员输出 `lens_candidates[]`（具体可讨论的问题，如"美国为何反复横跳"，非"博弈论"标签）。用户选一个 `lens`。探索 agent 按 `lens` 推演。

**视角来源抽象**（★预留扩展）：

```
type LensSource interface { Propose(topic, form) []Lens }
// 首批实现：AgentLensSource（LLM 生成候选）
// 预留：VideoCommentatorLensSource（视频评论员视角，后续 change）
//       ReportLensSource（研报视角）/ BlogLensSource ...
```

**选择理由（模式丙 vs 甲/乙）**：甲（全自动）视角同质化、常不命中用户关心点；乙（用户指定）门槛高，用户不知有啥视角；丙让 AI 发挥"列专业视角"优势（知识盲区）+ 用户发挥"知关心啥"优势（AI 猜不准），agent 给候选降门槛。

**视角必须具体（问题式）**：抽象标签（"博弈论""风投"）对推演无约束力；具体问题（"美国为何反复横跳"）直接驱动推演方向。

**备选**：模式甲（全自动）/ 乙（用户指定）——见上理由否决。

### D4. 见解层：分层 + 确定性分级 + 文章/时间线依据

```
analysis = {
  fact_layer:   [{claim, evidence:[文章ref], verified:bool}],   // 事实层·验证
  timeline:     [{date, event, ref}],                            // 时间线依据
  insight_layer:[{                                               // 见解层·推演（产出主体）
    cert: "high|medium|low|question",
    title, logic,                                                // 推演逻辑（凭什么 A→B）
    evidence:[文章ref + 时间线节点],                              // 必须挂依据，不悬空
    web_verified:[tool_ref]                                      // 可选：web 验证的中间环节
  }]
}
```

**选择理由**：分层把"铁事实"和"AI 推演"区隔——事实层严谨验证（防编），见解层敢推演（发挥 AI 优势）但每条挂依据+标确定性（防瞎说）。这是"敢推演"与"诚实"的调和：不是不判因果，是判因果要分层标注。

**确定性分级**：high（已验证）/ medium（推演·有据）/ low（假设·情景）/ question（提问·指出条件非预言成败）。提问式见解（"路线图能否打破周期？"）是 AI 优势——指出别人没想的可能性，而非硬下断言。

**备选**：混合不分层（旧因果链）——事实和推演混一起，读者分不清哪是真哪是猜，且 LLM 易把推演当事实陈述。

### D5. 工具集：多级入口 + web_search，替换纯金融

```
list_boards()                 → [{id,name,活跃度}]           // 看全景
list_lanes(board_id)          → [{lane_id,话题名,状态,活跃度}] // 版块下泳道
get_lane_detail(lane_id, win) → 泳道详情（14天/历史）          // 按需下钻
web_search(query)             → 网页结果                      // 验证事实 + 支撑推演中间环节
```

**选择理由**：多级入口让 agent 像研究者翻目录，按需下钻（省 token、像人探索），用**动态探索顶掉静态跨版块建模**（绕开 data-enrichment §11.1.1 暂缓的跨版块关联机制——不预先建关系，agent 分析时自己发现关联）。web_search 双用：验证事实节点 + 支撑推演（查日韩能源依赖度，让"输入性通胀"推演有据）。

**金融工具降级**：list_etf/get_etf/list_sectors 降为可选（仅金融话题），不再默认注册。

**备选**：一次性灌全 context（旧）——token 浪费、不像探索、锁死单版块视野。

### D6. 探索 agent：agent loop 作分析主引擎

复用 data-enrichment 的 agent loop 三防御（去重/不截断/thinking 关闭），扩展为分析主引擎：每轮 agent 自主决定调哪个工具（多级入口/web_search）、查多深、何时停。**按形态控深度**：事件链深挖（推演连锁）、骨感型浅出（诚实标注即停）。**按视角聚焦**：只推演与选定视角相关的链。

**选择理由**：分析本质是探索性、开放 ended 的（查到啥决定下一步查啥），固定流程套不住。agent loop 的自主性正是"发挥 AI 优势"的落点。

### D7. review：新发现/推翻对比（替定位变化）

演进定位删除后，原"prev.position vs curr.position"无意义。review 改为对照上次 `analysis.insight_layer` vs 本次，记 `new_findings[]`（新见解）/ `overturned[]`（推翻的旧见解）/ `confidence_shift[]`（确定性变化）。仍不回写新闻记忆（表1）。

**选择理由**：见解是产出主体，review 该跟踪见解的演进（这次比上次多了啥认知/推翻了啥），而非已删除的定位标签。

### D8. 数据模型：result {form, lens, analysis}，analysis 多态

`topic_enrichment_result.Sectors` jsonb（免 DDL，复用列）存复合对象：

```
{ form, lens, analysis, tool_calls, input_snapshot }
// analysis 按 form 多态：event_chain={fact,timeline,insight} / theme_vein={veins,cross_insight} / ...
```

**选择理由**：免 DDL（复用旧 jsonb 列），analysis 按 form 多态适配"形态随话题变"。旧演进定位数据（position/signals）不可复用，清空重跑。

### D9. 报告追问：复用探索 agent + 报告上下文

报告产出后用户多轮追问。追问 agent **复用 D6 探索 agent 循环**（三防御 + 多级入口 + web_search），输入从「视角」换「用户问题」，上下文增加报告本身（analysis + 依据 + 视角 + 形态）。

```
用户问题 + 报告上下文 → [探索 agent 循环] → 回答（双类引用 + 确定性）
                                      ↓（可选）
                               用户手动沉淀新依据回报告
```

**选择理由**：问答复用探索 agent（同构，不引入新架构）；报告上下文让回答紧扣报告；工具调用让追问能"再探索"（非干巴巴复述）。

**沉淀**：追问新依据手动沉淀（source=qa），不自动改报告——保报告"一次性产出"语义。

**会话存储**：新表 `topic_enrichment_qa`（result_id, question, answer, tool_calls, created_at），多轮 append-only，不回写 result（报告不可变）。

**备选**：① 纯复述报告（不调工具）——答疑弱，"还有证据吗"答不了。② 自动更新报告——破坏不可变语义，追问发现未必可靠。

## Risks / Trade-offs

- **[见解质量·LLM 推演跑偏]** → 分层标注确定性 + 每条挂文章/时间线依据 + web 验证中间环节 + 提问式见解（指出条件非预言成败）。低确定性见解视觉区分（琥珀色），用户自知是假设。
- **[形态判断不准]** → 元数据硬约束（hit=1 直 sparse）+ LLM 语义判断；前端展示判据（用户可纠正）；形态可人工 override（后续）。
- **[视角候选不命中用户]** → 模式丙给 3 候选 + 用户可自填；agent 候选基于话题特征，命中率应高于全自动。
- **[成本浮动]** → 探索深度随形态（骨感浅出省成本、事件链深挖）；解读员全层读仅 1 次；max_loops 上限兜底。
- **[视频语料不在本 change]** → D3 视角来源抽象预留接口（LensSource），后续 change 接入 VideoCommentatorLensSource，不动核心。
- **[单视角局限]** → 第一版只按一个视角推演，可能漏其他维度。Non-Goal（先验证效果）；多视角交叉留未来。
- **[web_search 结果质量]** → 工具层隔离，单次失败返回 error 不阻断；agent 自判结果可信度（纳入见解确定性）。
- **[追问发散/成本]** → 报告上下文约束聚焦；max_loops 上限；纯答疑为主（不自动改报告）；沉淀需用户手动确认。

## Migration Plan

1. 部署前：清空 `topic_enrichment_result` / `topic_enrichment_review` 旧数据（演进定位语义，不可复用）——迁移脚本 TRUNCATE + RESTART IDENTITY。
2. 部署：新 orchestrator（解读员+探索 agent）、tool_registry（多级入口+web_search）、review_judge（新发现对比）、前端组件上线。
3. 回滚：旧 data-enrichment-orchestration 代码保留（git 历史），回滚即恢复演进定位（但清空的数据需重跑）。
4. 循环A（新闻背景）独立产物，不受影响。

## Open Questions

- **视频语料视角源**（后续 change）：字幕获取（ASR/API）、评论员筛选、合规边界——独立 change 探索。
- **多视角交叉**（未来）：单视角验证效果后，是否进化为多视角综合推演（成本/深度权衡）。
- **形态判断的人工 override**：agent 判错形态时，用户能否手动指定（第一版不做，看反馈）。
- **见解质量的人工确认**：低确定性见解是否需用户点头才展示（第一版全展示+标注，看反馈）。
