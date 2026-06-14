# Design: topic-graph-fixes

## Context

当前实现存在几个彼此关联但边界清晰的问题：

- `TopicGraphRepository.ListReports` 将 `days > 30` 强制改为 30，而 `BoardDailyReportTimeline` 会持续执行 `days += 7`。前端无法判断请求被截断。
- `useTopicGraph` 的 `highlightedNodeIds` 只收集 focus 节点和直接相邻节点。
- `SectionLifecyclePanel` 的 hover 高亮也只检查直接邻居。
- `TopicGraphRepository.GetSectionLifecycle` 实际只查询一跳邻居，与主 `section-lifecycle` spec 所要求的“双向扩展并返回所有相关 sections”不一致。
- `TopicGraphCanvas.client.vue` 已启用 OrbitControls，但没有显式缩放控件；节点拖拽通过 `.enableNodeDrag(false)` 被关闭。
- `three-spritetext` 的 `textHeight` 决定标签在 3D 场景中的视觉高度，`fontSize` 只影响纹理分辨率。

## Goals

- 让日报 `days` 参数按请求值生效，不再静默截断到 30 天。
- 为主题图和生命周期面板提供一致、可测试的多跳高亮规则。
- 让生命周期 API 返回起始 section 所在的完整连通分量。
- 在密集、低直径图中保证高亮不会退化为全图高亮。
- 提供明确的相机缩放、节点尺寸、标签尺寸控制，并启用节点拖拽。

## Non-Goals

- 引入 cursor/offset 分页协议。
- 持久化图显示设置。
- 修改 section relation 的生成和匹配算法。
- 替换 `3d-force-graph`、Three.js 或现有生命周期 SVG 渲染方案。
- 允许用户在 UI 中修改高亮算法常量。

## Decisions

### D1: 移除 ListReports 的 30 天静默上限

`ListReports(boardID, days)` 保留 `days <= 0` 时默认 7 天的规则，但不再将正数天数限制为 30。查询已经按 `semantic_board_id` 和 `period_date` 过滤并排序，适合当前单用户、按 7 天逐步扩展的使用方式。

本次只修改板块日报列表使用的 `ListReports`。`ListReportsForAllBoards` 没有当前调用方，不作为本次 UI bug 的必要修改；若后续启用该接口，应单独确定其数据量和分页策略。

### D2: 使用“完整分量或有界 BFS”规则

共享工具先计算 focus 节点的完整无向连通分量：

```text
component = bfs(focusId)

if totalNodes <= SMALL_GRAPH_NODE_LIMIT:
  return component

if component.size < totalNodes * COMPONENT_THRESHOLD:
  return component

maxNodes = max(1, floor(totalNodes * COMPONENT_THRESHOLD))
return bfs(focusId, maxHops=MAX_HOPS, maxNodes=maxNodes)
```

常量：

- `COMPONENT_THRESHOLD = 0.4`
- `MAX_HOPS = 4`
- `SMALL_GRAPH_NODE_LIMIT = 8`

仅限制跳数不足以防止密集图全亮：完全图或低直径图在一两跳内即可覆盖全部节点。因此密集分量的第二次 BFS 必须同时限制最大跳数和最大节点数。

边按输入顺序建立邻接表；有界 BFS 结果因此稳定且可测试，但不承诺高亮节点之间存在业务优先级排序。

### D3: 共享工具使用泛型节点 ID 和标准边

新增：

```typescript
export interface GraphHighlightEdge<T extends string | number> {
  source: T
  target: T
}

export function bfsHighlight<T extends string | number>(
  focusId: T,
  edges: GraphHighlightEdge<T>[],
  totalNodes: number,
): Set<T>
```

- 主题图将可能被 `3d-force-graph` 替换为对象的 `source/target` 解析为字符串 ID 后调用。
- 生命周期面板将 `from_id/to_id` 映射为数字 `source/target` 后调用。
- focus 节点即使没有出现在任何边中，也必须包含在返回集合中。

### D4: relatedEdgeIds 只包含高亮子图内部边

主题图的 `relatedEdgeIds` 保持现有契约：仅返回两端节点都在高亮集合中的边。生命周期面板的边高亮遵循相同规则，而不是仅判断边是否直接连接 hover 节点。

### D5: 生命周期 API 返回完整连通分量

`GetSectionLifecycle(sectionID)` 沿 `daily_report_section_relations` 双向遍历，收集起始 section 所在的完整连通分量。实现可使用 PostgreSQL recursive CTE，也可使用分批迭代查询，但必须满足：

- 孤立 section 返回自身和空 relations。
- 返回的 relations 两端都必须属于返回的 sections。
- 不把仅一端属于结果集的边带入响应。
- status/ended 推导使用完整结果集和内部 relations。

这是现有主 spec 已要求但代码尚未实现的行为，不应继续作为 Non-Goal。

### D6: 显示控制保留语义数据，缩放仅作用于渲染

显示设置放在 `TopicGraphCanvas.client.vue`，不修改 `buildTopicGraphViewModel` 产生的语义 `node.size`：

- **视图缩放**：控制相机与图中心的相对距离；OrbitControls 的滚轮/触控缩放继续可用。
- **节点尺寸**：在 `buildNodeObject` 中乘以本地 multiplier。
- **标签尺寸**：对现有 trunk/branch/peripheral `textHeight` 基准值乘以本地 multiplier。
- **连线宽度**：随节点尺寸 multiplier 做有限比例缩放，保持视觉一致。
- **重置视图**：恢复默认缩放和尺寸值。

滑块变化后刷新 Three.js node object 和 link 样式。设置不写入 `localStorage`。

### D7: 启用节点拖拽

删除 `.enableNodeDrag(false)` 或显式设置为 `true`。拖拽是图布局交互，不增加额外开关。

## Risks & Trade-offs

| Risk | Mitigation |
|------|------------|
| 完整生命周期连通分量比一跳结果更大 | repository 测试覆盖分叉、合并、环和孤立节点；仅返回同一连通分量内部关系 |
| 高频 hover 重复构建邻接表 | 组件按 edges/relations 计算邻接结构或缓存高亮 computed；当前图规模可控 |
| 密集图有界 BFS 的边顺序影响截断结果 | 保持输入顺序以确保稳定；本次不引入业务排序规则 |
| 大 `days` 查询返回更多日报 | 当前 UI 每次只增加 7 天；增加 `days > 30` repository 回归测试并监控实际响应量 |
| 调整 Three.js 对象需要显式刷新 | 将刷新逻辑集中在 canvas 内，并以组件测试或 E2E 验证控件生效 |

## Affected Files

- `backend-go/internal/topicgraph/repository/daily_report_repository.go`
- `backend-go/internal/topicgraph/repository/*_test.go`
- `front/app/features/topic-graph/utils/graphBfsHighlight.ts`
- `front/app/features/topic-graph/utils/graphBfsHighlight.test.ts`
- `front/app/features/topic-graph/composables/useTopicGraph.ts`
- `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue`
- `front/app/features/tags/components/SectionLifecyclePanel.vue`
- 相关前端单元测试和 `front/tests/e2e/topic-graph.spec.ts`
