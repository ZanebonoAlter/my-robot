# Apply 实施上下文 — daily-report-lane-driven-clustering

> apply 子线程读这份 + `proposal.md`/`design.md`/`specs/` 即可动手，不必重新逆向代码。调研全貌见 `docs/experience/cluster-bias-investigation.md`。

## 0. 架构关键（先记住）
- **归属逻辑在 repository 层**，不在 service：`planTopicAssignments` / `assignAndUpdateTopics` 都在 `internal/topicgraph/repository/daily_report_assignment.go`。
- **配置集中在 `PersistentTopicConfig`**（`repository/daily_report_topic_repository.go:18`），新阈值加这里 + `DefaultPersistentTopicConfig`(L36) + `LoadPersistentTopicConfig`。
- **section embedding 当前从 cluster_label 算**（`orchestrator.go:262-287`），用于事后匹配 topic。新方案 section 天生挂 topic，但 **embedding 仍要算**（用于：更新 topic 质心 + thread fit 距离）。
- **assignAndUpdateTopics 在 SaveReport 内调用**（`repository/daily_report_repository.go:231`），不是 orchestrator 直接调。

## 1. service/daily_report_cluster.go（聚类收窄）

### 现状
- `ClusterTags(ctx, tags, existingTopics, briefs)` L114：对**全部** tag 调 LLM 自由聚类；`len(tags)<=2` 兜底各自成组（L122-130）。
- `buildClusterSystemPrompt` L16：注入 active+candidate topic 框架 + active 近期 briefs（L58-96）。
- `parseClusterResponse` L204：解 `{group_name, tag_ids, matched_topic_id}`；`usedTopicIDs` 保证一个 topic 只被一个 group 认领（L234，**新方案要放宽：L2 多 tag 可挂同 topic**）。
- `ClusterGroup` struct（`repository/daily_report_models.go:246`）：`GroupName/TagIDs/MatchedTopicID`。

### 目标改动
- **新增 `BucketTagsByCentroid(tags, topicsWithCentroid, cfg) → {L1,L2,L3}`**：每个 tag（用 `topic_tag_embeddings.semantic`）算到各 topic `centroid` 的 cosine 距离 argmin；L1<`LaneL1Threshold`(0.18) / L2 [0.18,0.30] / L3>`LaneL2Threshold`(0.30)；吸尘器降级（挂 `IsVacuum=true` topic 的 tag 从 L1 移 L2）。
- **`ClusterTags` 拆 L2/L3 分支**（不再收全量）：L2 注入每 tag top-K(`L2CandidateK`)候选 topic + briefs，输出 `{decision:keep/switch/new, target_topic_id}`；L3 起新叙事 `{group_name}`。`len(去重 tags)<=2` 兜底保留。
- **`parseClusterResponse` 适配新 schema**：L2 解 decision/target_topic_id；L3 解 group_name。target 不在候选集 → 降级 new + 标注 `llm_target_off_shortlist`。
- **`ClusterGroup` 加字段**：`Decision string` / `TargetTopicID *uint`（L2）/ `Lane string`（l1/l2/l3）。

### 注意
- L1 不调 LLM，**按 argmin topic 直接成组**（同 topic 的 L1 tag 合并）。成组逻辑从"LLM 输出"变成"按 topic 聚合"。
- `usedTopicIDs`（一 topic 一 group）要放宽：L2 多 tag 可挂同 topic。

## 2. service/daily_report_orchestrator.go（管线顺序）

### 现状管线（`GenerateDailyReport` L31）
Step1 collectBoardTags(38) → Step2 Deduplicate(53) → Step2.5 filterTagsByQuality(56) → Step3 ListAnchorableTopicsByBoard(64) → Step3.5 topicBriefs(81) → **ClusterTags(98)** → Step5 GenerateHighlights+GenerateClusterThreads(122-129) → Step6 assemble(151) + section embedding(262-287) → Step6.5 computeThreadFitDistances(294) → Step7 merge(296) → SaveReport（含 assignAndUpdateTopics）。

### 目标管线（反转）
Step1-2.5 不变 → Step3 ListAnchorableTopicsByBoard（**返回 centroid+is_vacuum**）→ **新增 BucketTagsByCentroid 分桶** → L1 直接成组 / L2+L3 调 ClusterTags → Step5 highlights/threads（按新 groups）→ Step6 组装 section（**天生挂 topic，lane_tier 入库**）+ section embedding（保留）→ Step7 merge → SaveReport → **质心增量更新 + 吸尘器统计更新**。

### 关键
- **移除事后 section 标题↔topic 匹配**：现归属靠 section embedding 匹配 topic（assignment.go），新方案归属在分桶阶段已定。
- section embedding（L262-287）**保留**：用途变为①更新 topic centroid ②computeThreadFitDistances。

## 3. repository/daily_report_assignment.go（归属重构）

