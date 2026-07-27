## MODIFIED Requirements

### Requirement: DailyReportSection 强制归属 PersistentTopic

`daily_report_sections` SHALL 包含 `persistent_topic_id`（外键到 board_persistent_topics）、`topic_match_distance`、`topic_match_confidence`、`lane_tier` 四列。`topic_match_confidence` SHALL 取值 anchor_hit / auto_new / unmatched / manual。`lane_tier` SHALL 取值 l1_direct / l2_llm / l3_new，标识该 section 的分桶来源。

归属算法 SHALL 保证每个新生成的 section 归属到恰好 1 个 PersistentTopic，采用三层分桶：

- **L1 直挂（lane_tier=l1_direct）**：当天 tag 经 embedding 质心匹配，到某 active/candidate topic 质心距离 < `lane_l1_threshold`（默认 0.18）时，section 直接归属该 topic，topic_match_confidence=anchor_hit，不调用 LLM。
- **L2 LLM 裁决（lane_tier=l2_llm）**：质心距离在 [`lane_l1_threshold`, `lane_l2_threshold`]（默认 [0.18, 0.30]）的 tag，系统 SHALL 将其与 embedding 预筛的 top-K 候选 topic（默认 K=5）一并交 LLM 做「留/换/新」三选一；LLM 选「留」归属候选 topic（confidence=anchor_hit），选「换」归属 LLM 指定 topic（须校验在候选集内，confidence=anchor_hit），选「新」转 L3。
- **L3 新建（lane_tier=l3_new）**：质心距离 > `lane_l2_threshold` 或 L2 LLM 选「新」的 tag，系统 SHALL 新建 candidate topic 归属，topic_match_confidence=auto_new。

embedding 为空时 confidence=unmatched、lane_tier 允许为空、persistent_topic_id 允许为 NULL（仅此例外）。`persistent_topic_id` 列 SHALL NOT 加 NOT NULL 约束。`topic_match_confidence=manual` 表示用户手动改写（非算法判定），lane_tier 保持改写前的值，`topic_match_distance` 为该 section embedding 到所属话题质心的距离。

归属判定 SHALL 在 section 生成阶段完成（section 天生挂 topic），不再有事后 section 标题↔topic 匹配环节。

#### Scenario: L1 直挂命中（质心强匹配）

- **GIVEN** board 有 active topic「Claude Code 工具链崛起」，当天 tag「Claude Code 模型选型」到该 topic 质心距离 0.12
- **WHEN** 系统执行三层分桶
- **THEN** 该 tag 所属 section SHALL 归属该 topic，lane_tier=l1_direct，topic_match_confidence=anchor_hit，不调用 LLM

#### Scenario: L2 弱区 LLM 裁决留/换/新

- **GIVEN** 当天 tag「特斯拉工业扩张」到最近 topic 质心距离 0.25（落在 L2 弱区）
- **WHEN** 系统将 tag 与 top-K 候选 topic 交 LLM
- **THEN** LLM SHALL 输出留/换/新三选一；选「留」归属候选 topic（anchor_hit），选「换」归属指定 topic（须在候选集内），选「新」转 L3

#### Scenario: L3 新建候选（质心挂不上）

- **GIVEN** 当天 tag「商务部出口管制清单」到所有 topic 质心距离均 > 0.30
- **WHEN** 系统执行三层分桶
- **THEN** 系统 SHALL 新建 candidate topic 归属，lane_tier=l3_new，topic_match_confidence=auto_new

#### Scenario: L2 LLM 指定 topic 超出候选集降级

- **WHEN** L2 裁决中 LLM 输出「换」且目标 topic 不在预筛候选集
- **THEN** 系统 SHALL 将其视为「新」，转 L3 新建 candidate，并在 section 元数据标注 `llm_target_off_shortlist=true`

#### Scenario: embedding 为空标记未匹配

- **WHEN** section embedding 为空
- **THEN** 系统 SHALL 设置 topic_match_confidence=unmatched，lane_tier 为空，persistent_topic_id 保持 NULL

#### Scenario: 手动改写归属

- **GIVEN** section #123 的 persistent_topic_id=8（lane_tier=l1_direct, confidence=anchor_hit）
- **WHEN** 用户在编排态将 section #123 串联进手动新建的 topic #20
- **THEN** 系统 SHALL 将 persistent_topic_id 改写为 20，topic_match_confidence=manual，lane_tier 保持 l1_direct，topic_match_distance=该 section embedding 到 #20 质心的距离

