# DAG 可视化实现计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 用 d3-dag Sugiyama 布局重写 SectionLifecyclePanel（垂直 DAG）和 BoardThreadBrowser（水平 DAG），实现叙事 split/merge 可视化。

**Architecture:** 新增 `useDagLayout` composable 封装 d3-dag 布局计算，两个组件共享该 composable 并各自实现 SVG 渲染。布局引擎只算坐标，渲染完全由 Vue 模板控制。

**Tech Stack:** d3-dag ^1.2.1 (已安装), Vue 3 Composition API, SVG

---

## Task 1: 创建 useDagLayout composable

**Files:**
- Create: `front/app/composables/useDagLayout.ts`

**Step 1: 创建 composable 文件**

```typescript
// front/app/composables/useDagLayout.ts
import { graphConnect, sugiyama } from 'd3-dag'

export interface DagNodeInput {
  id: number | string
  [key: string]: unknown
}

export interface DagEdgeInput {
  from: number | string
  to: number | string
  [key: string]: unknown
}

export interface PositionedNode<T extends DagNodeInput = DagNodeInput> {
  data: T
  x: number
  y: number
}

export interface EdgePath<T extends DagEdgeInput = DagEdgeInput> {
  data: T
  points: Array<{ x: number; y: number }>
  path: string
}

export type LayoutDirection = 'TB' | 'LR'

interface DagLayoutResult<T extends DagNodeInput, E extends DagEdgeInput> {
  nodes: PositionedNode<T>[]
  edges: EdgePath<E>[]
  width: number
  height: number
}

/**
 * 将 d3-dag 布局应用于节点和边数据
 *
 * @param nodes - 节点数组（至少包含 id）
 * @param edges - 边数组（至少包含 from/to）
 * @param direction - TB=从上到下（SectionLifecycle），LR=从左到右（BoardThreadBrowser）
 * @param nodeSize - [width, height] 节点尺寸，影响间距
 * @param gap - [x, y] 节点间距
 */
export function useDagLayout<
  T extends DagNodeInput = DagNodeInput,
  E extends DagEdgeInput = DagEdgeInput,
>(
  nodes: T[],
  edges: E[],
  direction: LayoutDirection = 'TB',
  nodeSize: [number, number] = [1, 1],
  gap: [number, number] = [1, 1],
): DagLayoutResult<T, E> | null {
  if (nodes.length === 0) return null

  // 构建边 ID 对
  const nodeIdSet = new Set(nodes.map(n => n.id))
  const validEdges = edges.filter(e => nodeIdSet.has(e.from) && nodeIdSet.has(e.to))
  const edgePairs = validEdges.map(e => [String(e.from), String(e.to)] as [string, string])

  // 孤立节点（无边连接）也要加入 graph，graphConnect 会自动处理
  // 为孤立节点添加自环占位不行——d3-dag 不允许自环
  // 解决：graphConnect 只处理有边的节点，孤立节点手动处理
  const connectedIds = new Set<string>()
  for (const [f, t] of edgePairs) {
    connectedIds.add(f)
    connectedIds.add(t)
  }

  // 构建数据 ID → 原始数据映射
  const dataMap = new Map<string, T>()
  for (const n of nodes) {
    dataMap.set(String(n.id), n)
  }

  // 边数据映射 "from->to" → edge data
  const edgeDataMap = new Map<string, E>()
  for (const e of validEdges) {
    edgeDataMap.set(`${e.from}->${e.to}`, e)
  }

  // 对有边连接的节点执行 d3-dag 布局
  let layoutNodes: PositionedNode<T>[] = []
  let layoutEdges: EdgePath<E>[] = []
  let layoutWidth = 0
  let layoutHeight = 0

  if (edgePairs.length > 0) {
    const builder = graphConnect()
    const graph = builder(edgePairs)

    const layout = sugiyama()
      .nodeSize(nodeSize)
      .gap(gap as [number, number])

    const result = layout(graph)

    layoutWidth = result.width
    layoutHeight = result.height

    for (const node of graph.nodes()) {
      const rawId = node.data as string
      const original = dataMap.get(rawId)
      if (!original) continue

      let px = node.x ?? 0
      let py = node.y ?? 0

      // LR 方向：交换 x/y
      if (direction === 'LR') {
        ;[px, py] = [py, px]
      }

      layoutNodes.push({ data: original, x: px, y: py })
    }

    for (const link of graph.links()) {
      const srcId = link.source.data as string
      const tgtId = link.target.data as string
      const edgeKey = `${srcId}->${tgtId}`
      const edgeData = edgeDataMap.get(edgeKey)
      if (!edgeData) continue

      const rawPoints = link.points.map(p => {
        let px = p[0]
        let py = p[1]
        if (direction === 'LR') {
          ;[px, py] = [py, px]
        }
        return { x: px, y: py }
      })

      // 确保 points 至少有 2 个点（起点和终点）
      if (rawPoints.length < 2) continue

      layoutEdges.push({
        data: edgeData,
        points: rawPoints,
        path: buildSvgPath(rawPoints),
      })
    }
  }

  // 处理孤立节点
  const positionedIds = new Set(layoutNodes.map(n => String(n.data.id)))
  const orphans = nodes.filter(n => !positionedIds.has(String(n.id)) && !connectedIds.has(String(n.id)))

  if (orphans.length > 0) {
    // 孤立节点放在布局下方（TB）或右方（LR），按 y 分行
    const startX = 0
    const startY = direction === 'TB'
      ? layoutHeight + gap[1]
      : layoutWidth + gap[0]

    for (let i = 0; i < orphans.length; i++) {
      const n = orphans[i]
      if (direction === 'TB') {
        layoutNodes.push({
          data: n,
          x: startX + i * (nodeSize[0] + gap[0]),
          y: startY + i * (nodeSize[1] + gap[1]),
        })
      } else {
        layoutNodes.push({
          data: n,
          x: startY + i * (nodeSize[0] + gap[0]),
          y: startX + i * (nodeSize[1] + gap[1]),
        })
      }
    }
  }

  if (direction === 'LR') {
    ;[layoutWidth, layoutHeight] = [layoutHeight, layoutWidth]
  }

  return {
    nodes: layoutNodes,
    edges: layoutEdges,
    width: layoutWidth,
    height: layoutHeight,
  }
}

/**
 * 从控制点生成 SVG path（三次贝塞尔曲线）
 */
function buildSvgPath(points: Array<{ x: number; y: number }>): string {
  if (points.length < 2) return ''

  if (points.length === 2) {
    // 直线
    return `M${points[0].x},${points[0].y} L${points[1].x},${points[1].y}`
  }

  // 多个控制点：使用 cubic bezier
  let d = `M${points[0].x},${points[0].y}`
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]
    const curr = points[i]
    const midX = (prev.x + curr.x) / 2
    const midY = (prev.y + curr.y) / 2
    d += ` C${midX},${prev.y} ${midX},${curr.y} ${curr.x},${curr.y}`
  }
  return d
}
```