### 现状
- `planTopicAssignments(sections, existingTopics, cfg, today)` L155：**双重确认 AND-gate**——L175 `findTopicsWithinThreshold(vec, parsed, cfg.MatchThreshold)` + L176-180 `sec.MatchedTopicID` 一致 → anchor_hit；否则 auto_new。
- `assignAndUpdateTopics(tx, boardID, periodDate, sections)` L264：写库（创建 candidate L288 `Embedding: spec.embedding`、`UpdateSectionTopicAssignment` L322、`planLifecycle` L227 更新 hit_count）。
- 辅助：`parseTopicEmbeddings` L65 / `findNearestTopic` L79 / `findTopicsWithinThreshold` L106 / `cosineDistance` L40（**分桶可复用 cosine**）。

### 目标改动
- **`planTopicAssignments` 重构**：不再双重确认，改为**接收上游分桶结果**（section 已带 lane + topic 归属），直接映射：L1/L2-keep/L2-switch → anchor_hit + lane_tier=l1_direct/l2_llm；L3 → auto_new + l3_new；embedding 空 → unmatched。
- 或归属完全前移到 service 分桶，`planTopicAssignments` 简化/废弃，`assignAndUpdateTopics` 直接读 section.LaneTier + PersistentTopicID 写库。
- **`assignAndUpdateTopics` 加**：①写 lane_tier（`UpdateSectionTopicAssignment` 签名加 laneTier）②新 section 入库后触发 `UpdateCentroidOnSectionChange`③每日生成后 `RecomputeVacuumStats`。

## 4. repository/daily_report_topic_repository.go（质心 + 配置 + 吸尘器）

### 现状
- `PersistentTopicConfig` L18：MatchThreshold/UpgradeThreshold/CandidateDecayWindow/CandidatePromptLimit/ClusterThreshold。
- `DefaultPersistentTopicConfig()` L36（0.30/3/7/20/0.28）。
- `LoadPersistentTopicConfig(r.db)` 从 ai_settings 加载。
- `ListAnchorableTopicsByBoard(boardID, reportDate, cfg)` L185：查 active+candidate topic → `selectAnchorableTopics`(L193) 筛窗口内 candidate。

### 目标改动
- **`PersistentTopicConfig` 加字段**：LaneL1Threshold(0.18)/LaneL2Threshold(0.30)/VacuumRatio(0.20)/CentroidWindow(30)/VacuumWindow(7)/L2CandidateK(5)。同步 Default + Load + ai_settings seed。
- **新增方法**：
  - `ComputeTopicCentroid(topicID)`：取该 topic 最近 `CentroidWindow` 条 section embedding 加权平均；section<2 用首义（现有 `Embedding`）。
  - `UpdateCentroidOnSectionChange(topicID)`：section 新增/归属变更后增量重算。
  - `RecomputeVacuumStats(boardID)`：按 `VacuumWindow` 统计 attracted/strong/mid，更新 `IsVacuum`。
- **`ListAnchorableTopicsByBoard` 返回 centroid + is_vacuum**（替换首义 embedding 用法）。`BoardPersistentTopic.Embedding`（首义）保留作退化兜底。

## 5. repository/daily_report_models.go（struct 变更）
- `BoardPersistentTopic`(L83) + `Centroid string(gorm:"type:vector")` / `IsVacuum bool` / `VacuumStrong int` / `VacuumMid int`。
- `DailyReportSection`(L115) + `LaneTier string`。
- `ClusterGroup`(L246) + `Decision string` / `TargetTopicID *uint` / `Lane string`。

## 6. platform/database/postgres_migrations.go（迁移）
照现有迁移模式（`Description` + `tableExists`/column 检查 + ALTER/CREATE + 幂等）。参考线程迁移 L477-527 的 column-exists + ALTER + 数据回填。
- 新迁移：①`board_persistent_topics` ADD COLUMN centroid vector(2560)/is_vacuum bool/vacuum_strong int/vacuum_mid int ②`daily_report_sections` ADD COLUMN lane_tier varchar(16) ③离线构建 centroid（按历史 section embedding，窗口 30）④初始化 is_vacuum（近 7 天吸引统计）。

## 7. 关键陷阱（apply 注意）
- **归属在 repository**：别在 service 重写归属逻辑，改 `assignment.go`。
- **section embedding 别删**：用途从"匹配 topic"变"更新质心 + thread fit"。
- **assignAndUpdateTopics 在 SaveReport 内**（`repository/daily_report_repository.go:231`）：质心/吸尘器更新加这里或其后。
- **ClusterGroup schema 变更**影响 raw_clusters JSON 调试 + parseClusterResponse + 前端（如有读 clusters）。
- **pgvector**：centroid 用 vector(2560)，`avg(vector)` 聚合（pgvector>=0.5 支持，调研查询已验证可用）。
- **ai_settings seed**：新阈值 key 命名沿用 `persistent_topic_*` 前缀（如 `persistent_topic_lane_l1_threshold`）。

## 8. 验证锚点（apply 后对照）
- SQL 复算 lane 分布：L1~47% / L2~51% / L3~1.3%（见 design.md 数据）。
- 吸尘器 topic（中国央行新闻 / 开发者工具链从本地调试 / XR 硬件生态）is_vacuum=true。
- 无事后 section↔topic 匹配（assignment.go 不再 `findTopicsWithinThreshold` 做归属）。
- centroid 首次离线构建后，topic 强挂率应贴近调研值（<0.20 占 ~62%）。