### Requirement: ClusterTags 注入历史叙事框架（职责收窄）

`ClusterTags` SHALL 仅处理 L2 弱区 tag 与 L3 新叙事 tag，不再对全部当天 tag 做自由聚类。

- 对 **L2 tag**，`ClusterTags` SHALL 查询该 board 下所有 active 与 candidate 状态 PersistentTopic 的质心，embedding 预筛 top-K 候选 topic（连同其近期 section 摘要）注入 LLM prompt，指示 LLM 在候选集内做「留/换/新」三选一。
- 对 **L3 tag**，`ClusterTags` SHALL 指示 LLM 为无法归属的 tag 起新叙事标题（开新组）。

LLM 输出 schema：L2 group 含 `decision`(keep/switch/new) + `target_topic_id`(keep/switch 时必填)；L3 group 含 `group_name`。`ClusterTags` SHALL 校验 `target_topic_id` 存在于传入候选集，非法值降级为 new。

#### Scenario: L2 候选预筛注入

- **GIVEN** L2 弱区 tag「OpenAI 模型入侵 HuggingFace」，board 有 active topic「大模型监管与安全」
- **WHEN** 系统构建 L2 LLM prompt
- **THEN** prompt SHALL 含 top-K 候选 topic（含「大模型监管与安全」）及其近期 section 摘要，要求 LLM 输出 keep/switch/new

#### Scenario: L3 新叙事起标题

- **GIVEN** L3 tag 集合无法归属任何现有 topic
- **WHEN** 系统执行 ClusterTags L3 分支
- **THEN** LLM SHALL 为其起新叙事标题，系统据此新建 candidate topic

## ADDED Requirements

### Requirement: PersistentTopic 质心表示

PersistentTopic 的 embedding 匹配锚点 SHALL 为「历史 section embedding 质心」，而非首条 section 标题继承的首义向量。质心 SHALL 按 `centroid_window`（默认最近 30 条 section，ai_settings 可配）加权计算，section 新增或归属变更时增量更新。质心 SHALL 物化存储于 `board_persistent_topics.centroid` 列（与 embedding 同维度），首次部署按历史 section embedding 离线构建一次。

历史 section 数 < 2 的 topic，质心退化为首义向量（等价于现有 embedding 列语义）。

#### Scenario: 质心按近期 section 计算

- **GIVEN** topic「Claude Code 工具链崛起」有历史 section 40 条
- **WHEN** 系统计算其质心
- **THEN** 系统 SHALL 取最近 30 条 section embedding 加权平均作为 centroid，centroid_window 之外的 section 不参与

#### Scenario: section 不足退化首义向量

- **GIVEN** topic 只有 1 条历史 section
- **WHEN** 系统计算其质心
- **THEN** centroid SHALL 等于该 section 的 embedding（首义向量语义）

#### Scenario: 质心增量更新

- **GIVEN** 新 section 归属 topic「X」
- **WHEN** section 持久化
- **THEN** topic X 的 centroid SHALL 增量重算（纳入新 section，窗口溢出的旧 section 淘汰）

### Requirement: 吸尘器 topic 检测

系统 SHALL 对每个 active/candidate topic 维护「吸引统计」：近 `vacuum_window`（默认 7 天）内以该 topic 为质心最近邻的当天 tag 数（attracted），其中质心距离 < `lane_l1_threshold` 的计为 strong、落在 [`lane_l1_threshold`, `lane_l2_threshold`] 的计为 mid。`strong/(strong+mid) < vacuum_ratio`（默认 0.20）的 topic 标记 `is_vacuum=true`。

挂到 `is_vacuum=true` topic 的 tag SHALL 降级到 L2（即使质心距离 < `lane_l1_threshold`），交 LLM 裁决，避免过宽质心误吸。

#### Scenario: 吸尘器 topic 识别

- **GIVEN** topic「中国中央银行相关新闻」近 7 天 attracted=17、strong=0、mid=11
- **WHEN** 系统计算 vacuum 标记
- **THEN** strong/(strong+mid)=0 < 0.20，系统 SHALL 标记 is_vacuum=true

#### Scenario: 吸尘器 topic 的强挂 tag 降级

- **GIVEN** topic「中国央行新闻」is_vacuum=true，当天 tag「WAIC 主旨讲话」到其质心距离 0.15（本应 L1 直挂）
- **WHEN** 系统分桶
- **THEN** 该 tag SHALL 降级到 L2 交 LLM 裁决，不直接 L1 直挂
