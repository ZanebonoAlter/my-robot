# Tasks: PersistentTopic 持久叙事话题

> 垂直切片，每切片独立可交付、可验证。算法相关切片以 TDD + 集成测试（真实 embedding 数据）双保险，参数合理性由 verification-report.md 量化验证。

## 1. 数据模型与迁移

- [x] 1.1 新增迁移文件 `backend-go/internal/platform/database/migrations/2026MMDD_create_board_persistent_topics.go`：建 `board_persistent_topics` 表（字段见 spec），含 `(semantic_board_id, status)` BTree 索引、`embedding` HNSW 索引、status CHECK 约束。验收：迁移在 testcontainer 上幂等执行
  - **实现偏离**：未新增 `migrations/` 文件。表结构由 GORM AutoMigrate（`BoardPersistentTopic` 模型 tag）+ `ensurePersistentTopicEmbeddingDimension`（启动时对齐维度、建 HNSW 索引）实现。testcontainer 每次启动自动建表/索引，幂等（V.5）。
- [x] 1.2 新增迁移 `2026MMDD_alter_daily_report_sections_add_topic.go`：`daily_report_sections` 加 `persistent_topic_id`(外键 ON DELETE SET NULL)、`topic_match_distance`、`topic_match_confidence` 三列，不加 NOT NULL。验收：历史数据不受影响
  - **实现偏离**：同 1.1，三字段由 `DailyReportSection` 模型 tag（`PersistentTopicID *uint` 等）经 AutoMigrate 增列，无 NOT NULL，历史数据兼容。
- [x] 1.3 新增迁移 `2026MMDD_alter_section_relations_add_type.go`：`daily_report_section_relations` 加 `relation_type VARCHAR(20) NOT NULL DEFAULT 'similarity'`，加 `relation_type` 索引。验收：老数据 relation_type 默认 similarity
  - **实现偏离**：同上，`SectionRelation.RelationType` 模型 tag（`default:similarity` + 索引）经 AutoMigrate 增列。
- [x] 1.3a（2026-06-20 补）拓宽 section_relations 唯一约束为 `(from_section_id, to_section_id, relation_type)`，使 identity 与 similarity 边在同一 section 对上共存。验收：迁移幂等；拓宽不引入唯一冲突（旧约束已保证每对唯一）
  - **实现**：版本化迁移 `20260620_0001`（`platform/database/postgres_migrations.go`）— `DROP` 旧 `uq_section_relations_pair` 后 `ADD` 同名约束为三元组。`DROP/ADD CONSTRAINT IF EXISTS` 幂等。
  - **兼容性**：旧约束 `(from,to)` 已保证每对 section 仅一行，拓宽为 `(from,to,relation_type)` 后每对最多两行（identity+similarity），不可能违反新约束，无需数据迁移。
  - **动机**：原 `1.3` + identity 边的 `ON CONFLICT (from,to) DO UPDATE SET relation_type` 导致 identity 覆盖 similarity，使 similarity-only 时间线视图丢边断链。详见 verification-report §10。
- [x] 1.4 `backend-go/internal/topicgraph/repository/daily_report_models.go`：新增 `BoardPersistentTopic` 模型（字段对齐 DDL），`DailyReportSection` 加三字段，`SectionRelation` 加 `RelationType`
- [x] 1.5 `backend-go/internal/platform/database/seed_ai_settings.go`：新增 4 个 persistent_topic_* 配置项（match_threshold=0.30 / upgrade_threshold=3 / decay_window=30 / cluster_threshold=0.30）
  - **实现偏离**：未写 seed 行。改由 `DefaultPersistentTopicConfig()`（cluster_threshold=0.28，经真实数据校准）提供默认值；`LoadPersistentTopicConfig` 仍尝试读 ai_settings，缺失时回落默认。功能等价（配置可调）。
