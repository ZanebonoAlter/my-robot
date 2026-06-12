## Capability

交互层——处理 Raycaster 悬停/点击、BFS 生命线展开（日期窗口约束）、2D 面板叠加、时间范围切换。协调 Vue 状态和 Three.js 场景之间的双向通信。

## API

### InteractionLayer

```typescript
class InteractionLayer {
  constructor(
    scene: TopicWallScene,
    camera: DirectorCamera,
    canvas: HTMLCanvasElement,
    callbacks: InteractionCallbacks
  )

  enable(): void
  disable(): void   // 转场期间调用

  // 外部触发
  setTimeRange(days: number): void
  resetToOverview(): void   // 点击空白区域时调用
}

interface InteractionCallbacks {
  onCardHover(card: PinCard | null): void
  onCardClick(card: PinCard): void
  onStringClick(string: RedString): void
  onBackgroundClick(): void
  onLifelineReady(
    nodes: SectionTimelineNode[],
    edges: SectionRelation[],
    startNode: SectionTimelineNode
  ): void
}
```

### 交互状态

```typescript
type InteractionMode =
  | 'idle'          // 默认，聚焦今天
  | 'focusing'      // BFS 生命线展开中
  | 'lifecycle'     // 完整生命周期视图

interface InteractionState {
  mode: InteractionMode
  focusedNodeId: number | null
  lifelineNodeIds: Set<number>
  lifelineEdgeKeys: Set<string>
  hoveredId: number | null
}
```

## BFS Lifeline Algorithm

```typescript
function bfsLifeline(
  startNodeId: number,
  relations: SectionRelation[],
  nodeMap: Map<number, SectionTimelineNode>,
  dateRange: { start: string; end: string }
): {
  nodes: Set<number>
  edges: Set<string>  // "from-to" key
} {
  const visited = new Set<number>()
  const edgeKeys = new Set<string>()
  const queue = [startNodeId]
  visited.add(startNodeId)

  // 预建邻接表
  const adj = new Map<number, number[]>()
  for (const r of relations) {
    let list = adj.get(r.from_id)
    if (!list) { list = []; adj.set(r.from_id, list) }
    list.push(r.to_id)
    list = adj.get(r.to_id) || []
    if (!adj.has(r.to_id)) adj.set(r.to_id, list)
    adj.get(r.to_id)!.push(r.from_id)
  }

  while (queue.length > 0) {
    const current = queue.shift()!
    const neighbors = adj.get(current) || []

    for (const neighborId of neighbors) {
      if (visited.has(neighborId)) continue

      const node = nodeMap.get(neighborId)
      if (!node) continue

      // 关键约束: 日期窗口
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue

      visited.add(neighborId)
      queue.push(neighborId)
      edgeKeys.add(`${current}-${neighborId}`)
    }
  }

  return { nodes: visited, edges: edgeKeys }
}
```

### BFS 动画序列

```
点击卡片后:

1. 计算 BFS 结果 (同步，< 1ms)
2. 启动 GSAP Timeline:
   a. 非相关卡片 dim() → stagger 0.02s
   b. 相机 transitionTo(topicFocus) → 并行
   c. BFS 节点按 depth 排序:
      - depth 0: 被点击的节点 → 立即 highlight()
      - depth 1: 一跳邻居 → delay 0.08s
      - depth 2: 二跳邻居 → delay 0.16s
      - ...
      - 每个 depth 的节点 stagger 0.04s
   d. 对应的红线按 depth 绘制:
      - drawProgress 0→1, duration 0.15s/条
      - 每条线比目标节点提前 0.05s 开始绘制
3. 2D 详情面板在 Timeline 完成后滑入
```

## Raycaster Interaction

### 悬停

```
事件: pointermove
处理:
  1. Raycaster 从相机投射
  2. 检测与 PinCard.paperMesh 的交叉
  3. 如果命中:
     - 调用 card.elevate()
     - 高亮直接邻居的红线
     - 显示 CSS2D tooltip (话题名 + 状态)
  4. 如果未命中:
     - 当前 hovered card.settle()
     - 取消红线高亮
     - 隐藏 tooltip

节流: requestAnimationFrame 驱动，不直接在 pointermove 里计算
```

### 点击

```
事件: pointerdown + pointerup (区分 click vs drag)
判断: down 和 up 之间位移 < 5px 视为 click

命中卡片:
  → mode === 'idle'
    → 执行 BFS，进入 'focusing' 模式
  → mode === 'focusing'
    → 同一卡片: resetToOverview()
    → 不同卡片: 重新执行 BFS

命中红线:
  → 以对端节点为起点，重新 BFS
  → 相机平移到新焦点

命中空白:
  → resetToOverview()
```

## 2D Detail Panel

### 面板内容

