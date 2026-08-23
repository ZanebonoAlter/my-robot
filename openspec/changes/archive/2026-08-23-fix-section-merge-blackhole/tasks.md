# Tasks: fix-section-merge-blackhole

<!-- doc-impact: flow configuration -->
<!-- doc-impact-excuse: api=并行会话 aggregate-tagging-resilience/tagmanagement 脏文件误报，本 change 未改 api; database=同上，未改模型/迁移; architecture=同上，未改架构层 -->

## 1. 配置开关

- [x] 1.1 `PersistentTopicConfig` 增加 `SectionMergeEnabled bool` 字段，`DefaultPersistentTopicConfig()` 默认 false；`LoadPersistentTopicConfig` 的 keys 列表与 switch 增加 `daily_report_section_merge_enabled`（`strconv.ParseBool`，解析失败 warning + 用默认）
- [x] 1.2 单测：无配置行 → false；"true" → true；非法值 → warning + false

## 2. 锚定边界过滤

- [x] 2.1 `daily_report_merge.go` 增加纯函数 `sameAnchorClass(a, b repository.DailyReportSection) bool`：`(a.MatchedTopicID == b.MatchedTopicID != nil) || (a.MatchedTopicID == nil && b.MatchedTopicID == nil)`
- [x] 2.2 `MergeSimilarSections` 在收集 deterministic/grayZone pairs 的双重循环内调用边界校验，不一致的 pair 跳过（不建边、不进灰区）
- [x] 2.3 单测覆盖五个场景：同 topic 距离 0.15 合并；不同 topic 距离 0.11 拒绝；NULL(l3)↔非 NULL 距离 0.14 拒绝；NULL↔NULL 距离 0.18 可合并；传递闭包不跨界（A(t7)↔B(t7)=0.15、B(t7)↔C(t12)=0.18 → 只合并 A,B）

## 3. 开关短路

- [x] 3.1 `MergeSimilarSections` 签名增加 `mergeEnabled bool`，函数开头 false 时直接返回原 sections/threadBatches，并记一条 `logging.Infof("daily-report: section merge disabled by config")`（每次生成一条，非每 pair）
- [x] 3.2 orchestrator `:337` 调用点传入 `topicCfg.SectionMergeEnabled`
- [x] 3.3 单测：`mergeEnabled=false` 时任何距离都不合并

## 4. Stage 1 审计日志

- [x] 4.1 deterministic 候选对（含被边界拒绝的对）在距离计算处记录一条日志：双方 `cluster_label`、`MatchedTopicID`、`lane_tier`、距离、结果（`merged` / `rejected-by-boundary` / `kept`）
- [x] 4.2 开关关闭时不产生逐 pair 日志（只有 3.1 的单条短路日志）

## 5. 验证与文档

- [x] 5.1 跑影响包测试：`go test ./internal/topicgraph/...`；`golangci-lint run ./...` + `go vet ./...` + `go build ./...`
- [x] 5.2 更新 `docs/reference/flow/daily-report.md` 业务约束节：同日合并受 `daily_report_section_merge_enabled` 控制且默认关闭；合并不得跨越锚定边界（引用本 change 溯源）
- [x] 5.3 归档前 `doc-impact.sh verify` + `check-standards.sh`；完工汇报包含部署后影响（下次 21:00 起 section 粒度变细、8-22 可手动重跑复原）与旧数据降级说明
