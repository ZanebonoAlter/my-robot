
## 观察期门禁改动锚点与红线

观察期门禁 change 的代码落点（探索阶段已验证）：

## 改动锚点
1. **门禁**：`backend-go/internal/topicgraph/service/daily_report_lane.go` `BucketTagsByCentroid`（~L130-190），L1 分支现为 `case nearestDist < cfg.LaneL1Threshold && nearestTopic != nil && !nearestTopic.IsVacuum:`，需加 `nearestTopic.Status == repository.TopicStatusActive && cfg.CandidateL1GateEnabled`；candidate 近距离自然落入下一 case `nearestDist <= cfg.LaneL2Threshold`（L2 分支，带 top-K 候选构造，无需新代码）。
2. **开关**：`backend-go/internal/topicgraph/repository/daily_report_topic_repository.go` `PersistentTopicConfig` 结构（~L30-80）+ `DefaultPersistentTopicConfig`（VacuumRatio 0.20 等默认值在此）+ `LoadPersistentTopicConfig`（~L190-210 有 `cfg.VacuumRatio = v` 式逐键解析）；新键 `persistent_topic_candidate_l1_gate_enabled` 默认 true。
3. **briefs**：`backend-go/internal/topicgraph/repository/daily_report_repository.go` `ListTopicRecentBriefs`（L1170-1268）——现状：仅查 `status=active` 话题，取 `ds.cluster_label AS section_label`，LATERAL 取 fit_distance 最小 2 条 thread title。改造：范围扩 `IN (active, candidate)`，section 内容改 `cluster_tag_ids`（JSON 数组列）join `topic_tags.label`（过滤 status='active'，每 section 截 5 个）；`TopicRecentBrief` 结构体字段同步。
4. **prompt**：`daily_report_lane.go` `buildL2Prompt`（L428+）system 加观察期从严段（design D5 措辞）；user 侧候选行 `近期 section 框架:` 块改渲染 tag 标签。`daily_report_cluster.go` `buildClusterSystemPrompt` Slice D 渲染块（`近期实际内容:`）同步适配。
5. **降级契约**：orchestrator Step 3.5 注释明确 briefs 失败降 label-only 不阻塞（daily_report_orchestrator.go ~L60-85），测试已有该模式。

## 历史红线（勿踩）
- 2026-08-19-daily-report-prompt-hygiene：L2 prompt 禁注入历史 thread 文案。本次注入的 tag label 是当天事实指纹，不触红线；严禁顺手恢复 thread title 注入。
- label 硬覆盖链：orchestrator `clusterLabel = topicLabelByID[*cluster.MatchedTopicID]`（~L262）导致 cluster_label 零信息——这是 briefs 必须改数据源的原因，勿试图改覆盖行为（前端依赖）。

## 测试文件
- `daily_report_lane_test.go`（分桶/lane 管线）、`daily_report_repository_test.go`（ListTopicRecentBriefs 现有 5 个用例在 L358-530，seed helper：seedTestActiveTopic/seedTestSectionWithTopic/seedTestThread）

## DB 实测基线（2026-08-24）
- candidate #1032「伊朗议长卡利巴夫透露哈梅内伊殉难后的凌晨应急决断」：born 08-01（单文章 section 出生），10 命中/24 天，近期 l1_direct dist 0.077-0.139——上线后应转为 l2_llm。
- 全库 candidate 800 个，其中 hit<3 且近 7 天有命中的隐藏苟活 52 个——门禁后预期自然失血退场（7 天窗口滑出）。

<!-- pinned 2026-08-24T15:22:02Z -->
