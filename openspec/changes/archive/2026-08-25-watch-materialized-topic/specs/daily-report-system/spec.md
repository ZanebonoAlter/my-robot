## MODIFIED Requirements

### Requirement: 日报生成编排流水线

系统 SHALL 提供 `GenerateDailyReport(ctx, boardID, date)` 编排函数，按顺序执行：收集板内事件标签 → 质量筛选 → 去重 → LLM 分组(带组数限制) → 查询昨日日报 → 并行生成(Call A + C×K) → section 内容化 embedding 生成（文本来源见 section-content-embedding 能力，基于该 section 所聚 tag 的 label/description/代表文章摘录，而非 cluster_label 标题文本） → 同日 section 两阶段合并 → **Watch 物化追加**（keyword_topic / sentence_topic 物化轨按 watch-materialized-topic 能力产出追加 section；任一关注的物化失败 SHALL 降级跳过，SHALL NOT 阻断流水线） → section embedding 匹配写入关系表 → 组装 BoardDailyReport + DailyReportSection(含 best_tier/avg_score) → 存储。生成 SHALL 通过 goroutine 异步执行。

物化追加的 section SHALL NOT 参与同日合并，SHALL NOT 参与 section 关系计算。

流水线 SHALL NOT 执行 thread 级别的 tag 交集匹配或 prev_thread_id 赋值。

#### Scenario: 完整流水线执行

- **WHEN** 触发 SemanticBoard #5 在 2026-05-25 的日报生成
- **THEN** 系统 SHALL 按序执行：收集标签 → 质量筛选 → 去重 → LLM分组 → 查询昨日日报 → 并行生成 → 内容化 embedding 生成 → 同日合并 → Watch 物化追加 → section embedding 匹配写入关系表 → 组装存储 → status="done"

#### Scenario: 物化失败不阻断流水线

- **GIVEN** board #5 有一个 active 的 sentence_topic 关注
- **WHEN** 该关注的辅助标签检索在生成中失败
- **THEN** 系统 SHALL 跳过该关注的当期物化并记录日志，日报 SHALL 正常完成并保存，status SHALL NOT 为 failed

#### Scenario: 生成失败

- **WHEN** 流水线中任一步骤失败（如 LLM 调用超时）
- **THEN** 系统 SHALL 设置 status="failed"，保留已完成的中间结果（raw_clusters 等），WS 广播失败状态

### Requirement: section lane 归属标记

`daily_report_sections` SHALL 包含 `lane_tier` 列（取值 l1_direct / l2_llm / l3_new / watch_keyword / watch_sentence），标识该 section 的分桶来源，供前端展示与下游分析。lane_tier SHALL 在 section 生成时与 `topic_match_confidence` 一同确定并持久化。

watch_keyword section 的 `persistent_topic_id` SHALL 为空；watch_sentence section SHALL 归属其关注的专属持久话题（见 watch-materialized-topic 能力）。

#### Scenario: section 记录 lane 来源

- **GIVEN** 某 section 由 L1 直挂产生
- **WHEN** section 持久化
- **THEN** lane_tier SHALL 为 l1_direct，topic_match_confidence 为 anchor_hit

#### Scenario: 物化 section 记录物化来源

- **GIVEN** 关键字物化追加产出一个 section
- **WHEN** section 持久化
- **THEN** lane_tier SHALL 为 watch_keyword，persistent_topic_id SHALL 为空
