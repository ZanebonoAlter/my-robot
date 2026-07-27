# Apply 进度跟踪 — daily-report-lane-driven-clustering

> 📍 **恢复点**：Wave 1 已验收✅；Wave 2 代码已落地⏸️ 但**主线程门禁核验被中断未完成**——恢复时先跑「Wave 2 节」里的门禁命令确认全绿，再做 §7.2 grep + Wave 3 文档 + 归档门禁。

> 主线程调度用（规范 §0.6 step 6）。子线程派发模型：核心逻辑用 `zai-coding-cn/glm-5.2`（省略 model 参数走默认）。

## 双源上下文（step 1，已完成）
- `doc-impact.sh suggest`：dirty worktree 噪声命中 architecture；本 change 真实域 = flow/database/standard/configuration（tasks.md §9 已声明，保留）。
- `doc-impact.sh context`：flow `daily-report.md` 约束 #2-#5（锚定双重确认 AND-gate / 三态 confidence / embedding 来源 / 快照不回填）即本 change 改写对象。

## 主线程定调的设计决策
1. **Vacuum 统计代理**：attracted = 近 vacuum_window 归属该 topic 的 section；strong/mid 用 `topic_match_distance` 阈值（迁移期）/ `lane_tier`（运行期）。只依赖已持久化 section 数据。
2. **分波策略**：Wave1=Foundation（加法式，独立可编译）；Wave2=Algorithm（消费 Wave1 API）；Wave3=Docs（spec 驱动，代码验证后跑）。
3. **UpdateSectionTopicAssignment 签名加 laneTier**：归 Wave2（与唯一调用方 assignment.go 同步改），保 Wave1 编译绿。

## Wave 1 — Foundation ✅ 已验收（主线程亲跑门禁全绿）
- [x] models.go 加字段（Centroid 用 `default:NULL` 偏离：GORM 对有 default tag 的零值字段 INSERT 时省略，DB 填 NULL——makeTopic 集成测试实证）
- [x] 迁移 20260727_0001：加列 + seed 6 ai_settings + centroid AVG 回填（non-fatal）+ vacuum COUNT FILTER 初始化
- [x] ComputeTopicCentroid / UpdateCentroidOnSectionChange / RecomputeVacuumStats + 纯 helper（meanPgVectors/computeVacuumFlag）
- [x] PersistentTopicConfig 加 6 字段 + Default(0.18/0.30/0.20/30/7/5) + Load
- 门禁：go build ✓ / vet ✓ / golangci-lint 0 issues ✓ / -short ✓ / 3 集成测试（迁移幂等+centroid window/退化+vacuum 识别）✓

## Wave 2 — Algorithm ⏸️ 实现已落地，主线程验收被打断（恢复点）

**⚠️ 恢复时第一件事：亲跑门禁确认（子线程报告全绿但主线程核验被用户中断，未完成）。**
```bash
cd backend-go
go build ./... && go vet ./internal/topicgraph/... && golangci-lint run ./internal/topicgraph/...
go test ./internal/topicgraph/... -short
# 集成（Docker pgvector 需 up）：
go test ./internal/topicgraph/... -run 'TestSaveReport_LaneDrivenCentroidRefresh|TestPlanTopicAssignments_LaneDriven|TestClusterTagsLane|TestBucketTagsByCentroid'
```

**数据流契约（service↔repository 缝合点，已实现）：**
- L1 tag（质心 dist<L1 且非 vacuum）→ 按 topic 聚合成 section（lane=l1_direct，不调 LLM）
- L2 tag（[L1,L2] 或 vacuum 降级）→ 每 tag 交 LLM keep/switch/new；keep/switch 并入目标 topic，new 转 L3
- L3 tag + L2-new tag → L3pool>2 时 LLM 起新叙事 group_name；≤2 各自成组不调 LLM
- section.lane_tier：挂现有 topic 且含 L1 tag→l1_direct；仅 L2 keep/switch→l2_llm；新建→l3_new

**已实现（子线程报告，待主线程核验）：**
- [x] 3.1 BucketTagsByCentroid（+9 单测）+ 3.2 吸尘器降级 + 3.3 ClusterTagsLane（L2/L3 拆分）
- [x] 4.1 L2 prompt（top-K 候选 + briefs，operation=daily_report.decide_l2_tags）+ 4.2 L3 复用旧 buildClusterSystemPrompt(nil)+buildClusterPrompt+parseClusterResponse
- [x] 4.3 target 校验 + off-shortlist 降级 new（parseL2Response + TestParseL2Response_SwitchOffShortlistDowngradesNew）
- [x] 5.1 planTopicAssignments 重构（读 sec.LaneTier+MatchedTopicID，移除 AND-gate）+ 5.2 orchestrator 改调 ClusterTagsLane + ListTagSemanticEmbeddings 加载 tag 向量 + 5.3 事后匹配已移除
- [x] UpdateSectionTopicAssignment 加 laneTier（+ manual_topic.go/backfill_topics.go 调用方同步）+ assignAndUpdateTopics 写 lane_tier + centroid 刷新/RecomputeVacuumStats
- [ ] 7.2 grep 不变量校验（恢复后做）

**子线程报告的 2 处偏离（恢复时核验合法性）：**
1. **未删旧 ClusterTags**：`cmd/verify-cluster-prompt/main.go:82` 仍调用它（导出符号，lint 不报 unused）。→ 合理，保留。
2. **新建 marshal_array.go 定义 marshalJSONArray**：前序 wave 留下孤儿 `marshal_array_test.go` 引用未定义符号阻塞 vet；已实现并接入 orchestrator 的 ClusterTagIDs（守护 nil→"null"→jsonb 22023 回归）。恢复时确认**无重复定义**（grep `func marshalJSONArray` 应只 1 处）。

**Wave 2 落地文件清单（恢复时 git diff 核对）：**
- 新建：service/daily_report_lane.go、service/marshal_array.go、service/daily_report_lane_test.go、service/daily_report_lane_pipeline_test.go、repository/daily_report_lane_integration_test.go、service/marshal_array_test.go
- 改：service/daily_report_orchestrator.go、repository/daily_report_assignment.go(+_test)、repository/daily_report_topic_repository.go、repository/daily_report_manual_topic.go、repository/daily_report_backfill_topics.go、repository/daily_report_models.go、repository/daily_report_repository.go、repository/daily_report_topic_lineage_test.go

## Wave 3 — Docs（代码验收后，可并行四域）
- [ ] 9.1 flow/daily-report.md 聚类节重写 + §12.2 变更溯源
- [ ] 9.2 database/ 补列 + 迁移记录
- [ ] 9.3 standard/ 聚类代码规约
- [ ] 9.4 configuration.md 补 6 阈值

## 归档门禁（§11，Wave 全部完成后）
- [ ] V.1-V.6 验证节实测
- [ ] check-standards.sh A-D 段
- [ ] doc-impact.sh verify