**Step 2: 验证 d3-dag 导入无误**

Run: `cd front && pnpm exec nuxi typecheck 2>&1 | head -20`

注意：由于 typecheck 需要在 Windows cmd 运行，可以先 `cd front && pnpm build` 验证。但此处只需检查 d3-dag 是否能导入：
```bash
cd front && node -e "const { graphConnect, sugiyama } = require('d3-dag'); console.log('d3-dag OK', typeof graphConnect, typeof sugiyama)"
```

**Step 3: Commit**

```bash
git add front/app/composables/useDagLayout.ts
git commit -m "feat: add useDagLayout composable wrapping d3-dag Sugiyama layout"
```

---

## Task 2: 重写 SectionLifecyclePanel.vue（垂直 DAG）

**Files:**
- Rewrite: `front/app/features/tags/components/SectionLifecyclePanel.vue`

**设计要点：**
- 使用 `useDagLayout` 做 TB（从上到下）布局
- SVG 渲染 DAG，贝塞尔曲线连线
- 保留：面板位置（fixed right 320px）、slide transition、skeleton loading、节点点击导航、当前话题高亮
- 新增：split/merge 的分支/合并视觉效果、ended 节点透明度降低 + 灰色边框
- 去掉：线性 dot+connector 渲染

**Step 1: 重写组件**

