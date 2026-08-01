/**
 * 话题态势版图 ECharts option 纯函数模块（design D2 + D5）。
 *
 * 三个 builder 全部纯函数化（输入 topics/dates/trend + palette → EChartsCoreOption），
 * 便于 Vitest 直接断言 series/axis/映射正确性（happy-dom 无 canvas，不渲染 echarts）。
 * 组件只负责挂载 / 传参 / 事件桥接（见 useEcharts + *.vue）。
 *
 * palette 由 readPalette() 从 CSS 变量读取（design D5：单一事实源在 main.css，
 * 避免硬编码色板随主题迭代漂移）。stance 语义色用 success/warning/error/info
 * （main.css 中标注「语义色，不随主题变」），archived 用 text-muted（主题相关）。
 */
import type { EChartsCoreOption } from 'echarts/core'
import type {
  TopicLandscapeTopic,
  TopicStance,
  LifelinePoint,
} from '~/api/semanticBoards'

/** stance 渲染顺序（与 StanceCardWall 一致：active→stalled→emerging→pending→archived）。 */
export const STANCE_ORDER: TopicStance[] = [
  'active',
  'stalled',
  'emerging',
  'pending',
  'archived',
]

/** stance 中文标签（用于 legend / tooltip）。 */
export const STANCE_LABEL: Record<TopicStance, string> = {
  active: '活跃',
  stalled: '停滞',
  emerging: '新冒头',
  pending: '待激活',
  archived: '已归档',
}

/** 图表调色板（readPalette 运行期从 CSS 变量读取，测试用固定 fixture）。 */
export interface ChartPalette {
  accent: string
  textPrimary: string
  textSecondary: string
  textMuted: string
  borderSubtle: string
  bgElevated: string
  stance: Record<TopicStance, string>
}

// ── 气泡尺寸（sqrt 缩放 + clamp，design D3） ────────────────────────────────
export const BUBBLE_MIN = 6
export const BUBBLE_MAX = 26

/** sqrt(section_count) * 5，clamp 到 [BUBBLE_MIN, BUBBLE_MAX]。 */
export function bubbleSize(count: number): number {
  const raw = Math.sqrt(Math.max(0, count)) * 5
  return Math.min(BUBBLE_MAX, Math.max(BUBBLE_MIN, raw))
}

// ── y 轴默认 dataZoom 窗口（design D3：默认显示排序前 ~25 个） ────────────────
const DEFAULT_Y_WINDOW = 25

/** 话题总数 > 窗口时返回 25/n*100 的 end 百分比，否则 100。 */
export function defaultZoomEnd(total: number): number {
  if (total <= DEFAULT_Y_WINDOW) return 100
  return Math.round((DEFAULT_Y_WINDOW / total) * 100)
}

/**
 * 话题排序：stance 分组序（STANCE_ORDER）+ 组内 hit_count DESC。
 * y 轴顺序与 StanceCardWall 分组顺序保持一致（design D3）。
 */
export function sortTopicsForAxis(
  topics: TopicLandscapeTopic[],
): TopicLandscapeTopic[] {
  const rank = new Map(STANCE_ORDER.map((s, i) => [s, i]))
  return [...topics].sort((a, b) => {
    const r = (rank.get(a.stance) ?? 99) - (rank.get(b.stance) ?? 99)
    if (r !== 0) return r
    return b.hit_count - a.hit_count
  })
}

/**
 * 从 topics 的 lifeline 提取连续日期轴（后端 generate_series 保证同一板块所有
 * 话题共享同一连续 N 日范围）。取最长 lifeline 的日期序列。
 */
export function getRhythmDates(topics: TopicLandscapeTopic[]): string[] {
  let longest: string[] = []
  for (const t of topics) {
    if (t.lifeline.length > longest.length) {
      longest = t.lifeline.map((p) => p.date)
    }
  }
  return longest
}

/** ISO 日期 "2026-01-05" → "1/5"。 */
function shortDate(iso: string): string {
  const m = Number(iso.slice(5, 7))
  const d = Number(iso.slice(8, 10))
  return `${m}/${d}`
}

/** 迷你柱图 tooltip 文案："M/D：N 节"。 */
export function formatMiniTooltip(date: string, count: number): string {
  return `${shortDate(date)}：${count} 节`
}

