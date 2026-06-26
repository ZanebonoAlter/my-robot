import { describe, expect, it } from 'vitest'
import {
  isThreadFitDemoted,
  threadFitLabel,
  THREAD_FIT_DEMOTE_THRESHOLD,
} from './threadFit'

describe('isThreadFitDemoted', () => {
  // 边界值一律用常量引用（标定后改 threadFit.ts 一处常量，测试自动适应；禁止硬编码 0.20）。
  const cases: Array<[number | undefined | null, boolean]> = [
    // 边界：严格 > 才降级，阈值本身不降级
    [THREAD_FIT_DEMOTE_THRESHOLD, false],
    [THREAD_FIT_DEMOTE_THRESHOLD + 0.001, true], // 刚超阈值 → 离群
    [THREAD_FIT_DEMOTE_THRESHOLD - 0.001, false], // 刚低于阈值 → 贴合
    // 明显贴合
    [0.05, false],
    [0, false], // 完美贴合（后端 *float64 非 nil 的 0.0）
    // 容错：异常输入不当离群
    [-1, false], // 负数
    [Number.NaN, false], // NaN
    // 无信号：历史 thread 不降级
    [undefined, false],
    [null, false],
  ]
  it.each(cases)('fit_distance=%s → isThreadFitDemoted=%s', (fit, expected) => {
    expect(isThreadFitDemoted(fit)).toBe(expected)
  })
})

describe('threadFitLabel', () => {
  const cases: Array<[number | undefined | null, string]> = [
    // 边界 ≤ 阈值 → 贴合；> 阈值 → 可能跑题
    [THREAD_FIT_DEMOTE_THRESHOLD, '贴合'],
    [THREAD_FIT_DEMOTE_THRESHOLD + 0.001, '可能跑题'],
    [THREAD_FIT_DEMOTE_THRESHOLD - 0.001, '贴合'],
    // 有效且 ≤ 阈值
    [0.05, '贴合'],
    [0, '贴合'],
    [-1, '贴合'], // 负数视为有效（容错），归入贴合
    // 无信号
    [Number.NaN, '无贴合信号'],
    [undefined, '无贴合信号'],
    [null, '无贴合信号'],
  ]
  it.each(cases)('fit_distance=%s → label=%s', (fit, expected) => {
    expect(threadFitLabel(fit)).toBe(expected)
  })
})

describe('boundary consistency (label ↔ isThreadFitDemoted)', () => {
  // 两者共用同一离群边界：严格 > 阈值 = 离群。
  it('treats the threshold itself as non-demoted (贴合)', () => {
    expect(isThreadFitDemoted(THREAD_FIT_DEMOTE_THRESHOLD)).toBe(false)
    expect(threadFitLabel(THREAD_FIT_DEMOTE_THRESHOLD)).toBe('贴合')
  })

  it('treats a value just above the threshold as demoted (可能跑题)', () => {
    const justOver = THREAD_FIT_DEMOTE_THRESHOLD + 0.001
    expect(isThreadFitDemoted(justOver)).toBe(true)
    expect(threadFitLabel(justOver)).toBe('可能跑题')
  })
})