完整替换 `SectionLifecyclePanel.vue` 的 `<script setup>` 和 `<template>` 和 `<style>`。

关键设计决策：
1. **SVG 画布**：面板宽度 320px，SVG 占满宽度，高度根据布局自适应（可滚动）
2. **节点渲染**：SVG group 包含圆点（颜色按 status）+ 文字（日期、标题、状态 badge、文章数）
3. **边渲染**：SVG path（贝塞尔曲线），颜色 rgba 白色低透明度
4. **当前节点**：高亮（亮色背景 + 更大圆点 + glow 效果）
5. **ended 节点**：`opacity: 0.5` + 灰色虚线边框
6. **点击交互**：点击节点 emit `navigate`
7. **节点尺寸**：宽 ~260px（320px - padding），高 ~60px（日期+标题+meta）

节点尺寸映射到 d3-dag 的 nodeSize：
- TB 方向：nodeSize = [260, 60]（每个节点占 260 宽、60 高）
- 但 d3-dag 的 nodeSize 单位是布局单位，实际像素渲染时需要乘以缩放系数
- 简化方案：用 nodeSize = [1, 1]，gap = [1, 1]，然后渲染时乘以实际像素尺寸

**实际像素映射策略：**
- d3-dag 布局输出 x/y 坐标
- 节点宽 = 260px，节点高 = 56px
- 间距：水平 20px，垂直 12px
- nodeSize 参数设为 [260, 56]，gap 设为 [20, 12]
- SVG viewBox 根据布局结果的 width/height 设定
- 或者更简单：用 nodeSize=[1,1] gap=[1,1]，然后在渲染时用缩放因子乘以 x/y

选择更简单的方案：**nodeSize=[1,1], gap=[1,1]**，渲染时 `x * colWidth` 和 `y * rowHeight`。

```vue
<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useDailyReportsApi, type SectionLifecycleNode, type SectionRelation } from '~/api/dailyReports'
import { useDagLayout } from '~/composables/useDagLayout'

const props = defineProps<{
  sectionId: number
  visible: boolean
}>()

const emit = defineEmits<{
  close: []
  navigate: [node: SectionLifecycleNode]
}>()

const { getSectionLifecycle } = useDailyReportsApi()

const sections = ref<SectionLifecycleNode[]>([])
const relations = ref<SectionRelation[]>([])
const loading = ref(false)
const error = ref(false)

// --- Status styling ---

const statusColors: Record<string, string> = {
  emerging: '#34d399',
  continuing: '#60a5fa',
  split: '#fb923c',
  merge: '#c084fc',
  ending: '#9ca3af',
}

const statusBadgeClasses: Record<string, string> = {
  emerging: 'bg-emerald-500/20 text-emerald-400',
  continuing: 'bg-blue-500/20 text-blue-400',
  split: 'bg-orange-500/20 text-orange-400',
  merge: 'bg-purple-500/20 text-purple-400',
  ending: 'bg-gray-500/20 text-gray-400',
}

const statusLabel: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

const weekDays = ['日', '一', '二', '三', '四', '五', '六']

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}月${d.getDate()}日 周${weekDays[d.getDay()]}`
}

// --- DAG Layout ---

const NODE_W = 260
const NODE_H = 56
const GAP_X = 24
const GAP_Y = 16
const PADDING = 16

const dagResult = computed(() => {
  if (sections.value.length === 0) return null

  const nodes = sections.value.map(s => ({ id: s.id, ...s }))
  const edges = relations.value.map(r => ({ from: r.from_id, to: r.to_id, ...r }))

  const result = useDagLayout(nodes, edges, 'TB', [1, 1], [1, 1])
  return result
})