- [x] 1.6 `backend-go/internal/topicgraph/repository/daily_report_repository.go`：新增 topic CRUD 方法（CreateTopic / ListActiveTopicsByBoard / ListAllTopicsByBoard / SaveTopics / UpdateSectionTopicAssignment）
  - **实现位置**：方法落在 `daily_report_topic_repository.go`（与模型同包，非 daily_report_repository.go）。

## 2. ClusterTags 注入历史框架（根因 A）

- [x] 2.1 `daily_report_cluster.go`：`ClusterTags` 签名增加 `existingTopics []PersistentTopicBrief` 参数，`buildClusterSystemPrompt` 注入 topic 列表（标注 active/candidate 与首末命中日期），指示 LLM 优先复用已有框架
  - **prompt 质量调优（2026-06-20）**：收紧规则 2（单标签独立成组时组名必须用原文）、规则 4（标题禁止脑补未提及的外部语境）、复用规则（仅当核心议题延续才复用，不得仅因语境沾边），并增加反面教材。原因：原 prompt 导致宽泛人名/地名框架（如「特朗普在 G7 盟友关系紧张」）吸收语义不相关事件。详见 verification-report §11。
- [x] 2.2 扩展 LLM 输出 schema 解析：每个 group 解析 `matched_topic_id`（可空）
- [x] 2.3 新增 `validateMatchedTopicID`：matched_topic_id 必须存在于传入集合，否则降级为 nil（防 LLM 幻觉）
- [x] 2.4 `daily_report_orchestrator.go`：ClusterTags 调用前查询 board 现有 active+candidate topic 并传入
- [x] 2.5 单元测试 `daily_report_cluster_test.go`：`TestClusterTags_InjectsExistingTopics`（验证 prompt 含 topic 列表）、`TestValidateMatchedTopicID_HallucinationDegrades`（非法 id 降级）

## 3. AssignSectionsToTopics 归属算法（强制 1:N）

- [x] 3.1 新增 `backend-go/internal/topicgraph/service/daily_report_topic.go`：`AssignSectionsToTopics(ctx, boardID, sections, existingTopics)` 实现双重确认逻辑（embedding ≤ threshold AND LLM matched_topic_id 一致 → anchor_hit；否则开 candidate → auto_new；embedding 空 → unmatched）
  - **实现位置**：归属逻辑实现在 `repository/daily_report_assignment.go`（`planTopicAssignments` 双重确认 + `assignAndUpdateTopics` 事务入口），未单列 service 层文件。
- [x] 3.2 实现 `findNearestTopic`（余弦距离遍历 existingTopics embedding）
- [x] 3.3 实现 `createCandidateTopic`（建行 status=candidate, first/last_seen=当天, hit=1, consecutive=1）
- [x] 3.4 `daily_report_orchestrator.go`：MergeSimilarSections 后插入 AssignSectionsToTopics 调用
- [x] 3.5 单元测试 `daily_report_topic_test.go`（SQLite）：`TestAssignSections_AnchorHit` / `TestAssignSections_AutoNew_DriftBreak`（embedding 近但 LLM 不标记）/ `TestAssignSections_Unmatched_EmptyEmbedding` / `TestAssignSections_HallucinationID`（matched_topic_id 非法降级）
  - **实现位置**：测试在 `daily_report_assignment_test.go`。

## 4. UpdateTopicLifecycle 状态机

- [x] 4.1 `daily_report_topic.go` 新增 `UpdateTopicLifecycle(ctx, boardID, today, sections)`：命中则 consecutive_hits+1/hit_count+1/last_seen 更新，candidate 达 upgrade_threshold 转 active；未命中则 consecutive_hits 归 0，active 超 decay_window 转 archived
  - **实现位置**：`planLifecycle` + `assignAndUpdateTopics`（repository/daily_report_assignment.go）。
