## Why

版块日报和话题总览存在多个影响可用性的 bug：(1) 时间筛选不生效（后端硬顶 30 天无前端反馈）；(2) hover 高亮只显示一跳节点，应高亮整个 BFS 连通分支（带阈值截断防全图亮起）；(3) 话题总览无法缩放/拖拽调整节点大小和字体；(4) 话题生命周期面板同样的高亮问题。

## What Changes

- 修复时间筛选：后端移除 30 天硬顶限制或增加前端提示，确保 7 天、14 天等筛选真正生效
- hover 高亮改为 BFS 连通分支遍历：
  - 从选中节点出发沿边 BFS 扩展
  - 如果连通分支节点数 < 总节点数的 40%，全高亮
  - 超过阈值则退回 4 跳限制
  - 阈值和跳数后续可配置
- 话题总览渲染增强：
  - 全局缩放控制（3D force-graph 已有 OrbitControls，暴露配置）
  - 节点大小可配置（通过 UI 控件调整 `buildNodeSize` 的缩放因子）
  - 字体大小可调（`three-spritetext` 的 `fontSize` 属性增加缩放因子）
  - 节点拖拽布局（3D force-graph 支持但可能未启用）
- 话题生命周期面板（`SectionLifecyclePanel.vue`）应用相同的 BFS 高亮逻辑

## Capabilities

### New Capabilities

- `topic-graph-highlight-bfs`: 基于 BFS 连通分支的话题图高亮能力，支持阈值截断防全图亮起

### Modified Capabilities

- `section-lifecycle`: 话题生命周期面板高亮逻辑升级为 BFS 连通分支
- `board-narrative-timeline`: 日报时间筛选修复，移除后端 30 天硬顶

## Impact

- `front/app/features/topic-graph/components/TopicGraphPage.vue`：`highlightedNodeIds` 计算逻辑改为 BFS
- `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue`：暴露缩放/字体大小/节点大小配置
- `front/app/features/topic-graph/utils/topicGraphCanvasLinks.ts`：配合 BFS 高亮调整
- `front/app/features/tags/components/SectionLifecyclePanel.vue`：高亮逻辑升级
- `front/app/features/tags/components/BoardDailyReportTimeline.vue`：时间筛选修复
- `backend-go/internal/domain/daily_report/repository.go`：`ListReports` 移除 30 天硬顶或增加分页