// 缩放：将布局坐标映射到像素
const svgWidth = computed(() => {
  if (!dagResult.value) return 288
  return (dagResult.value.width + 1) * (NODE_W + GAP_X) + PADDING * 2
})

const svgHeight = computed(() => {
  if (!dagResult.value) return 200
  return (dagResult.value.height + 1) * (NODE_H + GAP_Y) + PADDING * 2
})

function nodePos(node: { x: number; y: number }) {
  return {
    x: node.x * (NODE_W + GAP_X) + PADDING,
    y: node.y * (NODE_H + GAP_Y) + PADDING,
  }
}

function edgePathScaled(points: Array<{ x: number; y: number }>) {
  return points.map(p => ({
    x: p.x * (NODE_W + GAP_X) + PADDING + NODE_W / 2,
    y: p.y * (NODE_H + GAP_Y) + PADDING + NODE_H / 2,
  }))
}

// --- Data loading ---

let fetchId = 0

async function fetchLifecycle() {
  loading.value = true
  error.value = false
  const currentFetch = ++fetchId
  try {
    const res = await getSectionLifecycle(props.sectionId)
    if (currentFetch !== fetchId) return
    if (res.success && res.data) {
      sections.value = res.data.sections || []
      relations.value = res.data.relations || []
    } else {
      error.value = true
    }
  } catch {
    if (currentFetch !== fetchId) return
    error.value = true
  } finally {
    if (currentFetch === fetchId) {
      loading.value = false
    }
  }
}

watch(
  () => [props.visible, props.sectionId] as const,
  ([vis]) => {
    if (vis) fetchLifecycle()
  },
  { immediate: true },
)

function handleNodeClick(node: SectionLifecycleNode) {
  emit('navigate', node)
}
</script>
```

**Template:**

```vue
<template>
  <Transition name="slp-panel">
    <div v-if="visible" class="slp-panel">
      <div class="slp-header">
        <span class="slp-title">话题生命周期</span>
        <button type="button" class="slp-close" @click="$emit('close')">&#10005;</button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="slp-body">
        <div v-for="i in 3" :key="i" class="slp-skeleton-node">
          <div class="slp-skeleton-dot" />
          <div class="slp-skeleton-lines">
            <div class="slp-skeleton-line slp-skeleton-short" />
            <div class="slp-skeleton-line slp-skeleton-long" />
          </div>
        </div>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="slp-body">
        <div class="slp-error">加载失败</div>
      </div>

      <!-- Empty (孤立节点) -->
      <div v-else-if="!dagResult || dagResult.nodes.length === 0" class="slp-body">
        <div class="slp-empty">独立话题</div>
      </div>

      <!-- DAG View -->
      <div v-else class="slp-body slp-dag-scroll">
        <svg
          :width="svgWidth"
          :height="svgHeight"
          :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
          class="slp-dag-svg"
        >
          <!-- Edges -->
          <path
            v-for="(edge, i) in dagResult!.edges"
            :key="'e' + i"
            :d="buildScaledPath(edgePathScaled(edge.points))"
            fill="none"
            stroke="rgba(255,255,255,0.15)"
            stroke-width="1.5"
          />

          <!-- Nodes -->
          <g
            v-for="pn in dagResult!.nodes"
            :key="pn.data.id"
            class="slp-dag-node"
            :class="{ 'slp-dag-current': pn.data.id === sectionId, 'slp-dag-ended': pn.data.status === 'ending' }"
            :transform="`translate(${nodePos(pn).x}, ${nodePos(pn).y})`"
            @click="handleNodeClick(pn.data)"
          >
            <!-- Background rect -->
            <rect
              x="0" y="0"
              :width="NODE_W" :height="NODE_H"
              rx="6" ry="6"
              :fill="pn.data.id === sectionId ? 'rgba(255,255,255,0.06)' : 'rgba(255,255,255,0.02)'"
              :stroke="pn.data.status === 'ending' ? 'rgba(156,163,175,0.3)' : 'rgba(255,255,255,0.06)'"
              stroke-dasharray="pn.data.status === 'ending' ? '4,3' : 'none'"
              stroke-width="1"
            />

            <!-- Status dot -->
            <circle
              cx="14" cy="NODE_H / 2"
              r="5"
              :fill="statusColors[pn.data.status] || '#9ca3af'"
            />

            <!-- Text content -->
            <text x="28" y="14" font-size="10" fill="rgba(255,255,255,0.3)">
              {{ formatDate(pn.data.period_date) }}
            </text>
            <text x="28" y="28" font-size="12" font-weight="500" fill="rgba(255,255,255,0.75)">
              {{ truncate(pn.data.cluster_label, 20) }}
            </text>
            <text x="28" y="42" font-size="9" fill="rgba(255,255,255,0.3)">
              {{ pn.data.article_count }} 篇 · {{ pn.data.thread_count }} 条线索
            </text>

            <!-- Status badge -->
            <rect
              :x="NODE_W - 40" y="6"
              width="32" height="16"
              rx="3"
              :fill="statusColors[pn.data.status] ? statusColors[pn.data.status] + '20' : 'rgba(156,163,175,0.2)'"
            />
            <text
              :x="NODE_W - 24" y="17"
              font-size="8"
              font-weight="500"
              :fill="statusColors[pn.data.status] || '#9ca3af'"
              text-anchor="middle"
            >
              {{ statusLabel[pn.data.status] || pn.data.status }}
            </text>
          </g>
        </svg>
      </div>
    </div>
  </Transition>
