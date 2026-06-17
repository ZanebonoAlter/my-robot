## Capability

交互层——处理 Raycaster 悬停/点击、BFS 生命线展开（日期窗口约束）、2D 面板叠加、时间范围切换。协调 Vue 状态和 Three.js 场景之间的双向通信。

## API

### InteractionLayer

```typescript
class InteractionLayer {
  constructor(
    scene: TopicWallScene,
    directorCamera: DirectorCamera,
    canvas: HTMLCanvasElement,
    callbacks: InteractionCallbacks
  )

  enable(): void
  disable(): void   // 转场期间调用

  // 外部触发
  setTimeRange(days: number): void
  resetToOverview(): void   // 点击空白区域时调用

  // 拖拽相机时暂停 hover（由 WallCameraControls 调用）
  setHoverSuspended(suspended: boolean): void

  // 完整生命周期模式（spec §面板操作）
  enterLifecycle(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: { start: string; end: string }
  ): void
  exitLifecycle(): void
}
```

> tooltip 点击转发：CSS2D tooltip 的 DOM（`data-card-id`）启用 `pointer-events:auto`，
> InteractionLayer 在 enable() 时给 `scene.css2d.domElement` 注册 click 监听，命中
> `[data-card-id]` 即转发为对应卡片的 `handleCardClick`（解决"点 tooltip 文字落空"）。

> 键盘：ESC 由 Vue 层 `keydown` 监听处理（lifecycle→exitLifecycle / focusing→closePanel /
> idle→close），不在 InteractionLayer 内。

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

> 语义说明：本算法与现有 `graphBfsHighlight.ts` 的 `bfsHighlight` **不同**，不能直接复用。
> `bfsHighlight` 的意图是"高亮连通分量"（带小图/稠密图启发式截断，无日期约束）；
> 本算法的意图是"严格受日期窗口约束的 BFS 生命线"——日期窗口外的节点一律不进入结果。
> 因此独立实现，邻接表构造写成无歧义版本（早期版本曾因 `list` 变量复用导致游离数组 bug）。

```typescript
function bfsLifeline(
  startNodeId: number,
  relations: SectionRelation[],
  nodeMap: Map<number, SectionTimelineNode>,
  dateRange: { start: string; end: string }
): {
  nodes: Set<number>
  edges: Set<string>  // 规范化 "minId-maxId" key（无向，与 relation 方向无关）
  depth: Map<number, number>  // BFS hop count from start（start = 0），用于动画 stagger
} {
  const visited = new Set<number>()
  const edgeKeys = new Set<string>()
  const depth = new Map<number, number>([[startNodeId, 0]])  // BFS hop count
  const queue = [startNodeId]
  visited.add(startNodeId)

  // 预建无向邻接表（Map<number, Set<number>>，风格对齐现有 bfsHighlight）
  const adj = new Map<number, Set<number>>()
  const ensure = (id: number) => {
    let set = adj.get(id)
    if (!set) { set = new Set(); adj.set(id, set) }
    return set
  }
  for (const r of relations) {
    ensure(r.from_id).add(r.to_id)
    ensure(r.to_id).add(r.from_id)
  }
  ensure(startNodeId) // 保证孤立焦点也存在

  // 规范化 edge key 工具：与 relation 的方向无关，避免 BFS 反向遍历时 key miss
  const edgeKey = (a: number, b: number) =>
    a < b ? `${a}-${b}` : `${b}-${a}`

  while (queue.length > 0) {
    const current = queue.shift()!
    const neighbors = adj.get(current)
    if (!neighbors) continue
    const currentDepth = depth.get(current) ?? 0

    for (const neighborId of neighbors) {
      if (visited.has(neighborId)) continue

      const node = nodeMap.get(neighborId)
      if (!node) continue

      // 关键约束: 日期窗口（窗口外节点不进入结果，但仍记录已访问避免重复）
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue

      visited.add(neighborId)
      queue.push(neighborId)
      depth.set(neighborId, currentDepth + 1)
      edgeKeys.add(edgeKey(current, neighborId))
    }
  }

  return { nodes: visited, edges: edgeKeys, depth }
}
```

