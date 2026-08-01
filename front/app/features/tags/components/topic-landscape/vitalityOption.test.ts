/**
 * buildVitalityOption 单测（task 4.1）。
 *
 * 断言：
 *  - x 轴日期连续（data = 调用方传入的 MM-DD 日期标签，长度 = trend.length）
 *  - series 为 line + areaStyle
 *  - 含轻量坐标轴（x 轴首尾标签可见）+ axis tooltip
 *
 * 注：Vitality.trend 仅 number[]（无日期数组，本 change 不改后端），日期标签由
 * 调用方生成后传入，builder 内不调 Date.now，纯函数确定性。
 */
import { describe, it, expect } from 'vitest'
import { buildVitalityOption, type ChartPalette } from './chart-options'

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

const TREND = [4, 7, 0, 12, 5, 9, 3]
/** 调用方（VitalityBar.vue）从今天往前推生成的 MM-DD 日期标签（长度 = trend.length）。 */
const DATES = ['07-25', '07-26', '07-27', '07-28', '07-29', '07-30', '07-31']

describe('buildVitalityOption', () => {
  const option = buildVitalityOption(TREND, DATES, PALETTE)

  it('x 轴日期连续：data = 传入日期标签（长度 = trend.length）', () => {
    const xAxis = option.xAxis as { data: string[]; type: string }
    expect(xAxis.type).toBe('category')
    expect(xAxis.data).toHaveLength(TREND.length)
    expect(xAxis.data).toEqual(DATES)
  })

  it('x 轴开启轻量标签：首尾可见、中间隐藏、muted 色小字号', () => {
    const xAxis = option.xAxis as {
      axisLabel: {
        show?: boolean
        color: string
        fontSize: number
        interval: (index: number) => boolean
      }
    }
    expect(xAxis.axisLabel.show).not.toBe(false)
    expect(xAxis.axisLabel.color).toBe(PALETTE.textMuted)
    expect(xAxis.axisLabel.fontSize).toBeLessThanOrEqual(10)
    expect(xAxis.axisLabel.interval(0)).toBe(true)
    expect(xAxis.axisLabel.interval(DATES.length - 1)).toBe(true)
    expect(xAxis.axisLabel.interval(2)).toBe(false)
  })

  it('series 为 line + areaStyle（面积图）', () => {
    const series = option.series as Array<{ type: string; areaStyle: unknown; data: number[] }>
    expect(series).toHaveLength(1)
    expect(series[0]!.type).toBe('line')
    expect(series[0]!.areaStyle).toBeDefined()
    expect(series[0]!.data).toEqual(TREND)
  })

  it('tooltip 为 axis 触发（轻量 tooltip）', () => {
    const tooltip = option.tooltip as { trigger: string }
    expect(tooltip.trigger).toBe('axis')
  })

  it('tooltip formatter 输出 "N 节"', () => {
    const tooltip = option.tooltip as { formatter: (p: unknown) => string }
    expect(tooltip.formatter([{ value: 12, axisValue: 3 }])).toBe('12 节')
  })

  it('含轻量坐标轴（xAxis 有 axisLine）', () => {
    const xAxis = option.xAxis as { axisLine: unknown }
    expect(xAxis.axisLine).toBeDefined()
  })
})
