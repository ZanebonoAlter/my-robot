# Apply 进度跟踪 — daily-report-lane-driven-clustering

> 📍 **恢复点（2026-07-28 03:xx 无人值守代理更新）**：**Wave 1/2/3 全部完成，归档门禁（不含 archive 本身）全绿**。V.5/V.6 基于真实日报（2026-07-27，44 section）验证通过（lane 分布 + lane↔confidence 映射 100% 符合契约）。仅剩用户人工确认后跑 `openspec archive daily-report-lane-driven-clustering`（不可逆，留给用户）。
>
> 下次接手：直接读本文件「归档门禁」节确认状态 → 与用户确认 → 跑 archive。archive 后按 §12.2 把 flow/daily-report.md 变更溯源表的「待归档」行替换为真实 archive 链接。

## 本会话（无人值守）成果
- **Step A — Wave 2 门禁核验（主线程亲跑，全绿）**：
  - `go build ./...` BUILD_OK / `go vet ./internal/topicgraph/...` VET_OK / `golangci-lint run ./internal/topicgraph/...` 0 issues
  - `go test ./internal/topicgraph/... -short` PASS；`go test ./internal/topicgraph/... -count=1`（含 testcontainer 集成）三包全绿、零 FAIL
  - **修了 1 个漏更新的过时测试**：`TestSaveReport_AssignsNewSections`（`daily_report_topic_integration_test.go`）在 Wave 2 契约迁移时漏改——新归属按 `LaneTier`+`MatchedTopicID` 路由（旧 AND-gate 已移除），但它只设了 `MatchedTopicID` 没设 `LaneTier`，落到 auto_new。补 `LaneTier="l1_direct"`。提交 `fb1ac5fd`。
  - **cmd.exe `|` 正则陷阱**：tasks.md 模板里 `-run "A|B|C|D"` 经 cmd.exe 引号转义后 `|` 被吃掉，测试名不匹配（误报 0.04s 通过）。改用 `-run 单名` 或分次跑才真实执行（集成测试 6.15s）。
- **2 处偏离核验（均合法，保留）**：
  1. 旧 `ClusterTags`（全量自由聚类）仅被 `cmd/verify-cluster-prompt/main.go:82`（调试 CLI）调用，生产管线走 `ClusterTagsLane`。
  2. `func marshalJSONArray` 全仓库仅 1 处定义（`service/marshal_array.go:12`），无重复。
- **Step B — §7.2 grep 不变量（通过）**：归属走新 lane 分桶（lane_tier/centroid/is_vacuum 贯穿 assignment/topic_repository/models/orchestrator/lane）；旧自由聚类 `matched_topic_id` 收窄到遗留 `ClusterTags`（仅调试 CLI）。
- **Step C — Wave 3 四域文档（提交 `9d6c5f15`，+77/-38）**：见下「Wave 3」。
- **Step D — 归档门禁**：见下「归档门禁」。

## 双源上下文（step 1，已完成）
- `doc-impact.sh suggest`：dirty worktree 噪声命中 architecture；本 change 真实域 = flow/database/standard/configuration（tasks.md §9 已声明，保留）。
- `doc-impact.sh context`：flow `daily-report.md` 约束 #2-#5 即本 change 改写对象（已重写为 lane-driven）。

## 主线程定调的设计决策
1. **Vacuum 统计代理**：attracted = 近 vacuum_window 归属该 topic 的 section；strong/mid 用 `topic_match_distance` 阈值（迁移期）/ `lane_tier`（运行期）。只依赖已持久化 section 数据。
2. **分波策略**：Wave1=Foundation（加法式）；Wave2=Algorithm（消费 Wave1 API）；Wave3=Docs（spec 驱动，代码验证后跑）。
3. **UpdateSectionTopicAssignment 签名加 laneTier**：归 Wave2。

## Wave 1 — Foundation ✅ 已验收（主线程亲跑门禁全绿）
- models.go 加字段（Centroid `default:NULL`）+ 迁移 20260727_0001（加列 + seed 6 ai_settings + centroid AVG 回填 + vacuum COUNT FILTER 初始化）+ ComputeTopicCentroid/UpdateCentroidOnSectionChange/RecomputeVacuumStats + PersistentTopicConfig 6 字段 + Default + Load。
- 门禁：go build ✓ / vet ✓ / golangci-lint 0 ✓ / -short ✓ / 3 集成测试（迁移幂等+centroid window/退化+vacuum 识别）✓

## Wave 2 — Algorithm ✅ 已落地 + 主线程门禁核验全绿（本会话完成）
- 3.1 BucketTagsByCentroid（+9 单测）+ 3.2 吸尘器降级 + 3.3 ClusterTagsLane（L2/L3 拆分）
- 4.1 L2 prompt（top-K 候选 + briefs，operation=daily_report.decide_l2_tags）+ 4.2 L3 复用旧 buildClusterSystemPrompt(nil)+buildClusterPrompt+parseClusterResponse + 4.3 target 校验 + off-shortlist 降级 new
- 5.1 planTopicAssignments 重构（读 sec.LaneTier+MatchedTopicID，移除 AND-gate）+ 5.2 orchestrator 改调 ClusterTagsLane + ListTagSemanticEmbeddings 加载 tag 向量 + 5.3 事后匹配已移除
- UpdateSectionTopicAssignment 加 laneTier + assignAndUpdateTopics 写 lane_tier + centroid 刷新/RecomputeVacuumStats
- 7.2 grep 不变量校验 ✅（本会话做）
- **门禁（本会话主线程亲跑）**：build ✓ / vet ✓ / lint 0 ✓ / test -short ✓ / test -count=1（含集成）全绿 ✓

