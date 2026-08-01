# Tasks — daily-report-lane-driven-clustering

> apply 启动跑 `bash scripts/doc-impact.sh suggest` 预勾选 + `bash scripts/doc-impact.sh context` 双源注入（规范 §0.6 步骤1）。

## 1. 数据层迁移（规范 §10 显式迁移）

- [x] 1.1 `board_persistent_topics` 加 `centroid` vector(2560) NULL、`is_vacuum` bool default false、`vacuum_strong` int default 0、`vacuum_mid` int default 0
- [x] 1.2 `daily_report_sections` 加 `lane_tier` varchar(16) NULL
- [x] 1.3 离线迁移：按历史 section embedding 构建每个 topic 的 centroid（centroid_window=30）；section<2 用首义向量
- [x] 1.4 离线迁移：按近 7 天吸引统计初始化 `is_vacuum` + `vacuum_strong`/`vacuum_mid`
- [x] 1.5 迁移幂等（testcontainer 2 次重跑验证）

## 2. 质心计算与吸尘器（daily_report_topic_repository.go）

- [x] 2.1 `ComputeTopicCentroid(topicID)`：按 centroid_window 加权平均 section embedding，section<2 退化首义
- [x] 2.2 `UpdateCentroidOnSectionChange(topicID)`：section 新增/归属变更时增量重算
- [x] 2.3 `RecomputeVacuumStats(boardID)`：按 vacuum_window 算 attracted/strong/mid，更新 is_vacuum + vacuum_strong/vacuum_mid
- [x] 2.4 `ListAnchorableTopicsByBoard` 返回 centroid + is_vacuum（替换现有首义向量 embedding 用法）

## 3. 三层分桶（daily_report_cluster.go）

- [x] 3.1 `BucketTagsByCentroid(tags, topics) → {L1, L2, L3}`：每个 tag 算到 topic centroid 的 argmin 距离，按 lane_l1_threshold/lane_l2_threshold 分桶
- [x] 3.2 吸尘器降级：挂到 `is_vacuum=true` topic 的 tag 从 L1 移到 L2
- [x] 3.3 `ClusterTags` 拆 L2/L3 分支（替换全量自由聚类）；`len(去重后 tags)<=2` 兜底保留

## 4. L2/L3 LLM（daily_report_cluster.go）

- [x] 4.1 L2 prompt：注入 top-K 候选 topic（按距离）+ 近期 section 摘要，输出 keep/switch/new + target_topic_id
- [x] 4.2 L3 prompt：为无法归属 tag 起新叙事 group_name
- [x] 4.3 target_topic_id 校验（候选集内），非法降级 new，section 元数据标注 `llm_target_off_shortlist=true`

## 5. 归属与编排（daily_report_assignment.go + daily_report_orchestrator.go）

- [x] 5.1 `planTopicAssignments` 重构：按 L1/L2/L3 结果设 lane_tier + topic_match_confidence（L1 / L2-keep / L2-switch=anchor_hit；L3=auto_new）
- [x] 5.2 `GenerateDailyReport` 管线顺序：质心加载 → 分桶 → L2/L3 LLM → 组装 section（天生挂 topic）→ SaveReport → 增量更新 centroid/vacuum
- [x] 5.3 移除事后 section 标题↔topic 匹配环节（形态4根源）

## 6. 配置

- [x] 6.1 ai_settings 新增：`lane_l1_threshold`(0.18) / `lane_l2_threshold`(0.30) / `vacuum_ratio`(0.20) / `centroid_window`(30) / `vacuum_window`(7) / `l2_candidate_k`(5)
- [x] 6.2 阈值缺失降级到默认值

## 7. Review 与兼容性

- [ ] 7.1 人工聚焦 review（§0.6 步骤4）：质心 SQL 参数化、增量更新并发、L2 prompt 候选注入、迁移幂等
- [x] 7.2 grep 不变量：`grep -rn 'lane_tier\|centroid\|is_vacuum' backend-go/internal/topicgraph/` 确认归属走新分桶；`grep -rn 'matched_topic_id' backend-go/internal/topicgraph/service/daily_report_cluster.go` 确认旧自由聚类路径已收窄到 L2/L3
- [x] 7.3 兼容性：历史 section 归属不回刷（lane_tier 为 NULL 视为历史数据）；新日报走新流程

## 8. 测试（§11.2）

> 归档前重跑，零失败。后端 go 命令走 cmd.exe。

- [x] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/... -short"` → PASS
- [x] T.2 testcontainer 集成：迁移幂等（2 次重跑）+ centroid 构建正确性 + 分桶 L1/L2/L3 边界 + 吸尘器降级 + L2 三选一（keep/switch/new + off-shortlist 降级）→ PASS
- [x] T.3 质心增量更新：section 新增后 centroid 变化、窗口溢出淘汰 → PASS

## 9. 文档

<!-- doc-impact: flow, database, standard, configuration -->

- [x] 9.1 `docs/reference/flow/daily-report.md`：聚类节重写（三层分桶 + 质心 + 吸尘器 + LLM 退化），archive 后按 §12.2 补「变更溯源」链接
- [x] 9.2 `docs/reference/database/`：补 board_persistent_topics 的 centroid/is_vacuum/vacuum_strong/vacuum_mid 列 + daily_report_sections.lane_tier 列 + 迁移记录
- [x] 9.3 `docs/reference/standard/`：聚类代码规约（分桶/质心/吸尘器约束，按 §0.6 how 段注入）
- [x] 9.4 `docs/reference/configuration.md`：补 lane_l1_threshold / lane_l2_threshold / vacuum_ratio / centroid_window / vacuum_window / l2_candidate_k

## 10. 验证（§11.2，归档前实测）

- [x] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [x] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."` → VET_OK
- [x] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/..."` → 0 issues
- [x] V.4 `bash scripts/check-standards.sh` → A-D 段零失败
- [ ] V.5 SQL 复算：对最近 7 天日报用新分桶逻辑重算 lane 分布，确认 L1 占比 ~47%、L3 ~1.3%，抽样无万能包装标题
- [ ] V.6 功能验收（cmd 起后端）：生成一次日报，确认 ① L1 section `lane_tier=l1_direct` 且无 LLM 调用日志 ② L2 section 有 LLM decision 记录 ③ 吸尘器 topic 的 tag 进 L2 ④ 无事后 section↔topic 匹配