// ── readPalette（运行期读 CSS 变量，SSR 后 onMounted 调用） ──────────────────
function readVar(name: string, fallback: string): string {
  const val = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return val || fallback
}

/** 读取主题调色板（亮/暗主题均通过 CSS 变量反映）。 */
export function readPalette(): ChartPalette {
  return {
    accent: readVar('--color-accent', '#d94a4a'),
    textPrimary: readVar('--color-text-primary', '#1a1a1a'),
    textSecondary: readVar('--color-text-secondary', '#5a5a5a'),
    textMuted: readVar('--color-text-muted', '#8a8a8a'),
    borderSubtle: readVar('--color-border-subtle', 'rgba(26,26,26,0.08)'),
    bgElevated: readVar('--color-bg-elevated', '#f5f5f4'),
    stance: {
      active: readVar('--color-success', '#3d8a4a'),
      stalled: readVar('--color-info', '#3d7a8a'),
      emerging: readVar('--color-warning', '#c4883c'),
      pending: readVar('--color-error', '#c42f3c'),
      archived: readVar('--color-text-muted', '#8a8a8a'),
    },
  }
}

// ── 本地窄类型（tooltip formatter 参数，echarts 内部深类型不入 context） ──────
interface ScatterTipValue {
  value: number[]
}
interface AxisTipParam {
  value?: number
  axisValue?: string
}

// ════════════════════════════════════════════════════════════════════════════
// buildRhythmOption：话题节奏总览气泡图（design D3 / spec「话题节奏总览气泡图」）
// ════════════════════════════════════════════════════════════════════════════
/**
 * 数据点结构：[xIndex, yIndex, section_count, topicId]。
 * - x/y 用数值索引（category 轴接受数值作为索引，避免重复 label 串匹配塌缩）。
 * - topicId 附在末尾，供组件点击回调 emit selectTopic。
 */
export type RhythmPoint = [number, number, number, number]

