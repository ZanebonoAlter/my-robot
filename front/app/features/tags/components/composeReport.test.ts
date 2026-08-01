import { describe, expect, it } from 'vitest'
import {
  aggregatePreview,
  cosineDistance,
  crashReport,
  distanceTier,
  filterPoolByRange,
  outlierFlags,
  rankCandidates,
  TIER_LABEL,
} from './composeReport'
import type { ComposeCandidate } from '~/api/persistentTopics'

function cand(id: string, opts: Partial<ComposeCandidate> = {}): ComposeCandidate {
  return {
    id,
    periodDate: opts.periodDate ?? '2026-06-20T00:00:00Z',
    clusterLabel: opts.clusterLabel ?? `c${id}`,
    embedding: opts.embedding ?? [1, 0, 0],
    persistentTopicId: opts.persistentTopicId,
    persistentTopic: opts.persistentTopic,
  }
}

describe('aggregatePreview — mean pooling（镜像后端 aggregateEmbeddings）', () => {
  it('averages same-dimension vectors', () => {
    const r = aggregatePreview([[1, 2, 3], [4, 5, 6], [7, 8, 9]])
    expect(r.skipped).toBe(0)
    expect(r.mean).toEqual([4, 5, 6])
  })

  it('skips dimension-mismatched vectors', () => {
    const r = aggregatePreview([[1, 2, 3], [4, 5], [7, 8, 9]])
    expect(r.skipped).toBe(1)
    // mean of first and third only
    expect(r.mean).toEqual([4, 5, 6])
  })

  it('skips empty/nil vectors', () => {
    const r = aggregatePreview([[1, 2, 3], [], null as unknown as number[]])
    expect(r.skipped).toBe(2)
    expect(r.mean).toEqual([1, 2, 3])
  })

  it('returns null mean when all vectors unusable', () => {
    expect(aggregatePreview([])).toEqual({ mean: null, skipped: 0 })
    expect(aggregatePreview([[], []])).toEqual({ mean: null, skipped: 2 })
  })

  it('single vector → itself', () => {
    expect(aggregatePreview([[3.5, -2.1]])).toEqual({ mean: [3.5, -2.1], skipped: 0 })
  })
})

describe('outlierFlags — distance > threshold×1.3（镜像后端 detectOutliers）', () => {
  it('marks distances above cutoff only', () => {
    // cutoff = 0.3 * 1.3 = 0.39
    expect(outlierFlags([0.1, 0.8, 0.12], 0.3)).toEqual([false, true, false])
  })

  it('boundary (exactly cutoff) is NOT an outlier', () => {
    const boundary = 0.3 * 1.3
    expect(outlierFlags([0.1, boundary, boundary + 1e-9], 0.3)).toEqual([false, false, true])
  })

  it('returns null for empty input', () => {
    expect(outlierFlags([], 0.3)).toBeNull()
    expect(outlierFlags(null as unknown as number[], 0.3)).toBeNull()
  })
})

describe('distanceTier', () => {
  it('classifies good / boundary / outlier / far by threshold', () => {
    const t = 0.3
    expect(distanceTier(0.1, t)).toBe('good')
    expect(distanceTier(0.3, t)).toBe('good') // boundary inclusive of threshold → good
    expect(distanceTier(0.31, t)).toBe('boundary')
    expect(distanceTier(0.3 * 1.3, t)).toBe('boundary')
    expect(distanceTier(0.3 * 1.3 + 0.01, t)).toBe('outlier')
    expect(distanceTier(0.3 * 2, t)).toBe('outlier')
    expect(distanceTier(0.3 * 2 + 0.01, t)).toBe('far')
  })

  it('TIER_LABEL exposes Chinese labels', () => {
    expect(TIER_LABEL.good).toBe('贴合')
    expect(TIER_LABEL.boundary).toBe('边界')
    expect(TIER_LABEL.outlier).toBe('离群')
    expect(TIER_LABEL.far).toBe('远')
  })
})

describe('cosineDistance', () => {
  it('returns 0 for identical direction, 1 for orthogonal, 2 for opposite', () => {
    expect(cosineDistance([1, 0], [1, 0])).toBeCloseTo(0, 9)
    expect(cosineDistance([1, 0], [0, 1])).toBeCloseTo(1, 9)
    expect(cosineDistance([1, 0], [-1, 0])).toBeCloseTo(2, 9)
  })

  it('returns +Infinity for mismatched / empty / zero vectors', () => {
    expect(cosineDistance([1, 0], [1])).toBe(Number.POSITIVE_INFINITY)
    expect(cosineDistance([], [])).toBe(Number.POSITIVE_INFINITY)
    expect(cosineDistance([0, 0], [1, 1])).toBe(Number.POSITIVE_INFINITY)
  })
})

