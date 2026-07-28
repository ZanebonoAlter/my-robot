# 就地新建泳道（Inline Compose Lane）设计

> 状态：设计中（brainstorming 产出，待用户 review）
> 日期：2026-07-28
> 相关代码：`front/app/features/tags/components/BoardThreadBrowser.vue`、`ComposePanel.vue`、`composeReport.ts`

## 1. 背景与动机

新建泳道功能目前是一个**独立全屏视图**（`BoardThreadBrowser` 的 `viewMode='compose'`，渲染 `ComposePanel.vue`）。`viewMode` 四态（timeline/lanes/focus/compose）互斥全屏替换，导致：

- 进 compose 编排态后，**lanes 泳道主视图被整体盖掉**，用户选候选 section 时看不到已有 active 泳道全貌，无法对照避免重复/重叠。
- 不满意要返回 lanes 重看，再切回 compose ——「视图来回切换」，交互性与可观性差。

**目标**：把「新建泳道」从独立视图搬到 lanes 泳道主视图**就地完成**，编排态与浏览态合一。

## 2. 已确认决策（brainstorming 结论）

| # | 决策 | 选择 |
|---|------|------|
| D1 | 编排方式 | 就地编辑态叠加在 lanes 上，**不切 viewMode**，新增 `composeMode` 布尔叠加态 |
| D2 | 已有 active 泳道 | **全部保留、淡显做背景参照**（opacity↓、禁交互） |
| D3 | 候选模型 | 主视图勾 section，侧边栏列候选 topic |
| D4 | 主战场 | **unassigned 泳道**（`persistent_topic_id` 为空的 section，前端硬编码 `key:'unassigned'`、`topicId:null`） |
| D5 | 贴合度呈现 | 节点标数字 distance + 边框颜色分层（`distanceTier`）+ 顶部留**一张「聚类质量」体检卡** |
| D6 | 旧 ComposePanel | **废弃**（替换，不共存） |
| D7 | 泳道名输入 | **顶部浮工具条**（输入框 + 保存/取消一行） |
| D8 | active 泳道 section 勾选 | **允许**勾选（= 从原泳道挪到新泳道），勾时实时**成员移出提示**（节点标「将从【泳道X】移出」） |
| D9 | 移出确认 | 保存前**二次确认**，列出全部将从 active 泳道移出的 section（不占常驻卡，仅保存时弹） |
| —  | 主题撞车（crashReport） | 本期**不做**。原 `crashReport`（新泳道 centroid 跟其它 topic 太近）属主题级撞车，与 D8 的成员移出不是一回事，YAGNI 砍掉 |
| D10 | 并发编排 | **一次只建一条**新泳道 |

## 3. 交互设计

### 3.1 触发与状态叠加
- unassigned 泳道头部新增「新建泳道」按钮。
- 点击 → `composeMode=true` 叠加在 lanes 上（`viewMode` 不变）：
  - active 泳道淡显（opacity 降至约 0.3）、禁用点击穿透。
  - unassigned 泳道高亮为主战场，每个 section 节点渲染 checkbox。
  - 顶部浮出工具条，右侧滑出候选侧边栏。
- 保存或取消 → `composeMode=false`，视图恢复浏览态。

### 3.2 顶部浮工具条
- 泳道名输入框（placeholder「给新泳道起个名」）。
- 「已勾 N 个 section」计数（区分 unassigned 来源 / active 移入来源）。
- 「聚类质量」单卡（实时，见 3.5）。
- 取消 / 保存 按钮。保存时若存在 active 移入（撞车），先弹二次确认。

### 3.3 主战场（unassigned 泳道）
- section 节点左上角 checkbox。
- 勾/取消 → 实时重算聚合锚点 `aggregatePreview(vectors)` → 全员重算 `cosineDistance` → 节点更新：
  - 数字 distance 标注。
  - 边框分层（`distanceTier`：good / boundary / outlier）。
  - 离群标黄（`outlierFlags`，`distance > threshold×1.3`）。
- 阈值沿用现有 `matchThreshold = 0.3`。
- **active 勾走的 section 一并纳入 `aggregatePreview` 锚点计算**（它们就是新泳道成员，跟 unassigned 勾选的同等对待）。

### 3.4 active 泳道（淡显背景，可勾走）
- opacity↓，但仍可勾选（D8）。
- 勾一个 active section = 该 section 将从原 active 泳道移入新泳道：
  - 节点实时标注「将从【泳道X】移出」。
  - 工具条计数区显示「N 个来自现有泳道，将移出」。
  - 保存前二次确认（D9）列出全部移出项。

### 3.5 聚类质量卡（顶部，单卡）
- 数据源：`aggregatePreview` 只返回 `{ mean, skipped }`（**无 cohesion/离群数**）。「平均距离/离群数」需前端自行算：选中向量 → `aggregatePreview` 得 mean → 每个选中向量 `cosineDistance(v, mean)` → 求平均；`outlierFlags(distances, threshold)` 计离群数。
- 展示：成员数 / 平均距离 / 离群数，随勾选实时更新。
- 原 ComposePanel 的「撞车检查」卡与「未来预期」卡不保留（未来预期本就是 v1 淡显未实现）；撞车改为保存时动作提示。

### 3.6 候选侧边栏（右侧滑出）
- 列 `status='candidate'` 的 persistent_topic（系统 L3 建议话题，见 `daily_report_assignment.go` 的 `candidateTopicSpec`）。
- 每条：label + 关联 section 数 + 「采纳」按钮。
- 点「采纳」→ 预填新泳道名（用 topic label）+ 在 unassigned 里把与该 topic centroid 距离在 `matchThreshold` 内的 section 预勾（一键起步；预勾上限实现时定，避免一次勾太多）。