export function buildRhythmOption(
  topics: TopicLandscapeTopic[],
  dates: string[],
  palette: ChartPalette,
): EChartsCoreOption {
  // 只保留所选时间范围内至少有一个命中（section_count>0）的话题：lifeline 全 0 的
  // 话题在 y 轴上是无气泡的空泳道行，易被误读为「有节奏但没画出来」，故不渲染。
  const withRhythm = topics.filter((t) => t.lifeline.some((p) => p.section_count > 0))
  const sorted = sortTopicsForAxis(withRhythm)
  const labels = sorted.map((t) => t.label)
  const yIndex = new Map(sorted.map((t, i) => [t.id, i]))
  const xIndex = new Map(dates.map((d, i) => [d, i]))

  // 按 stance 分 5 个 series（免费获得 legend 可点选过滤）
  const seriesData: Record<TopicStance, RhythmPoint[]> = {
    active: [],
    stalled: [],
    emerging: [],
    pending: [],
    archived: [],
  }
  for (const t of sorted) {
    const yi = yIndex.get(t.id)
    if (yi === undefined) continue
    for (const p of t.lifeline) {
      if (p.section_count <= 0) continue // section_count=0 不画点
      const xi = xIndex.get(p.date)
      if (xi === undefined) continue
      seriesData[t.stance].push([xi, yi, p.section_count, t.id])
    }
  }

  const series = STANCE_ORDER.map((stance) => ({
    name: STANCE_LABEL[stance],
    type: 'scatter' as const,
    data: seriesData[stance],
    itemStyle: { color: palette.stance[stance] },
    symbolSize: (value: number[]) => bubbleSize(value[2] ?? 0),
    emphasis: { focus: 'series' as const },
  }))

  const zoomEnd = defaultZoomEnd(sorted.length)

  return {
    legend: {
      data: STANCE_ORDER.map((s) => STANCE_LABEL[s]),
      selected: { [STANCE_LABEL.archived]: false },
      top: 0,
      textStyle: { color: palette.textSecondary, fontSize: 12 },
      itemWidth: 14,
      itemHeight: 14,
    },
    grid: { left: 6, right: 26, top: 36, bottom: 24, containLabel: true },
    tooltip: {
      trigger: 'item',
      formatter: (p: unknown) => {
        const { value } = p as ScatterTipValue
        if (!value) return ''
        const xi = value[0]
        const yi = value[1]
        const count = value[2]
        if (xi === undefined || yi === undefined) return ''
        const date = dates[xi]
        const topic = labels[yi]
        if (date === undefined || topic === undefined || count === undefined) return ''
        return `<b>${topic}</b><br/>${shortDate(date)}：${count} 节`
      },
    },
    xAxis: {
      type: 'category',
      data: dates,
      boundaryGap: false,
      axisLine: { lineStyle: { color: palette.borderSubtle } },
      axisTick: { show: false },
      axisLabel: {
        color: palette.textMuted,
        fontSize: 11,
        formatter: (val: string) => shortDate(val),
        interval: Math.ceil(dates.length / 8) - 1,
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'category',
      data: labels,
      inverse: true,
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: { color: palette.textSecondary, fontSize: 12 },
      splitLine: { show: false },
    },
    dataZoom: [
      { type: 'inside', yAxisIndex: 0, start: 0, end: zoomEnd },
      {
        type: 'slider',
        yAxisIndex: 0,
        start: 0,
        end: zoomEnd,
        width: 14,
        right: 2,
        filterMode: 'filter',
      },
    ],
    series,
  }
}

// ════════════════════════════════════════════════════════════════════════════
// buildMiniBarOption：卡片迷你柱图（design D4 / spec「话题卡片 mini-lifeline」）
// ════════════════════════════════════════════════════════════════════════════
export function buildMiniBarOption(
  lifeline: LifelinePoint[],
  palette: ChartPalette,
): EChartsCoreOption {
  const dates = lifeline.map((p) => p.date)
  const counts = lifeline.map((p) => p.section_count) // 空日 0 值占位（category 轴仍保留该列）

  return {
    grid: { left: 0, right: 0, top: 2, bottom: 0, containLabel: false },
    xAxis: {
      type: 'category',
      show: false,
      data: dates,
    },
    yAxis: {
      type: 'value',
      show: false,
      min: 0,
    },
    tooltip: {
      trigger: 'axis',
      formatter: (p: unknown) => {
        const param = (Array.isArray(p) ? p[0] : p) as AxisTipParam | undefined
        if (!param) return ''
        const date = param.axisValue ?? ''
        const count = typeof param.value === 'number' ? param.value : 0
        return formatMiniTooltip(date, count)
      },
    },
    series: [
      {
        type: 'bar',
        data: counts,
        itemStyle: { color: palette.accent },
        barCategoryGap: '20%',
      },
    ],
  }
}

// ════════════════════════════════════════════════════════════════════════════
// buildVitalityOption：活力顶栏面积图（design / spec「活力顶栏」）
// ════════════════════════════════════════════════════════════════════════════
/**
 * 注：Vitality.trend 仅含每日 section 计数（number[]），API 不带日期数组
 * （本 change 明确不改后端/数据模型）。日期标签由调用方（VitalityBar.vue）
 * 从今天往前推生成（MM-DD）后传入，builder 内不调 Date.now，保持纯函数确定性。
 */
export function buildVitalityOption(
  trend: number[],
  dates: string[],
  palette: ChartPalette,
): EChartsCoreOption {
  return {
    grid: { left: 0, right: 0, top: 3, bottom: 3 },
    xAxis: {
      type: 'category',
      data: dates,
      boundaryGap: false,
      axisLine: { lineStyle: { color: palette.borderSubtle } },
      axisTick: { show: false },
      // 轻量坐标轴：迷你图只显示首尾两个日期标签，避免刻度挤满
      axisLabel: {
        color: palette.textMuted,
        fontSize: 9,
        interval: (index: number) => index === 0 || index === dates.length - 1,
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      show: false,
      scale: true,
    },
    tooltip: {
      trigger: 'axis',
      formatter: (p: unknown) => {
        const param = (Array.isArray(p) ? p[0] : p) as AxisTipParam | undefined
        if (!param) return ''
        const count = typeof param.value === 'number' ? param.value : 0
        return `${count} 节`
      },
    },
    series: [
      {
        type: 'line',
        data: trend,
        smooth: true,
        showSymbol: false,
        lineStyle: { color: palette.accent, width: 1.5 },
        areaStyle: { color: palette.accent, opacity: 0.18 },
      },
    ],
  }
}