### 结果与 SectionRelation 的匹配

BFS 返回的 `edges` 是规范化 key（`minId-maxId`）。下游渲染红线时，按同一规范化规则匹配 `relations`：

```typescript
const matchedRelations = relations.filter(r =>
  result.edges.has(edgeKey(r.from_id, r.to_id))
)
```

### BFS 动画序列

```
点击卡片后:

1. 计算 BFS 结果（含 depth: Map，同步，< 1ms）
2. 启动 GSAP Timeline:
   a. 非相关卡片 dim() → stagger 0.02s
   b. 相机 transitionTo(topicFocus) → 并行
   c. BFS 节点按 depth（bfsLifeline 返回）stagger highlight:
      - depth 0: 被点击的节点 → delay 0.1s（立即）
      - depth 1: 一跳邻居 → delay 0.18s (0.1 + 1*0.08)
      - depth 2: 二跳邻居 → delay 0.26s (0.1 + 2*0.08)
      - ...
      - 每个节点的 delay = 0.1 + depth * 0.08
      - 同时点亮选中卡片上方的红色 PointLight（selectionLight）
   d. 对应的红线按 depth 绘制:
      - drawProgress 0→1, duration 0.15s/条
      - 每条线比目标节点提前 0.05s 开始绘制
3. 2D 详情面板在 Timeline 完成后滑入
```

> 实现：`playLifelineAnimation(card, nodeIds, depth)` 的第三参接收 depth Map，
> delay 计算用 `(depth.get(c.data.id) ?? 0) * 0.08`。早期版本曾用数组索引 i 作
> depth 代理（顺序随机），现已改为真正的 BFS 深度。

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
  → mode === 'lifecycle'
    → 不执行 BFS（整个场景已是一条生命周期线）
    → 仅更新 selectionLight + onCardClick（面板更新）

命中红线:
  → 以对端节点为起点，重新 BFS
  → 相机平移到新焦点

命中空白:
  → mode === 'focusing': resetToOverview()
  → mode === 'lifecycle': onBackgroundClick()（交 Vue 决定，由 enterLifecycle/exitLifecycle
    退出，因 lifecycle 退出需 re-fetch timeline，属 Vue 职责）
  → mode === 'idle': onBackgroundClick()
```

> 实现：`processClick` 的背景分支用 `if (mode === 'focusing') resetToOverview()
> else onBackgroundClick()`。早期版本曾用 `if (mode !== 'idle') resetToOverview()`，
> 这会让 lifecycle 模式点空白时错误退出（resetToOverview 不 re-fetch timeline）。

## 2D Detail Panel

### 面板内容

```
┌──────────────────────────────────┐
│ 📁 案件编号 #{{sectionId}}        │
│ ─────────────────                │
│ {{ clusterLabel }}               │
│ {{ articleCount }}篇 · {{ threadCount }}线索  {{ statusLabel }}
│ ─── 汇总 ─────────────           │
│ 共 {{totalArticles}}篇 · {{totalThreads}}线索
│ ● 持续 3  ● 新兴 1  ● 分化 1     │
│                                  │
│ ─── 生命线/完整生命周期 ─────     │
│ ▸ 2026-01-02 霍尔木兹海峡…      │
│     └ 线索A (3篇)               │
│         · 文章1标题 ↗           │
│         · 文章2标题 ↗           │
│ ▸ 2026-01-05 航运恢复…          │
│                                  │
│ [查看完整生命周期]               │
│ [关闭面板 / 返回时间线]          │
└──────────────────────────────────┘
```

> 生命线节点列表用滚动容器（`.tdw-lifeline-list`，max-height: 40vh），不限数量。
> statusLabel 为中文化（emerging→新兴 等）。汇总区统计当前 lifelineNodes 的
> 总文章/线索数 + 状态分布彩点。

### 面板定位

```
详情面板 = 普通 Vue overlay（position: fixed），叠加在 Three.js canvas 上方
（不是 CSS2DRenderer：面板不跟随 3D 对象，用 CSS2DRenderer 是过度设计）

