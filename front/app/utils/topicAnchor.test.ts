import { describe, expect, it } from 'vitest'
import {
  topicAnchorTier,
  topicAnchorLabel,
  TOPIC_ANCHOR_TIGHT_THRESHOLD,
  TOPIC_ANCHOR_LOOSE_THRESHOLD,
} from './topicAnchor'

describe('topicAnchorTier', () => {
  const cases: Array<[number | undefined | null, string | undefined | null, number]> = [
    // anchor_hit 三档
    [0.02, 'anchor_hit', 0], // 极紧
    [0.05, 'anchor_hit', 0], // 边界 ≤0.05 → 极紧（spec scenario）
    [0.0501, 'anchor_hit', 1], // 稳锚
    [0.10, 'anchor_hit', 1],
    [0.15, 'anchor_hit', 1], // 边界 ≤0.15 → 稳锚（spec scenario）
    [0.1501, 'anchor_hit', 2], // 松锚
    [0.27, 'anchor_hit', 2],
    [0.30, 'anchor_hit', 2],
    // auto_new 恒为档3，忽略 distance
    [0.1, 'auto_new', 3],
    [0.4, 'auto_new', 3],
    [undefined, 'auto_new', 3],
    // unmatched / 缺失 → 档4，忽略 distance
    [0.1, 'unmatched', 4],
    [undefined, undefined, 4],
    [0.1, undefined, 4],
    [null, 'mystery', 4], // 未知 confidence
    // anchor_hit 但 distance 缺失/零值 → 防御降级档4（spec：缺失 distance → 未锚定）
    [undefined, 'anchor_hit', 4],
    [null, 'anchor_hit', 4],
    [0, 'anchor_hit', 4],
  ]
  it.each(cases)('distance=%s confidence=%s → tier %s', (d, c, expected) => {
    expect(topicAnchorTier(d, c)).toBe(expected)
  })

  it('exports the dual thresholds aligned to the 2026-06-26 measurement', () => {
    expect(TOPIC_ANCHOR_TIGHT_THRESHOLD).toBe(0.05)
    expect(TOPIC_ANCHOR_LOOSE_THRESHOLD).toBe(0.15)
  })
})

describe('topicAnchorLabel', () => {
  it('maps each tier to its chinese label', () => {
    expect(topicAnchorLabel(0.02, 'anchor_hit')).toBe('极紧锚定')
    expect(topicAnchorLabel(0.10, 'anchor_hit')).toBe('稳锚定')
    expect(topicAnchorLabel(0.27, 'anchor_hit')).toBe('松锚定')
    expect(topicAnchorLabel(0.1, 'auto_new')).toBe('新话题候选')
    expect(topicAnchorLabel(0.1, 'unmatched')).toBe('未锚定')
  })

  it('falls back to 未锚定 on missing data', () => {
    expect(topicAnchorLabel(undefined, undefined)).toBe('未锚定')
    expect(topicAnchorLabel(0, 'anchor_hit')).toBe('未锚定')
  })
})
