import { describe, expect, it } from 'vitest'
import {
  filterFocusNodes,
  isDragMove,
  buildFocusMeta,
} from './topicFocus'
import type { SectionTimelineNode } from '~/api/dailyReports'

// ---- fixtures ----
function makeNode(
  id: number,
  over: Partial<SectionTimelineNode> & { topicId?: number; date?: string } = {},
): SectionTimelineNode {
  const topicId = over.topicId ?? 5
  return {
    id,
    report_id: 1,
    period_date: `${over.date ?? '2026-06-18'}T12:00:00Z`,
    cluster_label: `节点${id}`,
    status: 'continuing',
    article_count: 2,
    thread_count: 1,
    persistent_topic_id: topicId,
    persistent_topic: {
      id: topicId,
      label: topicId === 5 ? '以黎冲突' : '他话题',
      status: 'active',
      color: '#b44f45',
      consecutive_hits: 3,
      can_activate: false,
    },
    ...over,
  }
}

describe('filterFocusNodes — 横向时间轴仅渲染该话题节点 (Scenario)', () => {
  it('keeps only nodes whose persistent_topic_id matches the focused topic', () => {
    const sections = [
      makeNode(1, { topicId: 5, date: '2026-06-18' }),
      makeNode(2, { topicId: 7, date: '2026-06-18' }),
      makeNode(3, { topicId: 5, date: '2026-06-20' }),
      makeNode(4, { topicId: 7, date: '2026-06-21' }),
    ]
    const out = filterFocusNodes(sections, 5)
    expect(out.map(n => n.id)).toEqual([1, 3])
  })

  it('returns an empty array when the topic has no nodes in the window (empty-topic fallback basis)', () => {
    const sections = [makeNode(1, { topicId: 7 }), makeNode(2, { topicId: 8 })]
    expect(filterFocusNodes(sections, 5)).toEqual([])
  })

  it('does not crash on an empty section list', () => {
    expect(filterFocusNodes([], 5)).toEqual([])
  })

  it('ignores nodes whose persistent_topic_id is undefined (unassigned)', () => {
    const orphan = makeNode(9, { topicId: 5 })
    delete orphan.persistent_topic_id
    const sections = [orphan, makeNode(1, { topicId: 5 })]
    expect(filterFocusNodes(sections, 5).map(n => n.id)).toEqual([1])
  })
})

describe('isDragMove — 拖拽平移时间轴: 区分 click 与 drag (Scenario)', () => {
  it('treats sub-threshold movement as a click (does NOT suppress)', () => {
    expect(isDragMove(2, 3)).toBe(false)
    expect(isDragMove(0, 3)).toBe(false)
    expect(isDragMove(-2, 3)).toBe(false)
  })

  it('treats movement beyond the threshold as a drag (suppresses the following click)', () => {
    expect(isDragMove(3, 3)).toBe(false) // boundary itself is still a click
    expect(isDragMove(4, 3)).toBe(true)
    expect(isDragMove(-10, 3)).toBe(true)
  })

  it('uses absolute distance so direction does not matter', () => {
    expect(isDragMove(-4, 3)).toBe(true)
    expect(isDragMove(4, 3)).toBe(true)
  })
})

describe('buildFocusMeta — sticky 标题元信息 + 空话题降级 (Scenarios)', () => {
  it('counts dynamics, span and latest date across the topic nodes', () => {
    const nodes = [
      makeNode(1, { topicId: 5, date: '2026-06-16' }),
      makeNode(2, { topicId: 5, date: '2026-06-29' }),
      makeNode(3, { topicId: 5, date: '2026-06-20' }),
    ]
    const meta = buildFocusMeta(nodes)
    expect(meta.count).toBe(3)
    expect(meta.firstDate).toBe('2026-06-16')
    expect(meta.lastDate).toBe('2026-06-29')
    // 06-16 .. 06-29 inclusive = 14 days
    expect(meta.spanDays).toBe(13)
  })

  it('handles a single node (zero span)', () => {
    const meta = buildFocusMeta([makeNode(1, { topicId: 5, date: '2026-06-18' })])
    expect(meta.count).toBe(1)
    expect(meta.firstDate).toBe('2026-06-18')
    expect(meta.lastDate).toBe('2026-06-18')
    expect(meta.spanDays).toBe(0)
  })

  it('returns an empty meta (count 0) for an empty topic — fallback signal, no throw', () => {
    const meta = buildFocusMeta([])
    expect(meta.count).toBe(0)
    expect(meta.firstDate).toBeNull()
    expect(meta.lastDate).toBeNull()
    expect(meta.spanDays).toBe(0)
    expect(meta.empty).toBe(true)
  })

  it('does not mutate the input order (read-only aggregation)', () => {
    const nodes = [
      makeNode(1, { topicId: 5, date: '2026-06-29' }),
      makeNode(2, { topicId: 5, date: '2026-06-16' }),
    ]
    const snapshot = nodes.map(n => n.id)
    buildFocusMeta(nodes)
    expect(nodes.map(n => n.id)).toEqual(snapshot)
  })
})
