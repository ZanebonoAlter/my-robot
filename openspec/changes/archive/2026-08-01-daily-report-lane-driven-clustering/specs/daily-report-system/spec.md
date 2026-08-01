## MODIFIED Requirements

### Requirement: 聚类数限制

日报聚类 SHALL 采用「embedding 质心先分桶 → LLM 弱区裁决/兜底」流程，取代 LLM 对全部当天 tag 自由聚类：

1. 对去重 + 质量筛选后的当天 event tag，按 PersistentTopic 质心做最近邻分桶：L1（< `lane_l1_threshold`）/ L2（[`lane_l1_threshold`, `lane_l2_threshold`]）/ L3（> `lane_l2_threshold`）。
2. L1 tag 按 topic 直接成组（同 topic 的 L1 tag 合并为该 topic 当日 section 候选），不调用 LLM。
3. L2 tag 交 `ClusterTags` 在 embedding 预筛的 top-K 候选 topic 上做「留/换/新」三选一。
4. L3 tag 交 `ClusterTags` 起新叙事成组。
5. section 天生挂 topic（L1 直挂 / L2 LLM 挂 / L3 新建），无事后 section 标题↔topic 匹配环节。

`len(去重后 tags) <= 2` 或 L2+L3 tag 合计不足以成组时，SHALL 跳过 LLM，每个 tag 独立成组（沿用现有兜底）。

#### Scenario: 三层分桶聚类

- **GIVEN** board 当天 40 个 event tag，质心分桶后 L1=18 / L2=20 / L3=2
- **WHEN** 系统执行聚类
- **THEN** L1 的 18 个 tag 按 topic 直接成组（不调 LLM），L2 的 20 个交 LLM 三选一，L3 的 2 个交 LLM 起新叙事

#### Scenario: 极少 tag 跳过 LLM

- **GIVEN** board 当天去重后仅 2 个 event tag
- **WHEN** 系统聚类
- **THEN** 系统 SHALL 跳过 LLM，每个 tag 独立成组（沿用 `len<=2` 兜底）

## ADDED Requirements

### Requirement: section lane 归属标记

`daily_report_sections` SHALL 新增 `lane_tier` 列（取值 l1_direct / l2_llm / l3_new），标识该 section 的分桶来源，供前端展示与下游分析。lane_tier SHALL 在 section 生成时与 `topic_match_confidence` 一同确定并持久化。

#### Scenario: section 记录 lane 来源

- **GIVEN** 某 section 由 L1 直挂产生
- **WHEN** section 持久化
- **THEN** lane_tier SHALL 为 l1_direct，topic_match_confidence 为 anchor_hit