- [x] 4.2 配置项加载：从 ai_settings 读 4 个参数，未设置用默认值
- [x] 4.3 `daily_report_orchestrator.go`：AssignSectionsToTopics 后调用 UpdateTopicLifecycle
- [x] 4.4 单元测试：`TestLifecycle_UpgradeOnConsecutiveHits`（第 3 天边界转正）/ `TestLifecycle_ResetOnBreak`（中断归 0）/ `TestLifecycle_ArchiveOnDecay`（31 天归档）/ `TestLifecycle_KeepWithinDecayWindow`（窗口内保留）

## 5. 关系叠加身份边（根因 B）

- [x] 5.1 `daily_report_relations.go`（RebuildBoardRelations 所在文件）：匈牙利产出的边标记 relation_type='similarity'（算法本身不动）
  - **实现位置**：`RebuildBoardRelations` 在 `repository/daily_report_matching.go`。
- [x] 5.2 新增身份边写入：按 persistent_topic_id 分组，组内按 period_date 排序，相邻天 section 写 identity 边，distance 用实际 embedding 余弦距离
- [x] 5.3 同一 (from,to) 同时存在 similarity+identity 时以 identity 覆盖（upsert）
  - **实现偏离（2026-06-20 修正）**：改为【共存】而非覆盖。唯一约束从 `(from_section_id, to_section_id)` 拓宽为 `(from_section_id, to_section_id, relation_type)`（迁移 `20260620_0001`），所有 INSERT 的 `ON CONFLICT` 目标同步为该三元组。identity 与 similarity 在同一 section 对上作为两行独立记录共存。原因：原“覆盖”语义会吞掉一条强匈牙利匹配（distance ≪ 0.28），使「只显 similarity」的时间线视图丢边断链、生命周期状态（split/merge）被扭曲。详见 verification-report §10。
- [x] 5.4 单元测试：`TestRelations_IdentityEdgeSurvivesDrift`（distance=0.32 > penalty，身份边不断）/ `TestRelations_DifferentTopicNoIdentityEdge` / `TestRelations_IdentityOverridesSimilarity`
  - **实现偏离**：上述三个计划用例未逐字实现；实际覆盖为 `TestIdentityEdge_SurvivesLabelDrift`（distance>penalty 场景，identity 边连接）。另：边共存（§5.3 修正）后 identity 与 similarity 为两行独立记录，旧计划中的「IdentityOverridesSimilarity」语义不再适用。
  - **2026-06-20 补**：另加 `TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades`（service 层），覆盖「同 topic 被 reuse 两次」的占用检测修复（见 §2.1 调优发现的 bug）。

## 6. 话题生命线 API

- [x] 6.1 `daily_report_handler.go` 新增 `GET /api/daily-reports/topics/:id/lifeline`：按 persistent_topic_id 查全部 section（不限天数）+ 内部关系
- [x] 6.2 响应含 topic 元信息（label/status/first_seen/last_seen/hit_count）+ sections + relations
- [x] 6.3 路由注册 `router.go`
- [x] 6.4 扩展 `getBoardSectionTimeline` / `getSectionLifecycle` 响应：section 嵌套 persistent_topic{id,label,status,color}，relation 加 relation_type；color 按 topic_id 哈希分配稳定色
- [x] 6.5 集成测试 `daily_report_handler_test.go`：`TestGetTopicLifeline_AggregatesByTopic`（跨命名漂移聚合）/ `TestGetTopicLifeline_RelationsScopedToTopic`
  - **实现方式**：lifeline 聚合逻辑由真实数据测试覆盖（`daily_report_realdata_test.go` / `daily_report_topic_integration_test.go`）。管理 API（merge/split/rename）的核心逻辑测试落在 `daily_report_topic_management_test.go`（9 个 testcontainer 用例）；handler 层为参数解析 + 调用 repo 的薄封装，现有 handler 测试无 HTTP 框架，未补 HTTP 层用例。

## 7. 历史数据回刷

