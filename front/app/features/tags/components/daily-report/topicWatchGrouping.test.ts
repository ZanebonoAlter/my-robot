import { describe, expect, it } from 'vitest'
import type { TopicWatch, TopicWatchHit } from '~/api/topicWatches'
import {
  WATCH_COLLAPSE_THRESHOLD,
  buildSectionTitleLookup,
  formatMoreLabel,
  groupHitsByWatch,
  partitionByStatus,
} from './topicWatchGrouping'

function watch(over: Partial<TopicWatch> = {}): TopicWatch {
  return {
    id: '1',
    semanticBoardId: '1974',
    label: 'w',
    status: 'active',
    createdAt: 'a',
    updatedAt: 'b',
    ...over,
  }
}

function hit(over: Partial<TopicWatchHit> = {}): TopicWatchHit {
  return {
    id: '10',
    watchId: '1',
    sectionId: '100',
    reportId: '9',
    periodDate: '2026-06-29',
    reason: 'r',
    ...over,
  }
}

describe('buildSectionTitleLookup', () => {
  it('maps section id (stringified) → cluster_label', () => {
    const map = buildSectionTitleLookup([
      { id: 100, cluster_label: '霍尔木兹油轮遇袭' },
      { id: 130, cluster_label: 'IAEA 浓缩铀' },
    ])
    expect(map.get('100')).toBe('霍尔木兹油轮遇袭')
    expect(map.get('130')).toBe('IAEA 浓缩铀')
  })

  it('treats missing cluster_label as empty string (defensive)', () => {
    const map = buildSectionTitleLookup([{ id: 1, cluster_label: undefined as unknown as string }])
    expect(map.get('1')).toBe('')
  })
})

describe('groupHitsByWatch', () => {
  const sections = buildSectionTitleLookup([
    { id: 123, cluster_label: '海峡油轮遇袭' },
    { id: 130, cluster_label: '浓缩铀纯度突破' },
    { id: 200, cluster_label: 'AI Act 执行细则' },
  ])

  it('groups hits by watch in watch-list order, joining section titles', () => {
    const watches = [
      watch({ id: '1', label: '美伊会不会真打起来' }),
      watch({ id: '2', label: 'AI 监管立法进展' }),
    ]
    const hits = [
      hit({ id: 'a', watchId: '2', sectionId: '200', reason: '执行阶段里程碑' }),
      hit({ id: 'b', watchId: '1', sectionId: '123', reason: '事态升级' }),
      hit({ id: 'c', watchId: '1', sectionId: '130', reason: '武器级前置' }),
    ]

    const groups = groupHitsByWatch(hits, watches, sections)

    // watch-list order preserved (w1 before w2), not hit-insertion order
    expect(groups.map(g => g.watch.id)).toEqual(['1', '2'])
    expect(groups[0]!.items.map(i => i.sectionTitle)).toEqual(['海峡油轮遇袭', '浓缩铀纯度突破'])
    expect(groups[0]!.items.map(i => i.hit.reason)).toEqual(['事态升级', '武器级前置'])
    expect(groups[1]!.items[0]!.sectionTitle).toBe('AI Act 执行细则')
  })

  it('drops watches with zero hits (no empty groups)', () => {
    const watches = [watch({ id: '1' }), watch({ id: '2' })]
    const hits = [hit({ watchId: '1', sectionId: '123' })]
    const groups = groupHitsByWatch(hits, watches, sections)
    expect(groups).toHaveLength(1)
    expect(groups[0]!.watch.id).toBe('1')
  })

  it('drops orphan hits whose watch is not in the list (deleted-between-fetch safety)', () => {
    const watches = [watch({ id: '1' })]
    const hits = [
      hit({ watchId: '1', sectionId: '123' }),
      hit({ id: 'x', watchId: '999', sectionId: '200' }),
    ]
    const groups = groupHitsByWatch(hits, watches, sections)
    expect(groups).toHaveLength(1)
    expect(groups[0]!.items).toHaveLength(1)
  })

  it('resolves unknown sectionId to empty title (stale hit graceful degrade)', () => {
    const groups = groupHitsByWatch(
      [hit({ watchId: '1', sectionId: '404' })],
      [watch({ id: '1' })],
      sections,
    )
    expect(groups[0]!.items[0]!.sectionTitle).toBe('')
  })

  it('threshold constant is 2 (mockup shows 2 then “还有 N 条”)', () => {
    expect(WATCH_COLLAPSE_THRESHOLD).toBe(2)
  })
})

describe('formatMoreLabel', () => {
  it('formats the remaining count', () => {
    expect(formatMoreLabel(1)).toBe('还有 1 条命中 ↓')
    expect(formatMoreLabel(3)).toBe('还有 3 条命中 ↓')
  })
})

describe('partitionByStatus', () => {
  it('splits active vs paused', () => {
    const { active, paused } = partitionByStatus([
      watch({ id: '1', status: 'active' }),
      watch({ id: '2', status: 'paused' }),
      watch({ id: '3', status: 'active' }),
      watch({ id: '4', status: 'paused' }),
    ])
    expect(active.map(w => w.id)).toEqual(['1', '3'])
    expect(paused.map(w => w.id)).toEqual(['2', '4'])
  })
})