describe('crashReport — 撞车检查（归属分布 / 移出明细）', () => {
  const topics = [{ id: '7', label: '中东局势' }, { id: '9', label: '以黎冲突' }]

  it('aggregates current ownership distribution across selected', () => {
    const selected = [
      cand('1', { persistentTopicId: '7' }),
      cand('2', { persistentTopicId: '7' }),
      cand('3', { persistentTopicId: '7' }),
      cand('4', { persistentTopicId: '9' }),
      cand('5'), // unassigned
    ]
    const r = crashReport(selected, topics)
    // 中东局势 3, 以黎冲突 1, 未归属 1
    expect(r.distribution).toHaveLength(3)
    expect(r.distribution[0]).toEqual({ topicId: '7', label: '中东局势', count: 3 })
    expect(r.moveOutCount).toBe(4) // 3 + 1 (excludes unassigned)
    // Scenario「撞车明确提示移出」: 中东局势 3 条将移出
    const mid = r.moveOutByTopic.find(m => m.topicId === '7')!
    expect(mid.count).toBe(3)
    expect(mid.label).toBe('中东局势')
  })

  it('excludes unassigned from moveOutByTopic', () => {
    const r = crashReport([cand('1'), cand('2')], topics)
    expect(r.moveOutCount).toBe(0)
    expect(r.moveOutByTopic).toEqual([])
    expect(r.distribution).toEqual([{ topicId: '__unassigned__', label: '未归属', count: 2 }])
  })

  it('falls back to 话题 #id label when topic missing from existingTopics', () => {
    const r = crashReport([cand('1', { persistentTopicId: '42' })], topics)
    expect(r.moveOutByTopic[0]!.label).toBe('话题 #42')
  })

  it('handles empty selection', () => {
    expect(crashReport([], topics)).toEqual({ distribution: [], moveOutCount: 0, moveOutByTopic: [] })
  })
})

describe('filterPoolByRange — 时间窗口兜底过滤', () => {
  const pool = [
    cand('1', { periodDate: '2026-06-29T00:00:00Z' }),
    cand('2', { periodDate: '2026-06-20T00:00:00Z' }),
    cand('3', { periodDate: '2025-01-01T00:00:00Z' }),
  ]

  it('keeps only in-window sections (anchored to max date)', () => {
    // max=06-29, days=14 → cutoff=06-16 → keeps 06-29 & 06-20, drops 2025
    const out = filterPoolByRange(pool, 14)
    expect(out.map(c => c.id)).toEqual(['1', '2'])
  })

  it('days<=0 returns all history', () => {
    expect(filterPoolByRange(pool, 0)).toHaveLength(3)
    expect(filterPoolByRange(pool, -1)).toHaveLength(3)
  })

  it('does not mutate input', () => {
    const copy = [...pool]
    filterPoolByRange(pool, 14)
    expect(pool).toEqual(copy)
  })

  it('empty pool → empty', () => {
    expect(filterPoolByRange([], 14)).toEqual([])
  })
})

describe('rankCandidates — 语义渐进收敛排序（镜像 spec「候选池语义搜索」）', () => {
  const pool = [
    cand('1', { embedding: [1, 0] }),
    cand('2', { embedding: [0, 1] }),
    cand('3', { embedding: [-1, 0] }),
  ]

  it('anchor 非空时按到 anchor 距离升序', () => {
    expect(rankCandidates(pool, [1, 0], null).map(c => c.id)).toEqual(['1', '2', '3'])
  })

  it('anchor 优先于 queryVec（已选接管排序）', () => {
    // anchor=[1,0], queryVec=[0,1] → 按 anchor 排，queryVec 被忽略
    expect(rankCandidates(pool, [1, 0], [0, 1]).map(c => c.id)).toEqual(['1', '2', '3'])
  })

  it('anchor 为空时退回 queryVec 排序（冷启动）', () => {
    // queryVec=[0,1]: '2'(d=0) → '1'(d=1) → '3'(d=1)，stable 保 '1' 在 '3' 前
    expect(rankCandidates(pool, null, [0, 1]).map(c => c.id)).toEqual(['2', '1', '3'])
  })

  it('anchor 和 queryVec 都为空时保持原序', () => {
    expect(rankCandidates(pool, null, null).map(c => c.id)).toEqual(['1', '2', '3'])
  })

  it('不修改入参数组', () => {
    const copy = [...pool]
    rankCandidates(pool, [1, 0], null)
    expect(pool).toEqual(copy)
  })

  it('维度不匹配的候选排到末尾', () => {
    const mixed = [
      cand('1', { embedding: [1, 0] }),
      cand('2', { embedding: [0.9, 0.1] }),
      cand('x', { embedding: [1, 0, 0] }), // 维度不匹配 → +∞ → 末尾
    ]
    const out = rankCandidates(mixed, [1, 0], null)
    expect(out[out.length - 1]!.id).toBe('x')
  })

  it('空 ref 向量退回原序', () => {
    expect(rankCandidates(pool, [], []).map(c => c.id)).toEqual(['1', '2', '3'])
  })

  it('空池返回空数组', () => {
    expect(rankCandidates([], [1, 0], null)).toEqual([])
  })
})
