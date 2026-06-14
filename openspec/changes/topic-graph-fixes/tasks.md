# Tasks: topic-graph-fixes

## 1. 修复日报时间范围

- [x] 1.1 在 `backend-go/internal/topicgraph/repository/daily_report_repository.go` 中移除 `ListReports` 的 30 天上限，保留 `days <= 0` 时默认 7 天
- [x] 1.2 为 `ListReports` 增加 repository 测试，覆盖 `days=0`、`days=7` 和 `days=42`，验证查询起始日期和结果排序
- [x] 1.3 验证 `BoardDailyReportTimeline.vue` 继续按 7 天递增请求，并在切换 board 时重置为 7 天；只在现有行为不满足 spec 时修改前端

## 2. 实现共享有界 BFS 工具

- [x] 2.1 新增 `front/app/features/topic-graph/utils/graphBfsHighlight.ts`，定义泛型 `GraphHighlightEdge<T>`、`bfsHighlight<T>()` 及三个命名常量
- [x] 2.2 实现完整连通分量遍历，以及密集图的 `MAX_HOPS` 与最大节点数双重限制
- [x] 2.3 增加 `graphBfsHighlight.test.ts`，覆盖稀疏分量、密集星形图、超过 4 跳、小图、孤立节点、数字 ID 和环形关系

## 3. 将主题图切换到共享高亮

- [x] 3.1 在 `front/app/features/topic-graph/composables/useTopicGraph.ts` 中标准化 edge 端点 ID，并用 `bfsHighlight` 替换一跳 `highlightedNodeIds` 计算
- [x] 3.2 保持 `relatedEdgeIds` 只包含两端都在高亮集合中的边
- [x] 3.3 增加或更新单元测试，覆盖 A-B-C 多跳高亮、取消选中清空高亮和密集图截断

## 4. 修复 Section Lifecycle API 数据范围

- [x] 4.1 将 `TopicGraphRepository.GetSectionLifecycle` 从一跳查询改为沿 from/to 双向遍历完整连通分量
- [x] 4.2 查询 relations 时限制两端都位于返回 section 集合，避免返回悬空边
- [x] 4.3 使用完整 sections/relations 推导 status 和 ended
- [x] 4.4 增加 repository 测试，覆盖多跳链、分叉、合并、孤立节点、环形关系和连通分量外关系隔离

## 5. 将生命周期面板切换到共享高亮

- [x] 5.1 在 `SectionLifecyclePanel.vue` 中将 `from_id/to_id` 映射为数字 `{ source, target }` 边
- [x] 5.2 使用 computed 缓存 hover 节点的 `bfsHighlight` 结果
- [x] 5.3 节点按集合成员关系高亮，边仅在两端节点都位于集合时高亮
- [x] 5.4 增加组件或工具层测试，覆盖多跳、孤立节点和 hover 清除

## 6. 增加主题图显示控制

- [x] 6.1 在 `TopicGraphCanvas.client.vue` 右上角增加默认折叠的设置面板，提供视图缩放、节点尺寸、标签尺寸和重置视图
- [x] 6.2 节点尺寸 multiplier 仅作用于 `buildNodeObject` 的渲染半径，不修改 view model 的语义 `node.size`
- [x] 6.3 标签尺寸 multiplier 作用于现有 `SpriteText.textHeight` 基准值，不使用 `fontSize` 作为视觉字号
- [x] 6.4 视图缩放通过相机状态实现，并保留 OrbitControls 的滚轮/触控缩放
- [x] 6.5 设置变化时刷新 node object、连线样式或相机状态，不重新请求图数据
- [x] 6.6 不写入 `localStorage`；组件重新创建后恢复默认值

## 7. 启用节点拖拽

- [x] 7.1 删除 `.enableNodeDrag(false)` 或显式启用节点拖拽
- [x] 7.2 增加 E2E 或可观察的交互验证，确认拖拽后节点坐标发生变化

## 8. 验证与文档

- [x] 8.1 在 `backend-go/` 运行 `go test ./internal/topicgraph/repository`
- [x] 8.2 在 `backend-go/` 运行 `golangci-lint run ./...`、`go vet ./internal/topicgraph/...` 和 `go build ./...`
- [x] 8.3 在 `front/` 运行 `pnpm lint` 和 `pnpm test:unit`
- [x] 8.4 通过 Windows cmd 运行 `pnpm exec nuxi typecheck` 和 `pnpm build`
- [x] 8.5 更新 `docs/reference/api/daily-reports.md`，说明 `days` 默认值及不再静默截断到 30 天
- [x] 8.6 更新相关 Topic Graph / Section Lifecycle 用户文档，说明多跳高亮、显示控制和节点拖拽
- [x] 8.7 运行 `openspec validate topic-graph-fixes --strict`