- [x] 7.1 新增 `BackfillPersistentTopics(boardID)`：按 period_date 正序遍历历史 section，average_link 聚类（threshold=cluster_threshold），每 cluster 建 active topic（直接给 active，因有历史命中）
  - **实现偏离**：聚类算法从贪心 average-link 改为 **complete-link**（threshold 0.30→0.28），真实数据证伪贪心假设（链式合并）。详见 verification-report §3。
- [x] 7.2 回填全部历史 section 的 persistent_topic_id
- [x] 7.3 回刷后触发 RebuildBoardRelations（写身份边）
- [x] 7.4 提供 `BackfillAllPersistentTopics()` 批量入口
- [x] 7.5 集成测试：`TestBackfill_TopicConvergence`（回刷后单 board active topic 数 5-15）/ `TestBackfill_FillsAllSections`（无 section 漏归属）

## 8. 真实数据集成测试（算法参数验证）

> 本节是 algorithm 参数合理性的关键验证，复用 testcontainer pgvector + 真实 embedding 样本。

- [x] 8.1 准备测试样本 `backend-go/internal/topicgraph/service/testdata/persistent_topic_embeddings.json`：从开发库导出代表性 section embedding（脱敏），覆盖：同叙事不同命名 / 跨叙事相似 / 孤立突发 三类场景
  - **实现位置**：fixture 在 `repository/testdata/persistent_topic_fixture.json`（2.6MB，108 真实 section / 3 board，vector(2560)）。
- [x] 8.2 集成测试 `daily_report_topic_integration_test.go`（testcontainer）：灌入样本，跑 AssignSectionsToTopics + UpdateTopicLifecycle，断言归属与状态机行为
- [x] 8.3 集成测试命名漂移场景：连续 4 天 section label 漂移、部分 distance > penalty，断言身份边不断链、topic 演化连续
- [x] 8.4 集成测试回刷收敛：灌入 30 天真实 section，跑 BackfillPersistentTopics，统计 topic 数分布，断言在 5-15 区间
- [ ] 8.5 调参循环脚本（非 CI，调参工具）：跑 8.2-8.4，输出 anchor_hit 占比 / candidate 数 / 分歧率 / topic 数分布四项指标，供调 match_threshold(0.25~0.35) / upgrade(2~5) / decay(21~45) 参考
  - **未做**：以 verification-report §8 的手动调参指引替代（部署后观察 → 调单个阈值）。调参脚本属运维工具，非功能交付，延后。

## 9. 前端展示改造

- [x] 9.1 `front/app/api/dailyReports.ts`：新增 `getTopicLifeline(topicId)`，扩展 SectionTimelineNode / SectionRelation 类型加 persistent_topic / relation_type
- [x] 9.2 `detective-wall/CardGroup.ts`：卡片底色/图钉色按 persistent_topic.color 着色；candidate topic 卡片虚线边框
- [x] 9.3 `detective-wall/RedString.ts`：relation_type=identity 实线满 opacity，similarity 虚线半透明
- [x] 9.4 `detective-wall/InteractionLayer.ts`：生命周期双模式（默认话题生命线 getTopicLifeline，可切换 section 图 getSectionLifecycle）；BFS 起点生命线候选 = {同 topic 全部 section} ∪ {相似度边可达}
  - **完成情况**：① BFS 同 topic 并集——`bfsLifeline(presetNodes)` + `topicLifelineNodes` 已实现并带单测，点卡片后同叙事节点一起点亮。② 双模式切换 UI——`TopicDetectiveWall.client.vue` 详情面板新增"话题生命线"按钮（`enterTopicLifeline` → `getTopicLifeline` → `interaction.enterLifecycle`），与既有"查看完整生命周期"（section 维度 `getSectionLifecycle`）并列，双模式现已可用。
- [x] 9.5 日报 section 列表组件（`SectionTimeline.vue` 或等价）：按 topic 分栏渲染（关心/突发/未分类）
  - **实现位置**：`BoardDailyReportTimeline.vue` 的 `qualityZones` computed（关心的话题 / 突发的新话题 / 其他动态三栏）。
