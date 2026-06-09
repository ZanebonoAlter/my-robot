## Purpose

BFS 连通分支高亮能力：从选中/悬停节点出发沿边执行广度优先搜索，高亮整个连通分支。当连通分支过大（>= 40% 总节点数）时退回固定跳数限制，防止全图亮起。供 TopicGraphPage 和 SectionLifecyclePanel 共用。

## Requirements

### Requirement: BFS 连通分支高亮算法

系统 SHALL 提供共享工具函数 `bfsHighlight(focusId, edges, totalNodes)`，执行以下逻辑：

1. 从 `focusId` 出发，将 `edges` 视为无向图构建邻接表
2. 执行 BFS，收集可达节点集合 `component`
3. 如果 `len(component) < totalNodes * 0.4` → 返回 `component`（全高亮）
4. 如果 `len(component) >= totalNodes * 0.4` → 重新执行 BFS，限制最大跳数 4，返回截断后的集合

常量 SHALL 导出：`COMPONENT_THRESHOLD = 0.4`，`MAX_HOPS = 4`。

#### Scenario: 稀疏图高亮整个连通分支

- **WHEN** 图有 100 个节点，focusNode 的连通分支包含 25 个节点（25% < 40%）
- **THEN** `bfsHighlight` SHALL 返回全部 25 个节点的 ID

#### Scenario: 密集图退回 4 跳限制

- **WHEN** 图有 100 个节点，focusNode 的连通分支包含 60 个节点（60% >= 40%）
- **THEN** `bfsHighlight` SHALL 返回从 focusNode 出发最多 4 跳可达的节点集合

#### Scenario: 孤立节点

- **WHEN** focusNode 无任何边连接
- **THEN** `bfsHighlight` SHALL 返回仅包含 focusNode 的集合

#### Scenario: 小图阈值不触发

- **WHEN** 图有 5 个节点全部连通（连通分支 5 个 = 100% >= 40%）
- **THEN** `bfsHighlight` SHALL 退回 4 跳限制，返回全部 5 个节点（4 跳足以覆盖全部）

### Requirement: TopicGraphPage 使用 BFS 高亮

`TopicGraphPage.vue` 的 `highlightedNodeIds` 计算属性 SHALL 调用 `bfsHighlight` 替代当前的 1-hop 逻辑。`relatedEdgeIds` SHALL 返回两端节点均在高亮集合中的边。

#### Scenario: 选中话题高亮多跳连通分支

- **WHEN** 用户选中话题节点 A，A 经边连接 B，B 经边连接 C（共 3 个节点，图总节点 50）
- **THEN** `highlightedNodeIds` SHALL 包含 [A, B, C]，`relatedEdgeIds` SHALL 包含 A-B 和 B-C 两条边

#### Scenario: 取消选中清除高亮

- **WHEN** 用户取消选中（selectedTopicSlug 和 selectedKeywordSlug 均为空）
- **THEN** `highlightedNodeIds` SHALL 返回空数组

### Requirement: SectionLifecyclePanel 使用 BFS 高亮

`SectionLifecyclePanel.vue` 的 `isNodeHighlighted` 和 `isEdgeHighlighted` SHALL 使用 `bfsHighlight` 计算高亮集合。hover 变化时重新计算，结果缓存为 computed 属性。

#### Scenario: 悬停 section 高亮上下游

- **WHEN** 面板显示 section #50 → #60 → #70 的链式关系，用户悬停 #50
- **THEN** #50、#60、#70 SHALL 全部高亮，边 #50-#60 和 #60-#70 SHALL 高亮

#### Scenario: 悬停无关联 section

- **WHEN** 面板显示孤立 section #20（无 relation），用户悬停 #20
- **THEN** 仅 #20 高亮，无边高亮
