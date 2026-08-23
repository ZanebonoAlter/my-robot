## Why

日报持久话题出现"吸附黑洞"：candidate 话题「美军机从以色列境内基地起飞对伊朗实施打击」(id=976) 连续 8 天命中，但每日挂入的线索（阿联酋贸易、塞浦路斯、加沙空袭）与话题标题完全不相关。根因是 section embedding 从 `cluster_label` 文本生成，而 L1/L2 命中 section 的 cluster_label 又强制冻结为 topic label——导致：① 命中即回声（topic_match_distance 恒 ≈0.00002，无信息量）；② centroid（质心）永远停在首义标题文本上，不被真实内容拉动；③ 同域 tag 永远落在 L2 仲裁带被 keep，循环加深污染。section embedding 应代表 section 的**实际内容**而非标题文本。

## What Changes

- **section embedding 来源改为内容聚合**：从「cluster_label 文本嵌入」改为「当日该 section 所聚 tag 的 label+description+代表文章摘录」拼接文本嵌入（确定性，无 LLM 依赖），cluster_label 仅作空 tags 兜底。
- **新建候选话题首义向量随之内容化**：L3 新建 candidate 的 `embedding` 取该 section 内容 embedding（现有代码传递 `sec.Embedding`，随上游自动变化）。
- **质心动态化**：`ComputeTopicCentroid` 机制不变（近 N 条 section 均值），但输入变成内容 embedding 后，质心随实际内容漂移，挂错的内容会把质心拉离标题语义，后续无关 tag 距离自然出带。
- **扩展现有回刷端点**：`POST /api/daily-reports/backfill-embeddings` 增加 `recompute`/`board_id`/`since_days` 参数——重算模式按内容规则重算历史 section embedding + 受影响 topic 质心 + 重建关系；补缺模式口径同步统一为内容规则。
- `topic_match_distance`、thread `fit_distance`、同日合并、跨日关系匹配、前端 compose 搜索等所有消费方语义从「标题↔标题」变为「内容↔内容/内容↔标题」，判别力提升。

## Capabilities

### New Capabilities
- `section-content-embedding`: section embedding 的内容化生成——文本组装规则（tag label+description+代表文章，截断上限）、空 tags 兜底链、生成时机（threads 生成后、同日合并前）、历史回刷。

### Modified Capabilities
- `daily-report-system`: 「日报生成编排流水线」中 section embedding 生成步骤的输入从 cluster_label 文本改为 section 内容聚合文本。
- `section-relations`: 「关系写入逻辑」中"新 section 的 embedding 仍基于 cluster_label 文本生成"改为基于内容聚合文本。
- `persistent-topic`: 「PersistentTopic 质心表示」补充质心/首义向量锚定的语义为 section 实际内容（候选话题创建 scenario 的 embedding 来源随之为内容向量）。

## Impact

- **代码**：`backend-go/internal/topicgraph/service/daily_report_orchestrator.go`（embedding 文本组装替换）、新增 `buildSectionEmbedText` 纯函数（service 层，可单测）、`daily_report_handler.go` + `daily_report_repository.go`（回刷端点扩展）；`daily_report_merge.go` / `daily_report_thread_fit.go` / centroid 计算逻辑不动（消费方自动受益）。
- **数据**：无 schema 变更；历史 section embedding 需回刷才与新逻辑一致（不刷则新旧混用，质心窗口 30 条会自然换血）。
- **行为变化**：topic_match_distance / fit_distance 数值分布整体上移（不再有 ≈0 的回声距离）；同日合并与跨日关系的匹配基准从标题变为内容。
- **用户操作**：部署后建议对近期时间窗跑一次回刷；被污染的话题（如 976）建议手动归档让其重新聚类。