</template>
```

注意 template 中需要辅助函数：

```typescript
function truncate(str: string, maxLen: number): string {
  return str.length > maxLen ? str.slice(0, maxLen) + '…' : str
}

function buildScaledPath(points: Array<{ x: number; y: number }>): string {
  if (points.length < 2) return ''
  if (points.length === 2) {
    return `M${points[0].x},${points[0].y} L${points[1].x},${points[1].y}`
  }
  let d = `M${points[0].x},${points[0].y}`
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]
    const curr = points[i]
    const cpx1 = prev.x + (curr.x - prev.x) * 0.4
    const cpx2 = prev.x + (curr.x - prev.x) * 0.6
    d += ` C${cpx1},${prev.y} ${cpx2},${curr.y} ${curr.x},${curr.y}`
  }
  return d
}
```

**Style（保留面板样式，新增 DAG 相关）:**

保留 `.slp-panel`, `.slp-header`, `.slp-close`, `.slp-body`, loading/error/empty/skeleton 样式不变。

新增/修改：

```css
/* DAG scroll container */
.slp-dag-scroll {
  overflow-y: auto;
  overflow-x: hidden;
}

.slp-dag-svg {
  display: block;
  width: 100%;
}

/* DAG node */
.slp-dag-node {
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.slp-dag-node:hover rect:first-child {
  fill: rgba(255, 255, 255, 0.06);
}

.slp-dag-current rect:first-child {
  fill: rgba(255, 255, 255, 0.08);
}

.slp-dag-current circle {
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.3));
}