### 3.7 保存/取消闭环
- **保存**：复用 `createManualLane` API，传 `{ name, sectionIDs }` → 后端建 `status='active'` 的 persistent_topic + 关联 section（从 unassigned 与/或原 active 泳道移出）→ 前端 lanes 即时渲染新泳道，unassigned 与原泳道相应减少。
- **取消**：清空勾选与名字，`composeMode=false`，退回浏览态（无副作用）。

## 4. 数据模型与 API（复用为主）

| 用途 | 来源 | 说明 |
|------|------|------|
| 候选 section 向量池 | `GET /api/semantic-boards/:id/persistent-topics/compose-candidates` | 查全量带 embedding 的 section（不限 unassigned），前端算 distance 依赖 embedding |
| 候选 topic 列表 | 列 `status='candidate'` persistent_topic 的现有 API | 侧边栏数据源 |
| 距离/分层/离群/锚点 | `composeReport.ts`：`cosineDistance`/`aggregatePreview`/`distanceTier`/`outlierFlags` | 纯前端，直接复用 |
| 建泳道 | `createManualLane`（`front/app/api/persistentTopics.ts`） | 复用，传 name + sectionIDs |

**数据源决策（已明确，原「技术风险」已解决）**：编辑态数据源 = `compose-candidates` 池（已含 embedding + 现有归属 `persistentTopic{id,label}`，覆盖 unassigned 与 active 全量带向量 section）。按 section id 与 lanes 节点对齐渲染，不依赖 lanes 节点本身带不带 embedding。

**移出可行性（已验证）**：`createManualLane(boardId, label, sectionIds)` 只收 label + section_ids，section 重新指向新 topic 即自动离开原 active 泳道，移出无需额外 API。D8 提示「将从【泳道X】移出」的原泍道名直接读 `candidate.persistentTopic.label`。

## 5. 组件架构

- **废弃**：`ComposePanel.vue` + `viewMode='compose'` 入口。
- **保留**：`composeReport.ts`（被新模式复用）。
- **新增**：
  - `ComposeInlineToolbar.vue` — 顶部浮工具条（名/计数/聚类质量卡/取消/保存）。
  - `ComposeSidebar.vue` — 候选 topic 侧边栏。
- **扩展**：`BoardThreadBrowser.vue`
  - 新增 `composeMode` 状态。
  - section 节点渲染增加 checkbox + distance/tier 标注。
  - active 泳道淡显样式 + 可勾走逻辑。
  - unassigned 泳道头部「新建泳道」按钮。

## 6. 范围外（YAGNI）

- 多条新泳道同时编排（D10：一次一条）。
- 「未来预期」体检卡（v1 本就未实现）。
- 常驻撞车检查卡（改为保存时动作提示）。
- 主题撞车 `crashReport`（centroid 级，区别于成员移出提示）。
- 侧边栏「最近勾选」回退列表（未要求）。

## 7. 后续

设计 review 通过后，按项目规范走 openspec change（`docs/reference/开发执行规范.md` §0.6）：apply 启动跑 `doc-impact.sh suggest+context`，归档前跑 `verify+check-standards.sh`。

---

## 附录 A：关键代码索引（实现速查，免重新探索）

### 前端

**编排宿主** `front/app/features/tags/components/BoardThreadBrowser.vue`
- L33 `viewMode`（timeline/lanes/focus/compose 四态互斥）—— 本期新增 `composeMode` 布尔叠加态，不改 viewMode
- L211 unassigned 泍道硬编码：`{ key:'unassigned', label:'待确认 / 未分类', color:'#64748b', status:'none', topicId:null }`
- L225 `sectionLaneKey(s)` → `s.persistent_topic_id` 空归 `'unassigned'`
- L526 `unassignedCount = sections.filter(s => !s.persistent_topic_id).length`
- L589 unassigned 泍道不进专注视图

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
- `ComposeCandidate { id, embedding, persistentTopicId?, persistentTopic?:{id,label} }` —— **已带现有归属**，D8 提示直接读 `.persistentTopic.label`
- `createManualLane` 只收 label + section_ids —— section 重指新 topic 即自动离开原 active 泍道，移出无需额外 API

### 后端

- `backend-go/internal/topicgraph/handler/daily_report_handler.go`
  - L72-74 路由 `GET /semantic-boards/:id/persistent-topics/compose-candidates`
  - L667-691 `composeCandidates` handler → `Repo.GetComposeCandidates(boardID, days)`
- `backend-go/internal/topicgraph/repository/daily_report_manual_topic.go`
  - L76 `ComposeCandidateSection`；L89 `ComposeCandidatesResponse`；L94+ `GetComposeCandidates` WHERE `embedding IS NOT NULL` + 时间窗，**不限 unassigned**
- candidate topic（侧边栏数据源）`backend-go/internal/topicgraph/repository/daily_report_assignment.go` L127 `candidateTopicSpec` —— L3 auto_new、`lane_tier=l3_new`、`consecutive_hits=1`、status=candidate

### 数据模型要点
- section 归类由 `persistent_topic_id` 决定（null = unassigned）
- `DailyReportSection.LaneTier`：l1_direct / l2_llm / l3_new
- topic status：candidate / active / archived
- 阈值：前端 `matchThreshold`（compose-candidates 响应带回）；后端 `PersistentTopicConfig` + `semantic_board_upgrade_cluster_distance_threshold`
