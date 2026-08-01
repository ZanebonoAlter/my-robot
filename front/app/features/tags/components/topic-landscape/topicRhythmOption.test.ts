/**
 * buildRhythmOption 单测（task 2.1）。
 *
 * happy-dom 无 canvas，只测纯函数（design D2）。断言：
 *  - y 轴话题排序（stance 分组序 active→stalled→emerging→pending→archived、组内 hit_count DESC）
 *  - 按 stance 分 5 个 series
 *  - archived series 默认 legend unselected
 *  - section_count=0 不产数据点
 *  - 气泡尺寸 sqrt 缩放且 clamp [6,26]
 *  - y 轴 dataZoom（inside + slider）默认窗口
 */
import { describe, it, expect } from 'vitest'
import {
  buildRhythmOption,
  bubbleSize,
  defaultZoomEnd,
  sortTopicsForAxis,
  STANCE_ORDER,
  STANCE_LABEL,
  type ChartPalette,
} from './chart-options'
import type { TopicLandscapeTopic, TopicStance, LifelinePoint } from '~/api/semanticBoards'

// ── fixture ──────────────────────────────────────────────────────────────────
const PALETTE: ChartPalette = {
  accent: '#d94a4a',
  textPrimary: '#000',
  textSecondary: '#555',
  textMuted: '#888',
  borderSubtle: '#eee',
  bgElevated: '#fff',
  stance: {
    active: '#3d8a4a',
    stalled: '#3d7a8a',
    emerging: '#c4883c',
    pending: '#c42f3c',
    archived: '#888',
  },
}

function lp(date: string, section_count: number): LifelinePoint {
  return { date, section_count }
}

function mkTopic(
  id: number,
  label: string,
  stance: TopicStance,
  hit_count: number,
  lifeline: LifelinePoint[],
): TopicLandscapeTopic {
  return {
    id,
    label,
    status: 'active',
    source: 'identity',
    stance,
    is_vacuum: false,
    vacuum_strong: 0,
    hit_count,
    consecutive_hits: 0,
    first_seen_date: '2026-01-01',
    last_seen_date: '2026-01-03',
    days_since_last: 0,
    can_activate: false,
    lifeline,
  }
}

// 输入故意乱序，验证排序
const DATES = ['2026-01-01', '2026-01-02', '2026-01-03']
const TOPICS: TopicLandscapeTopic[] = [
  mkTopic(1, 'active-A', 'active', 10, [lp('2026-01-01', 2), lp('2026-01-02', 5), lp('2026-01-03', 0)]),
  mkTopic(2, 'active-B', 'active', 20, [lp('2026-01-01', 0), lp('2026-01-02', 1), lp('2026-01-03', 9)]),
  mkTopic(3, 'stalled-A', 'stalled', 5, [lp('2026-01-01', 3), lp('2026-01-02', 0), lp('2026-01-03', 0)]),
  mkTopic(4, 'emerging-A', 'emerging', 1, [lp('2026-01-02', 1)]),
  mkTopic(5, 'pending-A', 'pending', 3, [lp('2026-01-03', 2)]),
  mkTopic(6, 'archived-A', 'archived', 8, [lp('2026-01-01', 4)]),
]

// ── 排序 ─────────────────────────────────────────────────────────────────────
describe('sortTopicsForAxis', () => {
  it('按 stance 分组序 + 组内 hit_count DESC', () => {
    const sorted = sortTopicsForAxis(TOPICS)
    const labels = sorted.map((t) => t.label)
    expect(labels).toEqual([
      'active-B', // active, hit 20
      'active-A', // active, hit 10
      'stalled-A', // stalled
      'emerging-A', // emerging
      'pending-A', // pending
      'archived-A', // archived
    ])
  })

  it('STANCE_ORDER 顺序与卡片墙一致', () => {
    expect(STANCE_ORDER).toEqual([
      'active',
      'stalled',
      'emerging',
      'pending',
      'archived',
    ])
  })
})