/* Ended nodes: reduced opacity + gray border (already in rect attrs) */
.slp-dag-ended {
  opacity: 0.55;
}
```

**Step 2: 手动验证**

启动 `pnpm dev`，打开 BoardDailyReportTimeline 页面，点击某个 section 打开 SectionLifecyclePanel：
- 验证面板正常打开
- 验证 SVG DAG 正确渲染
- 验证当前节点高亮
- 验证点击节点触发 navigate

**Step 3: Commit**

```bash
git add front/app/features/tags/components/SectionLifecyclePanel.vue
git commit -m "feat: rewrite SectionLifecyclePanel as vertical DAG with d3-dag"
```

---

## Task 3: 重写 BoardThreadBrowser.vue（水平 DAG 时间线）

**Files:**
- Rewrite: `front/app/features/tags/components/BoardThreadBrowser.vue`

**设计要点：**
- 使用 `useDagLayout` 做 LR（从左到右）布局
- SVG 渲染水平 DAG：横轴=日期（由 d3-dag 层级决定），纵轴=话题 lane
- 保留：days 选择器 (7/14/30/60)、节点点击详情弹窗、loading/empty 状态
- 新增：split/merge 的分支/合并贝塞尔曲线、lane 自动分配
- 新增：ended 节点透明度降低 + 灰色边框
- 去掉：HTML table 布局、线性 chain 构建、直线 connector

**关键差异 vs SectionLifecyclePanel：**
1. LR 方向：x 轴代表时间流向（从左到右）
2. 节点更紧凑：圆形节点而非矩形卡片
3. 保留日期列标题行
4. 需要将 d3-dag 的层级（y in TB → x in LR）映射到日期列

**LR 布局的坐标映射：**
- `useDagLayout` 的 LR 模式会交换 x/y，所以布局输出的 x 代表时间轴，y 代表 lane
- 节点尺寸：小圆点 + tooltip，nodeSize = [1, 1]
- 缩放：x * colWidth, y * rowHeight

**Step 1: 重写组件**

```vue
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation } from '~/api/dailyReports'
import { useDagLayout } from '~/composables/useDagLayout'

const props = defineProps<{ boardId: number }>()

const { getBoardSectionTimeline } = useDailyReportsApi()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)

// --- Status styling ---

const statusColors: Record<string, string> = {
  emerging: '#34d399',
  continuing: '#60a5fa',
  split: '#fb923c',
  merge: '#c084fc',
  ending: '#9ca3af',
}

const statusLabels: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

// --- Date columns for header ---

const dateColumns = computed<string[]>(() => {
  const cols: string[] = []
  const now = new Date()
  for (let i = days.value - 1; i >= 0; i--) {
    const d = new Date(now)
    d.setDate(d.getDate() - i)
    cols.push(formatDateISO(d))
  }
  return cols
})

function formatDateISO(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function formatDateShort(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

// --- DAG Layout ---

const COL_W = 40   // 日期列宽
const ROW_H = 40   // lane 高度
const PAD = 20     // padding
const NODE_R = 6   // 节点半径

const dagResult = computed(() => {
  if (sections.value.length === 0) return null

  const nodes = sections.value.map(s => ({ id: s.id, ...s }))
  const edges = relations.value.map(r => ({ from: r.from_id, to: r.to_id, ...r }))

  return useDagLayout(nodes, edges, 'LR', [1, 1], [1, 1])
})

// SVG 尺寸
const svgWidth = computed(() => {
  if (!dagResult.value) return 400
  return (dagResult.value.width + 1) * COL_W + PAD * 2
})

const svgHeight = computed(() => {
  if (!dagResult.value) return 200
  return (dagResult.value.height + 1) * ROW_H + PAD * 2
})

function nodeScreenPos(node: { x: number; y: number }) {
  return {
    cx: node.x * COL_W + PAD + COL_W / 2,
    cy: node.y * ROW_H + PAD + ROW_H / 2,
  }
}

function edgePointsScaled(points: Array<{ x: number; y: number }>) {
  return points.map(p => ({
    x: p.x * COL_W + PAD + COL_W / 2,
    y: p.y * ROW_H + PAD + ROW_H / 2,
  }))
}

function buildEdgePath(points: Array<{ x: number; y: number }>): string {
  if (points.length < 2) return ''
  if (points.length === 2) {
    return `M${points[0].x},${points[0].y} C${(points[0].x + points[1].x) / 2},${points[0].y} ${(points[0].x + points[1].x) / 2},${points[1].y} ${points[1].x},${points[1].y}`
  }
  let d = `M${points[0].x},${points[0].y}`
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]
    const curr = points[i]
    d += ` C${(prev.x + curr.x) / 2},${prev.y} ${(prev.x + curr.x) / 2},${curr.y} ${curr.x},${curr.y}`
  }
  return d
}