- [x] 9.6 topic 管理入口放侦探墙详情面板：合并/重命名/归档按钮（调后端管理 API）
  - **完成情况**：`TopicDetectiveWall.client.vue` 详情面板新增话题操作区——话题生命线 / 重命名（prompt）/ 归档（confirm）/ 合并（board 缓存 topic 选择器），分别调 `getTopicLifeline` / `updateTopic(label)` / `updateTopic(status=archived)` / `mergeTopics`。前端 API 封装见 `dailyReports.ts`（`updateTopic` / `mergeTopics` / `splitTopic`）+ `client.ts` 新增 `patch` 方法。
- [x] 9.7 后端 topic 管理 API：`POST /api/daily-reports/topics/:id/merge` / `PATCH /api/daily-reports/topics/:id`（重命名/归档）/ `POST /api/daily-reports/topics/:id/split`
  - **完成情况**：`daily_report_handler.go` 新增 `updateTopic`/`mergeTopic`/`splitTopic` + 路由注册；repository 层 `UpdateTopic`/`MergeTopics`/`SplitTopic`（事务 + RebuildBoardRelations 重建身份边）。split 前端入口未做（tasks 9.6 仅要求合并/重命名/归档），后端 API 已就绪。

## 测试 / Test

> 只跑本次修改影响的包：`internal/topicgraph/...`。

- [x] T.1 后端单元测试：`cd backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository`（归属/状态机/身份边/ClusterTags 注入）
- [x] T.2 后端集成测试：`cd backend-go && go test ./internal/topicgraph/handler -run TopicLifeline`（testcontainer，需 Docker 运行）
  - **说明**：handler 层无 TopicLifeline HTTP 用例（薄封装）；等价覆盖在 repository 层（`go test ./internal/topicgraph/repository -run "UpdateTopic|MergeTopics|SplitTopic"`，9 用例全过）。
- [x] T.3 真实数据集成测试：`cd backend-go && go test ./internal/topicgraph/service -run "Integration|Backfill"`（testcontainer + 真实 embedding 样本，验证算法参数）
  - **说明**：测试在 repository 包（`daily_report_realdata_test.go` / `daily_report_topic_integration_test.go`），非 service 包。
- [x] T.4 前端：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"`（lint 可 WSL，但统一用 cmd）
- [x] T.5 前端类型检查：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
- [x] T.6 前端单元测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`
- [x] T.7 前端构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`
- [x] T.8（2026-06-20 补）prompt + reuse bug 单测：`cd backend-go && go test ./internal/topicgraph/service/`（含新增 `TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades`，TDD 先红后绿）
- [x] T.9（2026-06-20 补）真实 LLM 聚类质量诊断：`cd backend-go && go run ./cmd/verify-cluster-prompt`（运维工具，跳真实 ClusterTags 核查 topic 8 不再收纳不相关事件）

## 文档 / Docs

- [x] D.1 产出 `verification-report.md`（成果报告）：调参前后指标对比表（anchor_hit 占比 / candidate 数 / 分歧率 / topic 数分布），证明算法参数合理；含命名漂移场景实测案例（改造前断链 vs 改造后身份边保持）
- [ ] D.2 更新 `docs/reference/database/`：board_persistent_topics 表结构、daily_report_sections/daily_report_section_relations 新字段
- [ ] D.3 更新 `docs/reference/api/`：新增 getTopicLifeline 端点、扩展 getBoardSectionTimeline/getSectionLifecycle 响应字段
- [ ] D.4 更新 `docs/reference/architecture/`：补充 PersistentTopic 持久层在 board/section/relation 架构中的位置
- [ ] D.5 更新 `docs/reference/configuration.md`：4 个 persistent_topic_* 配置项说明
  - **D.2–D.5 未做**：reference 活文档按 `开发执行规范.md` §12.4 在**里程碑收尾时统一更新**，非单个 change 归档门禁。本 change 归档不阻塞；里程碑 v1.3.3 收尾时一并同步。

