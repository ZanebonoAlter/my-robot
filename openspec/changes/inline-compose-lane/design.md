## Context

Syntopica 话题系统的「新建泳道」由 `2026-07-05-manual-topic-lane` 引入，落地为 `BoardThreadBrowser` 的 `viewMode='compose'` 全屏编排态（`ComposePanel.vue`）。`viewMode` 四态（timeline/lanes/focus/compose）互斥全屏替换，导致编排态盖掉 lanes 泳道主视图，用户选候选时丢失 active 泳道全貌上下文。

本 change 把编排态搬到 lanes 主视图就地完成，编排态与浏览态合一。纯前端交互重构，后端 API/数据模型全复用。

## Goals / Non-Goals

**Goals:**
- 编排态就地叠加在 lanes 上（新增 `composeMode` 布尔叠加态，不改 `viewMode`），编排时仍见泳道全貌。
- active 泳道淡显保留做背景参照，恢复可观性。
- unassigned 泳道为主战场，section 节点就地勾选 + 实时贴合度（distance/tier/离群）。
- 候选 topic 侧边栏「采纳」一键起步（预填名 + 预勾）。
- 允许勾走 active section（移出到新泳道），带实时移出提示 + 保存前二次确认。

**Non-Goals:**
- 多条新泳道并发编排（一次只建一条）。
- 主题撞车 `crashReport`（新泳道 centroid 跟其它 topic 太近，属主题级，区别于成员移出）。
- 「未来预期」体检卡（v1 本就未实现）。
- 后端改动 / 新 API / 数据模型变更（全复用）。

## Decisions

1. **`composeMode` 叠加态 vs 新 `viewMode`**：选叠加态。`viewMode` 互斥全屏替换正是痛点根源；叠加态让编排时 lanes 全貌仍在。备选（新增 viewMode）被否，因它仍是全屏替换。
2. **数据源 = `compose-candidates` 池，非 lanes `SectionTimelineNode`**：池已带 embedding + `persistentTopic{id,label}`，覆盖 unassigned + active 全量带向量 section。lanes 节点可能不带 embedding，直接用池做数据源，按 section id 对齐渲染。
3. **移出走 `createManualLane`，无需新 API**：`createManualLane(boardId, label, sectionIds)` 只收 label + section_ids，section 重指新 topic 即自动离开原 active 泳道。原泳道名直接读 `candidate.persistentTopic.label`。
4. **撞车分两类，本期只做成员移出**：成员移出（勾走 active section，本期做，节点标「将从【泳道X】移出」+ 保存前二次确认）vs 主题撞车 `crashReport`（centroid 级，本期 YAGNI 砍）。备选（两者都做）被否，因主题撞车价值低且增加复杂度。
5. **聚类质量卡数据源**：`aggregatePreview` 只返回 `{mean, skipped}`，无 cohesion/离群数。平均距离/离群数前端用 `cosineDistance(v, mean)` 求平均 + `outlierFlags(distances, threshold)` 计数自算。
6. **废弃 `ComposePanel.vue`（替换不共存）**：用户嫌旧的割裂，本期又砍了两张体检卡，新旧功能重叠没必要留。备选（共存当高级模式）被否。

## Risks / Trade-offs

- **[编辑态布局密度]** unassigned + 淡显 active + 顶部工具条 + 右侧侧边栏同屏 → 可能拥挤。→ Mitigation：侧边栏可折叠；淡显 active 低 opacity（~0.3）。
- **[淡显 active 可勾走误操作]** 用户误勾 active section 导致非预期移出。→ Mitigation：节点实时标「将从【泳道X】移出」+ 保存前二次确认列全部移出项。
- **[废弃 ComposePanel 回滚]** 替换后回滚需 git revert。→ Mitigation：纯前端无数据影响，git revert 干净。
- **[采纳预勾过多]** matchThreshold 内 section 可能很多。→ Mitigation：设数量上限（实现时定）。

## Migration Plan