// ── buildRhythmOption ────────────────────────────────────────────────────────
describe('buildRhythmOption', () => {
  const option = buildRhythmOption(TOPICS, DATES, PALETTE)

  it('y 轴话题排序 = stance 分组序 + hit_count DESC', () => {
    const yAxis = option.yAxis as { data: string[] }
    expect(yAxis.data).toEqual([
      'active-B',
      'active-A',
      'stalled-A',
      'emerging-A',
      'pending-A',
      'archived-A',
    ])
  })

  it('y 轴 inverse=true', () => {
    expect((option.yAxis as { inverse: boolean }).inverse).toBe(true)
  })

  it('x 轴 = 传入日期 category', () => {
    const xAxis = option.xAxis as { data: string[]; type: string }
    expect(xAxis.type).toBe('category')
    expect(xAxis.data).toEqual(DATES)
  })

  it('按 stance 分 5 个 series，名称对应 STANCE_LABEL', () => {
    const series = option.series as Array<{ name: string; type: string }>
    expect(series).toHaveLength(5)
    expect(series.map((s) => s.name)).toEqual(STANCE_ORDER.map((s) => STANCE_LABEL[s]))
    expect(series.every((s) => s.type === 'scatter')).toBe(true)
  })

  it('archived series 默认 legend unselected', () => {
    const legend = option.legend as { selected: Record<string, boolean> }
    expect(legend.selected[STANCE_LABEL.archived]).toBe(false)
  })

  it('section_count=0 不产数据点（active-A 第 3 日 0、active-B 第 1 日 0 不应出现）', () => {
    const series = option.series as Array<{ name: string; data: Array<[number, number, number, number]> }>
    const activeSeries = series.find((s) => s.name === STANCE_LABEL.active)!
    // active-A: 01=2, 02=5, 03=0 → 2 点；active-B: 01=0, 02=1, 03=9 → 2 点；合计 4 点
    expect(activeSeries.data).toHaveLength(4)
    // 不存在 count=0 的点
    expect(activeSeries.data.every((d) => d[2] > 0)).toBe(true)
  })

  it('数据点结构 = [xIndex, yIndex, section_count, topicId]', () => {
    const series = option.series as Array<{ name: string; data: Array<[number, number, number, number]> }>
    const archivedSeries = series.find((s) => s.name === STANCE_LABEL.archived)!
    // archived-A：唯一一条 [xi=0(01-01), yi=5, count=4, topicId=6]
    expect(archivedSeries.data).toEqual([[0, 5, 4, 6]])
  })

  it('气泡尺寸 = bubbleSize（sqrt 缩放，clamp [6,26]）', () => {
    const series = option.series as Array<{ symbolSize: (v: number[]) => number }>
    const fn = series[0]!.symbolSize
    expect(fn([0, 0, 1])).toBe(bubbleSize(1))
    expect(fn([0, 0, 9])).toBe(bubbleSize(9))
  })

  it('y 轴 dataZoom 含 inside + slider，默认窗口覆盖全部（话题数 ≤ 25）', () => {
    const dz = option.dataZoom as Array<{ type: string; yAxisIndex: number; start: number; end: number }>
    expect(dz).toHaveLength(2)
    expect(dz.map((d) => d.type).sort()).toEqual(['inside', 'slider'])
    expect(dz.every((d) => d.yAxisIndex === 0)).toBe(true)
    expect(dz[0]!.start).toBe(0)
    expect(dz[0]!.end).toBe(100)
  })

  it('lifeline 全 0 的话题不进入 y 轴（无空泳道行）、不产数据点', () => {
    const noHit = mkTopic(99, 'empty-no-hit', 'stalled', 3, [
      lp('2026-01-01', 0),
      lp('2026-01-02', 0),
      lp('2026-01-03', 0),
    ])
    const topics = [...TOPICS, noHit]
    const opt = buildRhythmOption(topics, DATES, PALETTE)
    const yAxis = opt.yAxis as { data: string[] }
    expect(yAxis.data).toEqual([
      'active-B',
      'active-A',
      'stalled-A',
      'emerging-A',
      'pending-A',
      'archived-A',
    ])
    const series = opt.series as Array<{ data: Array<[number, number, number, number]> }>
    const allPoints = series.flatMap((s) => s.data)
    expect(allPoints.some((p) => p[3] === 99)).toBe(false)
  })
})

// ── 气泡尺寸 / 缩放窗口 ──────────────────────────────────────────────────────
describe('bubbleSize', () => {
  it('sqrt(count)*5 并 clamp 到 [6,26]', () => {
    expect(bubbleSize(0)).toBe(6) // sqrt(0)*5=0 → clamp 6
    expect(bubbleSize(1)).toBe(6) // sqrt(1)*5=5 → clamp 6
    expect(bubbleSize(4)).toBe(10) // sqrt(4)*5=10
    expect(bubbleSize(9)).toBe(15) // sqrt(9)*5=15
    expect(bubbleSize(16)).toBe(20) // sqrt(16)*5=20
    expect(bubbleSize(30)).toBeLessThanOrEqual(26)
    expect(bubbleSize(36)).toBe(26) // sqrt(36)*5=30 → clamp 上界
  })
})

describe('defaultZoomEnd', () => {
  it('话题数 ≤ 25 → 100', () => {
    expect(defaultZoomEnd(6)).toBe(100)
    expect(defaultZoomEnd(25)).toBe(100)
  })
  it('话题数 > 25 → 25/n*100', () => {
    expect(defaultZoomEnd(50)).toBe(50)
    expect(defaultZoomEnd(100)).toBe(25)
  })
})