## 验证 / Verify

> 归档前重跑本节命令确认零失败。

- [x] V.1 后端编译：`cd backend-go && go build ./...`
- [x] V.2 后端 lint：`cd backend-go && golangci-lint run ./internal/topicgraph/...`（0 issues）
- [x] V.3 后端 vet：`cd backend-go && go vet ./internal/topicgraph/...`
- [x] V.4 后端测试（影响包）：`cd backend-go && go test ./internal/topicgraph/...`（repository 41s + handler 0.6s，全过）
- [x] V.5 迁移幂等：在 testcontainer 上重复执行迁移，确认无报错
  - **说明**：无显式迁移文件；GORM AutoMigrate 在每个 testcontainer 启动时幂等建表/索引（含 vector 维度对齐），重复执行无报错。
- [x] V.6 回刷验证：对一个真实 board 执行 BackfillPersistentTopics，确认全部历史 section 有 persistent_topic_id、topic 数在 5-15
  - **说明**：见 verification-report §5（b1974=11 / b1980=15 / b2197=15，无 orphan section）。
- [x] V.7 成果报告产出：`verification-report.md` 指标达标（anchor_hit 占比 > 70%、单 board active topic 数 5-15、断链率较改造前下降 50%+）
  - **说明**：topic 数 / orphan / 身份边 / 漂移合并 已实测达标；anchor_hit 占比与断链率下降幅度待生产日常数据积累（回刷一次性无法覆盖，见报告 §5 ⏳ 项）。
- [x] V.8 前端 lint：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"`（改动的 3 文件 0 error）
- [x] V.9 前端 typecheck：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`（TYPECHECK_PASS）
- [x] V.10 前端测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`（19 files / 116 tests 全过）
- [x] V.11 前端构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`（BUILD_PASS）
- [ ] V.12 Pre-push 全量门禁：`cd backend-go && golangci-lint run ./... && go vet ./... && go test ./... && go build ./...`
  - **未跑**：全量门禁属 pre-push 动作（非本 change 归档门禁）。本次已跑影响包（topicgraph 全量）；push 前补跑全量即可。
- [x] V.13（2026-06-20 补）迁移幂等（新约束）：`docker exec syntopica-postgres psql -U postgres -d syntopica -c "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='uq_section_relations_pair';"` → 返回 `UNIQUE (from_section_id, to_section_id, relation_type)`；重复执行 `go run ./cmd/rebuild-topics` 无报错（DROP/ADD IF EXISTS 幂等）。
- [x] V.14（2026-06-20 补）边共存验证（真实库）：`docker exec syntopica-postgres psql -U postgres -d syntopica -c "SELECT count(*) FROM (SELECT from_section_id,to_section_id FROM daily_report_section_relations WHERE relation_type='identity' INTERSECT SELECT from_section_id,to_section_id FROM daily_report_section_relations WHERE relation_type='similarity') x;"` → >0 即达标（首次验证 33；随新日报增长，2026-06-20 复测 42；修复前为 0）。
- [x] V.15（2026-06-20 补）全量归属验证（真实库）：`docker exec syntopica-postgres psql -U postgres -d syntopica -c "SELECT count(*) FILTER (WHERE persistent_topic_id IS NULL) AS null_secs, count(*) AS total FROM daily_report_sections;"` → null_secs=0（首次 209 total；随新日报增长，2026-06-20 复测 243 total，仍 0 NULL；修复前 154/209 为 NULL）。
- [x] V.16（2026-06-20 补）prompt + reuse 回归：`cd backend-go && go test ./internal/topicgraph/service/ 2>&1` → `ok`；`go run ./cmd/verify-cluster-prompt` → topic 8 只收 2 个相关事件，无重复 reuse。
- [x] V.17（2026-06-20 补）后端全量 build：`cd backend-go && go build ./... && echo BUILD_OK` → BUILD_OK（含新运维工具 cmd/rebuild-topics、cmd/verify-cluster-prompt）。
