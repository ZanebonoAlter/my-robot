## Why

主题图和板块日报存在三类可用性问题：

1. `BoardDailyReportTimeline` 每次将 `days` 增加 7，但后端 `ListReports` 静默截断到 30 天，导致“加载更早”在 30 天后看似成功、实际不再返回更早数据。
2. 主题图当前只高亮选中节点及其一跳邻居；`SectionLifecyclePanel` 同样只高亮一跳，而且生命周期 API 目前也只返回一跳邻居，无法表达完整上下游关系。
3. 主题图缺少明确的视图缩放、节点尺寸和标签尺寸控制，节点拖拽还被显式禁用，较大图谱难以阅读和整理。

## What Changes

- 修复板块日报“加载更早”：
  - 移除 `ListReports(boardID, days)` 的 30 天静默上限。
  - 保留 `days <= 0` 时默认 7 天的行为。
  - 前端继续每次增加 7 天并重新请求完整时间范围。
- 新增共享图高亮工具：
  - 将边标准化为泛型 `{ source, target }` 无向图。
  - 稀疏连通分量小于总节点数 40% 时，高亮完整连通分量。
  - 密集连通分量同时受最大 4 跳和最多 40% 节点数约束，避免低直径图在 4 跳内仍点亮全图。
  - 小图（不超过 8 个节点）允许高亮完整连通分量。
- 修复 Section Lifecycle 数据范围：
  - 后端沿 `daily_report_section_relations` 的 from/to 方向双向遍历，返回起始 section 所在的完整连通分量及其内部关系。
  - 前端在完整数据集上复用共享高亮工具。
- 增强主题图显示控制：
  - 提供相机缩放、节点尺寸和标签视觉高度控制。
  - 标签视觉大小调整 `SpriteText.textHeight`，不使用仅影响纹理分辨率的 `fontSize`。
  - 启用 `3d-force-graph` 节点拖拽。
  - 设置仅在当前组件生命周期内生效，不持久化。

## Capabilities

### New Capabilities

- `topic-graph-highlight-bfs`: 共享的连通分量高亮能力，密集图同时受跳数和节点数量约束。
- `topic-graph-display-controls`: 主题图相机缩放、节点尺寸、标签尺寸及节点拖拽能力。

### Modified Capabilities

- `daily-report-system`: 日报列表查询不再将 `days` 静默截断到 30 天。
- `section-lifecycle`: 生命周期 API 返回完整连通分量，面板使用共享 BFS 高亮。

## Impact

- `backend-go/internal/topicgraph/repository/daily_report_repository.go`
  - `ListReports` 移除 30 天上限。
  - `GetSectionLifecycle` 从一跳查询改为完整连通分量查询。
- `front/app/features/topic-graph/composables/useTopicGraph.ts`
  - 主题图高亮从一跳计算改为共享 BFS 工具。
- `front/app/features/topic-graph/utils/graphBfsHighlight.ts`
  - 新增泛型 BFS 高亮工具及阈值常量。
- `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue`
  - 新增显示控制，应用节点/标签/连线缩放，并启用节点拖拽。
- `front/app/features/tags/components/SectionLifecyclePanel.vue`
  - 将生命周期 relation 映射为标准边并复用共享高亮工具。
- `front/app/features/tags/components/BoardDailyReportTimeline.vue`
  - 继续使用递增 `days` 的加载方式；无需引入另一套分页协议。