function selectNode(node: SectionTimelineNode) {
  selectedNode.value = selectedNode.value?.id === node.id ? null : node
}

// --- Data loading ---

async function loadData() {
  loading.value = true
  selectedNode.value = null
  try {
    const res = await getBoardSectionTimeline(props.boardId, days.value)
    if (res.success && res.data) {
      sections.value = res.data.sections || []
      relations.value = res.data.relations || []
    } else {
      sections.value = []
      relations.value = []
    }
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.boardId, days],
  () => { loadData() },
  { immediate: true },
)
</script>
```

**Template:**

```vue
<template>
  <div class="btb-container">
    <!-- Controls -->
    <div class="btb-controls">
      <div class="btb-controls-left">
        <Icon icon="mdi:source-branch" width="15" class="text-white/50" />
        <span class="btb-controls-title">话题总览</span>
      </div>
      <div class="btb-days-toggle">
        <button
          v-for="d in [7, 14, 30, 60]"
          :key="d"
          class="btb-days-btn"
          :class="{ active: days === d }"
          @click="days = d"
        >
          {{ d }}天
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="btb-loading">
      <div v-for="i in 3" :key="i" class="btb-skeleton" />
    </div>

    <!-- Empty -->
    <div v-else-if="!dagResult || dagResult.nodes.length === 0" class="btb-empty">
      <Icon icon="mdi:source-branch" width="28" class="text-white/15" />
      <p>暂无话题数据</p>
    </div>

    <!-- DAG Chart -->
    <div v-else class="btb-chart">
      <!-- Date header -->
      <div class="btb-date-headers" :style="{ paddingLeft: `${PAD}px`, width: `${svgWidth}px` }">
        <div
          v-for="date in dateColumns"
          :key="date"
          class="btb-date-cell"
          :style="{ width: `${COL_W}px` }"
        >
          {{ formatDateShort(date) }}
        </div>
      </div>

      <!-- SVG DAG -->
      <div class="btb-svg-wrap">
        <svg
          :width="svgWidth"
          :height="svgHeight"
          :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
          class="btb-dag-svg"
        >
          <!-- Date grid lines (subtle) -->
          <line
            v-for="(date, i) in dateColumns"
            :key="'grid' + i"
            :x1="i * COL_W + PAD + COL_W / 2"
            :y1="0"
            :x2="i * COL_W + PAD + COL_W / 2"
            :y2="svgHeight"
            stroke="rgba(255,255,255,0.03)"
            stroke-width="1"
          />

          <!-- Edges -->
          <path
            v-for="(edge, i) in dagResult!.edges"
            :key="'e' + i"
            :d="buildEdgePath(edgePointsScaled(edge.points))"
            fill="none"
            stroke="rgba(255,255,255,0.12)"
            stroke-width="1.5"
          />

          <!-- Nodes -->
          <g
            v-for="pn in dagResult!.nodes"
            :key="pn.data.id"
            class="btb-dag-node"
            :class="{ 'btb-dag-selected': selectedNode?.id === pn.data.id, 'btb-dag-ended': pn.data.status === 'ending' }"
            @click="selectNode(pn.data)"
          >
            <circle
              :cx="nodeScreenPos(pn).cx"
              :cy="nodeScreenPos(pn).cy"
              :r="NODE_R"
              :fill="statusColors[pn.data.status] || '#9ca3af'"
              stroke="rgba(255,255,255,0.15)"
              stroke-width="1"
            />
            <title>{{ pn.data.cluster_label }} ({{ statusLabels[pn.data.status] }})</title>
          </g>
        </svg>
      </div>
    </div>

    <!-- Node detail popup -->
    <Teleport to="body">
      <Transition name="btb-popup">
        <div v-if="selectedNode" class="btb-popup-overlay" @click.self="selectedNode = null">
          <div class="btb-popup">
            <div class="btb-popup-header">
              <span
                class="btb-popup-status"
                :style="{ background: statusColors[selectedNode.status] || '#9ca3af' }"
              >
                {{ statusLabels[selectedNode.status] || selectedNode.status }}
              </span>
              <button class="btb-popup-close" @click="selectedNode = null">
                <Icon icon="mdi:close" width="14" />
              </button>
            </div>
            <div class="btb-popup-title">{{ selectedNode.cluster_label }}</div>
            <div class="btb-popup-meta">
              <span>{{ formatDateShort(selectedNode.period_date) }}</span>
              <span>{{ selectedNode.article_count }} 篇</span>
              <span>{{ selectedNode.thread_count }} 条线索</span>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