```
┌──────────────────────────────────┐
│ 📁 案件编号 #{{sectionId}}        │
│ ─────────────────                │
│ 线索链: {{ chainLabel }}          │
│ 时间跨度: {{ startDate }} - {{ endDate }}
│ 总文章: {{ totalArticles }}篇     │
│ 总线索: {{ totalThreads }}条      │
│                                  │
│ ─── 状态分布 ─────────────        │
│ ● 持续 3  ● 新兴 1  ● 分化 1     │
│                                  │
│ ▸ 查看详细线索                   │
│ ▸ 查看完整生命周期               │
│ ▸ 返回总览                       │
└──────────────────────────────────┘
```

### 面板定位

```
CSS2DRenderer 叠加在 Three.js canvas 上方

位置策略:
  - 默认: 屏幕右侧固定区域 (right: 2rem, top: 50%, transform: translateY(-50%))
  - 不跟随 3D 对象移动 (固定屏幕位置)
  - 面板宽度: 280px
  - 动画: gsap.from(panel, { x: 50, opacity: 0, duration: 0.3 })

内容更新:
  - BFS 完成后填充数据
  - chainLabel: 从 BFS 节点中提取最短路径的 label 链
  - 状态分布: 统计 BFS 节点的 status
  - 总文章/线索: sum BFS 节点的 article_count / thread_count
```

### 面板操作

```
查看详细线索:
  → 调用 getDailyReportDetail(reportId) 获取 thread 列表
  → 面板内展开线索列表 (max 10 条，每条显示标题 + 文章数)
  → 点击线索 → openArticlePreview(articleId)

查看完整生命周期:
  → 调用 getSectionLifecycle(sectionId)
  → mode 切换为 'lifecycle'
  - 迷雾 disable()
  - 清空当前卡片，只渲染 lifecycle 数据
  - 相机 transitionTo(lifecycleFull)
  - 红线从最早节点画到最新节点 (单条时间线)
  - 面板更新为生命周期视图

返回总览:
  → resetToOverview()
```

## Time Range Switching

### 切换流程

```
用户点击 [7天] / [14天] / [30天] / [60天]

1. 如果 mode === 'focusing':
   - 保留 focusedNodeId
   - 用新时间范围重新 BFS
   - 更新高亮

2. scene.loadBoardData() 加载新数据
3. fog.animateToDensity(newDensity, 0.8)
4. 相机:
   - 'idle': snapTo(todayFocus) 然后微调看迷雾边缘
   - 'focusing': transitionTo(topicFocus) 保持聚焦
   - 'lifecycle': transitionTo(lifecycleFull)
5. 新卡片从迷雾边缘 fade in (stagger)
6. 旧卡片如果在窗口外 → 消失在迷雾中
```

## Vue ↔ Three.js Bridge

### TopicDetectiveWall.client.vue

```typescript
// Vue 组件职责:
// 1. 管理 DOM (canvas + overlay)
// 2. 调用 API 获取数据
// 3. 把数据传给 TopicWallScene
// 4. 接收 InteractionLayer 回调，更新 Vue 响应式状态
// 5. 渲染 2D overlay (BoardSelector, DaysRange, DetailPanel, ChapterTitle)

// 数据获取
const boardId = ref<number>(props.selectedBoardId)
const days = ref(7)
const { data } = await getBoardSectionTimeline(boardId.value, days.value)

// 场景初始化
const scene = new TopicWallScene(canvasRef.value!)
const interaction = new InteractionLayer(scene, directorCamera, canvasRef.value!, {
  onCardClick: (card) => {
    focusedNode.value = card.data
    showDetailPanel.value = true
  },
  onLifelineReady: (nodes, edges, start) => {
    // 更新面板数据
    detailData.value = computeDetailData(nodes, edges, start)
  },
  onBackgroundClick: () => {
    focusedNode.value = null
    showDetailPanel.value = false
  }
})

// 板块切换
function switchBoard(newBoardId: number) {
  chapterTransition.play({
    name: boards.find(b => b.id === newBoardId)!.name,
    dateRange: formatDateRange(days.value),
    topicCount: /* ... */
  })
}
```

## Constraints

- Raycaster 检测在 requestAnimationFrame 中执行，不在 pointermove 回调中直接计算
- BFS 计算是同步的，数据量 < 100 节点时耗时 < 1ms，不需要 Web Worker
- CSS2D 面板不跟随 3D 对象，使用固定屏幕位置（避免旋转/缩放时面板乱跑）
- 完整生命周期模式使用 `getSectionLifecycle`（不限天数），BFS 模式使用 `getBoardSectionTimeline`（受 days 限制），两个 API 不能混用
- 转场期间 InteractionLayer.disable()，转场结束后 enable()
- 移动端（< 768px）不提供 3D 入口，此 spec 不涉及移动端适配