位置策略:
  - 默认: 屏幕右侧固定区域 (right: 2rem, top: 50%, transform: translateY(-50%))
  - 不跟随 3D 对象移动 (固定屏幕位置)
  - 面板宽度: 280px
  - 动画: motion-v 过渡（x: 50, opacity: 0 → enter），由 Vue 组件声明式驱动
     → 不用 gsap，遵循 design.md §Animation Library Split 的 2D/3D 分工

内容更新:
  - BFS 完成后填充数据
  - chainLabel: 从 BFS 节点中提取最短路径的 label 链
  - 状态分布: 统计 BFS 节点的 status
  - 总文章/线索: sum BFS 节点的 article_count / thread_count
```

### Card Tooltip（跟随 3D 卡片，才用 CSS2DRenderer）

```
卡片悬停 tooltip 才用 CSS2DRenderer：
  - 内容: 话题名 + 状态
  - 跟随被悬停卡片的 3D 坐标投射到屏幕
  - 失焦（pointermove 离开）即隐藏

→ 详情面板（固定屏幕位置）≠ tooltip（跟随 3D），两者实现不同：
  详情面板 → 普通 Vue overlay + motion-v
  tooltip  → CSS2DRenderer
```

### 面板操作

```
查看详细线索:
  → 调用 getDailyReportDetail(reportId) 获取 thread 列表
  → 面板内展开线索列表（滚动容器，不限数量；早期版本曾 slice(0,10)）
  → 点击线索 → toggleThreadArticles(thread)：二级展开该线索的文章列表
    （批量 getArticle 取标题，最多 10 篇，超出的显示"还有 N 篇…"）
  → 点击具体文章 → openArticle(articleId)（不再默认取首篇）

查看完整生命周期:
  → 调用 getSectionLifecycle(sectionId)
  → interaction.enterLifecycle()：mode='lifecycle'
    - 迷雾 disable()
    - 清空当前卡片，只渲染 lifecycle 数据
    - 相机 transitionTo(lifecycleFull)
    - 红线从最早节点画到最新节点 (单条时间线)
    - 面板更新为生命周期视图

返回总览（聚焦/生命周期模式下，面板底部按钮）:
  → lifecycle 模式: exitLifecycle() → fog.enable(days) + re-fetch timeline + 相机回 todayFocus
  → focusing 模式: closePanel() → interaction.resetToOverview()（仅关面板，不退出 3D）

> 语义区分：面板内「关闭面板/返回时间线」≠ 顶栏「返回」（close，退出整个 3D 视图）。
> 早期版本面板按钮曾直接调 close（退出 3D），现已拆分。
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
- 详情面板（固定屏幕位置）用普通 Vue overlay + motion-v，不用 CSS2DRenderer；只有卡片悬停 tooltip 用 CSS2DRenderer（跟随 3D 卡片坐标）。详见 §2D Detail Panel
- 完整生命周期模式使用 `getSectionLifecycle`（不限天数），BFS 模式使用 `getBoardSectionTimeline`（受 days 限制），两个 API 不能混用
- 背景点击：lifecycle 模式下不调 resetToOverview（会错误退出且不 re-fetch），而是 onBackgroundClick 交 Vue 处理 exitLifecycle
- BFS 动画的 highlight stagger 必须用 bfsLifeline 返回的 depth Map，不能用 cardGroup.cards 的数组索引（顺序随机，spec §BFS 动画序列）
- 移动端（< 768px）不提供 3D 入口，此 spec 不涉及移动端适配

> ChapterTransition：本 change 当前无 BoardSelector 入口，ChapterTransition 的
> watch(boardId)/wipe/cover DOM 已移除（避免死代码）。ChapterTransition.ts 类文件保留，
> 供后续补 BoardSelector 时复用。转场期间禁用交互的约束暂不生效。