```

**Style:**

```css
.btb-container {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.025);
}

.btb-controls { /* same as current */ }
.btb-controls-left { /* same */ }
.btb-controls-title { /* same */ }
.btb-days-toggle { /* same */ }
.btb-days-btn { /* same */ }

/* Loading/empty */
.btb-loading { /* same skeleton */ }
.btb-empty { /* same */ }

/* Chart */
.btb-chart {
  display: flex;
  flex-direction: column;
  overflow-x: auto;
}

.btb-date-headers {
  display: flex;
  flex-shrink: 0;
  padding-bottom: 0.3rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  margin-bottom: 0.25rem;
}

.btb-date-cell {
  flex-shrink: 0;
  text-align: center;
  font-size: 0.55rem;
  color: rgba(255, 255, 255, 0.25);
  transform: rotate(-35deg);
  transform-origin: center bottom;
  white-space: nowrap;
}

.btb-svg-wrap {
  overflow-x: auto;
  overflow-y: hidden;
}

.btb-dag-svg {
  display: block;
}

/* DAG nodes */
.btb-dag-node {
  cursor: pointer;
}

.btb-dag-node:hover circle {
  r: 8;
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.3));
}

.btb-dag-selected circle {
  r: 9;
  stroke-width: 2;
  stroke: rgba(255, 255, 255, 0.5);
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.4));
}

/* Ended nodes */
.btb-dag-ended {
  opacity: 0.45;
}

.btb-dag-ended circle {
  stroke-dasharray: 3,2;
  stroke: rgba(156, 163, 175, 0.5);
}

/* Popup styles - keep existing */
.btb-popup-overlay { /* same */ }
.btb-popup { /* same */ }
/* ... keep all popup styles from current component */
```

**Step 2: 手动验证**

启动 dev server，打开 BoardDailyReportTimeline，切换到话题总览 tab：
- 验证 DAG 正确渲染
- 验证分支/合并的贝塞尔曲线
- 验证日期列标题对齐
- 验证节点点击弹出详情
- 验证 ended 节点视觉处理

**Step 3: Commit**

```bash
git add front/app/features/tags/components/BoardThreadBrowser.vue
git commit -m "feat: rewrite BoardThreadBrowser as horizontal DAG with d3-dag"
```

---

## Task 4: 整体验证与 lint

**Step 1: 前端 lint**

```bash
cd front && pnpm lint
```

Expected: PASS（无新增 error）

**Step 2: 前端 typecheck**

```bash
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
```

Expected: PASS（可能有 TagsPage.vue 既有错误，非本次变更）

**Step 3: 前端 build**

```bash
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

Expected: PASS

**Step 4: Commit（如有 lint 修复）**

```bash
git add -A && git commit -m "fix: lint fixes for DAG visualization components"
```

---

## 验收标准

1. `useDagLayout` composable 正确封装 d3-dag Sugiyama 布局
2. SectionLifecyclePanel 以垂直 DAG 展示话题生命周期，支持 split/merge 可视化
3. BoardThreadBrowser 以水平 DAG 展示话题时间线，贝塞尔曲线连接分支/合并
4. `ending` 状态节点视觉降低透明度 + 灰色虚线边框
5. 保留所有现有交互：面板开合、节点点击导航、loading/error/empty 状态
6. 前端 lint/typecheck/build 通过