- 纯前端交互重构，无 DB schema / API 变更，无数据迁移。
- 部署：前端发版即可；新建泳道仍走现有 `createManualLane`，历史 `source=manual` 泳道数据完全兼容。
- 回滚：git revert 前端变更恢复 `ComposePanel`；历史手动泳道数据不受影响。

## Open Questions

- 「采纳」预勾 section 数量上限（实现时定；候选：固定 N 或 matchThreshold 内全勾）。
- 编辑态各区域具体布局尺寸/坐标（实现时 UI 微调）。

---

## 关键代码索引（实现速查）

### 前端

**编排宿主** `front/app/features/tags/components/BoardThreadBrowser.vue`
- L33 `viewMode`（timeline/lanes/focus/compose 四态互斥）—— 本期新增 `composeMode` 叠加态，不改 viewMode
- L211 unassigned 泳道硬编码：`{ key:'unassigned', label:'待确认 / 未分类', color:'#64748b', status:'none', topicId:null }`
- L225 `sectionLaneKey(s)` → `s.persistent_topic_id` 空归 `'unassigned'`
- L526 `unassignedCount = sections.filter(s => !s.persistent_topic_id).length`
- L589 unassigned 泳道不进专注视图

**待废弃** `front/app/features/tags/components/ComposePanel.vue`
- L21 import composeReport 导出；L608-656 体检三卡；L671 撞车确认弹窗；内部 `matchThreshold = 0.3`

**纯逻辑（保留复用）** `front/app/features/tags/components/composeReport.ts`
- `cosineDistance(a, b)` L16
- `AggregateResult { mean: number[]|null, skipped: number }` L31 —— **只给 mean，无 cohesion/离群数**
- `aggregatePreview(vectors)` L42 —— mean pooling 算锚点，镜像后端 `aggregateEmbeddings`
- `outlierFlags(distances, threshold)` L69 —— `distance > threshold×1.3` 标离群
- `DistanceTier = 'good'|'boundary'|'outlier'|'far'` L75；`distanceTier(distance, threshold)` L87；`TIER_LABEL` L94
- `crashReport(...)` L126 —— 主题撞车（本期不做）；`filterPoolByRange` L154；`rankCandidates` L184

**API 层（保留复用）** `front/app/api/persistentTopics.ts` → `usePersistentTopicsApi()`
- `getComposeCandidates(boardId, days)` → GET `/compose-candidates` → `{ sections: ComposeCandidate[], matchThreshold }`
- `createManualLane(boardId, label, sectionIds)` → POST `/persistent-topics/manual` → `{ topic, skipped }`
- `embedQuery(boardId, query)` → POST `/embed-query`（文本→向量，供语义排序）
- `ComposeCandidate { id, embedding, persistentTopicId?, persistentTopic?:{id,label} }` —— **已带现有归属**，移出提示直接读 `.persistentTopic.label`
- `createManualLane` 只收 label + section_ids —— section 重指新 topic 即自动离开原 active 泳道，移出无需额外 API

### 后端

- `backend-go/internal/topicgraph/handler/daily_report_handler.go` L72-74 路由 / L667-691 `composeCandidates` handler → `Repo.GetComposeCandidates(boardID, days)`
- `backend-go/internal/topicgraph/repository/daily_report_manual_topic.go` L76 `ComposeCandidateSection` / L89 `ComposeCandidatesResponse` / L94+ `GetComposeCandidates` WHERE `embedding IS NOT NULL` + 时间窗，**不限 unassigned**
- candidate topic（侧边栏数据源）`backend-go/internal/topicgraph/repository/daily_report_assignment.go` L127 `candidateTopicSpec` —— L3 auto_new、`lane_tier=l3_new`、`consecutive_hits=1`、status=candidate

### 数据模型要点

- section 归类由 `persistent_topic_id` 决定（null = unassigned）
- `DailyReportSection.LaneTier`：l1_direct / l2_llm / l3_new
- topic status：candidate / active / archived
- 阈值：前端 `matchThreshold`（compose-candidates 响应带回）；后端 `PersistentTopicConfig` + `semantic_board_upgrade_cluster_distance_threshold`
