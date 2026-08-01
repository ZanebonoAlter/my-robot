/**
 * buildMiniBarOption 单测（task 3.1）。
 *
 * 断言：
 *  - x 轴取 lifeline 全部日期（含空日），日期轴连续
 *  - 柱高 = section_count（空日 0 值占位，不跳过列）
 *  - tooltip formatter 输出 "M/D：N 节"
 */
import { describe, it, expect } from 'vitest'
import { buildMiniBarOption, formatMiniTooltip, type ChartPalette } from './chart-options'
import type { LifelinePoint } from '~/api/semanticBoards'

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

const LIFELINE: LifelinePoint[] = [
  { date: '2026-01-01', section_count: 3 },
  { date: '2026-01-02', section_count: 0 }, // 空日：0 值占位
  { date: '2026-01-03', section_count: 7 },
  { date: '2026-01-05', section_count: 0 }, // 空日：0 值占位
  { date: '2026-01-06', section_count: 1 },
]

describe('buildMiniBarOption', () => {
  const option = buildMiniBarOption(LIFELINE, PALETTE)

  it('x 轴取 lifeline 全部日期（连续，含空日）', () => {
    const xAxis = option.xAxis as { data: string[]; type: string }
    expect(xAxis.type).toBe('category')
    expect(xAxis.data).toEqual(LIFELINE.map((p) => p.date))
  })

  it('柱高 = section_count，空日为 0（不跳过列）', () => {
    const series = option.series as Array<{ type: string; data: number[] }>
    expect(series).toHaveLength(1)
    expect(series[0]!.type).toBe('bar')
    expect(series[0]!.data).toEqual([3, 0, 7, 0, 1])
  })

  it('tooltip 为 axis 触发', () => {
    const tooltip = option.tooltip as { trigger: string }
    expect(tooltip.trigger).toBe('axis')
  })

  it('tooltip formatter 输出 "M/D：N 节"', () => {
    const tooltip = option.tooltip as { formatter: (p: unknown) => string }
    const out = tooltip.formatter([{ value: 7, axisValue: '2026-01-03' }])
    expect(out).toBe('1/3：7 节')
  })

  it('tooltip formatter 对空数组入参安全（返回空串）', () => {
    const tooltip = option.tooltip as { formatter: (p: unknown) => string }
    expect(tooltip.formatter([])).toBe('')
  })
})

describe('formatMiniTooltip', () => {
  it('"2026-01-05",3 → "1/5：3 节"', () => {
    expect(formatMiniTooltip('2026-01-05', 3)).toBe('1/5：3 节')
  })
  it('"2026-12-25",0 → "12/25：0 节"（空日也能展示）', () => {
    expect(formatMiniTooltip('2026-12-25', 0)).toBe('12/25：0 节')
  })
})