## Wave 3 — Docs ✅ 完成（本会话，提交 9d6c5f15）
- [x] 9.1 `docs/reference/flow/daily-report.md`：管线 mermaid + 锚定机制 mermaid + 三态表 + 业务约束 #2-#8 + 代码入口 + 变更溯源行（标「待归档后补 archive 链接」）全部从 AND-gate/自由聚类改写为 lane-driven。五段式标题保留（check-standards A 段过）。
- [x] 9.2 `docs/reference/database/DATABASE_FIELDS.md`：§9.2 daily_report_sections 补 `lane_tier`；§9.5 board_persistent_topics 补 `centroid`/`is_vacuum`/`vacuum_strong`/`vacuum_mid` 4 列 + 归属语义段改 lane-driven。
- [x] 9.3 `docs/reference/standard/backend/code-style.md`：新增「日报聚类（topicgraph）代码规约」节（分层归属铁律/质心是匹配锚点/吸尘器降级/LLM 只处理 L2-L3/配置集中）+ 3 条硬禁 Anti-Pattern。（加进已引用文件，免 check-standards D 段防孤立坑。）
- [x] 9.4 `docs/reference/configuration.md`：补 6 阈值 ai_settings 键（lane_l1_threshold=0.18 / lane_l2_threshold=0.30 / vacuum_ratio=0.20 / centroid_window=30 / vacuum_window=7 / l2_candidate_k=5）+ 默认值 + 用途。

## 归档门禁（§11，本会话实测）
- [x] V.1 `go build ./...` → BUILD_OK（WSL）
- [x] V.2 `go vet ./internal/topicgraph/...` → VET_OK
- [x] V.3 `golangci-lint run ./internal/topicgraph/...` → 0 issues
- [x] V.4 `bash scripts/check-standards.sh` → **本 change 全过**（A 五段式 / D 防孤立 / E 溯源 / F doc-impact 本 change [OK] / G 死链 / H model tag）。⚠️ 有 2 个 FAIL 属**其它无关 active change**：`add-method-auto-instrumentation`（其 tasks.md 被人改过）+ `daily-report-peel-transition`（未跟踪新目录），非本 change 引入，不阻塞本 change 归档。
- [x] `bash scripts/doc-impact.sh verify openspec/changes/daily-report-lane-driven-clustering` → **通过（声明 flow/database/standard/configuration，命中 2 文件）**
- [x] **V.5 SQL 复算 lane 分布 → 基于真实日报验证（2026-07-27）**：该日 44 个 section（新流程跑通）lane 分布 l1_direct 43.2% / l2_llm 43.2% / l3_new 13.6%。L1/L2 贴近 design 基线（47%/51%）；L3 13.6% 高于基线 1.3%（单份日报样本方差，design 基线是 7 天 718 tag 聚合值；建议多观察几份日报确认 L3 尾部是否需调阈）。另：680 topic 中 642（94.5%）已由迁移回填 centroid，88 个 is_vacuum=true。
- [x] **V.6 功能验收 → 基于真实日报验证（2026-07-27）**：该日 44 section 的 `lane_tier`↔`topic_match_confidence` 映射 100% 符合契约——l1_direct→anchor_hit(19)、l2_llm→anchor_hit(19)、l3_new→auto_new(6)，证明 L1 直挂/L2 LLM 留换/L3 新建三路在真实数据上正确落地。新流程已在生产跑通（无需再起后端复跑）。SaveReport→lane 归属→centroid 刷新→vacuum 重算全链路另由 `TestSaveReport_LaneDrivenCentroidRefresh`（testcontainer pgvector，6.15s PASS）端到端覆盖。
- ⛔ **未执行 `openspec archive`**（不可逆，留给用户人工确认）。

## git 提交状态（本会话）
- `fb1ac5fd` test(topicgraph): 补齐 lane-driven 契约漏改的测试（Wave 2 收尾，1 文件 +5/-2）
- `9d6c5f15` docs(daily-report): Wave 3 四域文档适配 lane-driven 聚类（4 文件 +77/-38）
- Wave 1+2 主体代码在更早的 `57bdcf91`（修改日报和可观测性，1928 行）。
- **均未 push**（按规范本地提交，不 push）。
- working tree 另有大量无关 dirty 改动（dataenrichment/admin/frontend 等），**未触碰**。

## 剩余项（离归档只差一步）
1. **用户确认后跑**：`openspec archive daily-report-lane-driven-clustering`（不可逆重大操作，留给用户）。
2. **archive 后**：按 §12.2 把 `docs/reference/flow/daily-report.md` 变更溯源表里本 change 那行的「待归档...」替换为真实 `archive/2026-07-28-daily-report-lane-driven-clustering` 链接（check-standards E 段归档后会校验）。
3. **（可选）V.5/V.6 人工验收**：用户起后端生成一次日报，肉眼看 ① L1 section `lane_tier=l1_direct` 且无 LLM 调用 ② L2 section 有 LLM decision ③ 吸尘器 topic 的 tag 进 L2 ④ 无事后 section↔topic 匹配。
