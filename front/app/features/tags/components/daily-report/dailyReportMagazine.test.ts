import { describe, expect, it, vi } from 'vitest'
import {
  buildBezierPath,
  buildLifelineWindow,
  buildQualityZones,
  createRequestCache,
  selectLeadStory,
} from './dailyReportMagazine'
import type { DailyReport, DailyReportSection, SectionRelation, SectionTimelineNode } from '~/api/dailyReports'

function section(
  id: number,
  tier: number,
  score: number,
  status?: string,
  topicStatusAtReport?: string | null,
): DailyReportSection {
  return {
    id,
    cluster_index: id,
    cluster_label: `section-${id}`,
    cluster_tag_ids: [],
    threads: [],
    article_count: 1,
    best_tier: tier,
    avg_score: score,
    persistent_topic_id: status || topicStatusAtReport != null ? id : undefined,
    persistent_topic: status ? {
      id,
      label: `topic-${id}`,
      status,
      color: '#aa0000',
      consecutive_hits: 1,
      can_activate: false,
    } : undefined,
    topic_status_at_report: topicStatusAtReport,
  }
}

describe('dailyReportMagazine data mapping', () => {
  it('sorts sections inside active and briefs zones by snapshot', () => {
    const zones = buildQualityZones([
      section(1, 2, 0.5, 'active', 'active'),
      section(2, 0, 0.9, 'candidate', 'candidate'),
      section(3, 1, 0.7, 'active', 'active'),
      section(4, 3, 0.1, undefined, null),
    ])

    expect(zones.map(zone => zone.key)).toEqual(['active', 'briefs'])
    expect(zones[0]?.sections.map(item => item.id)).toEqual([3, 1])
  })

  it('puts snapshot=active into "关心的话题"', () => {
    const zones = buildQualityZones([section(1, 0, 0.9, 'active', 'active')])
    expect(zones).toHaveLength(1)
    expect(zones[0]!.key).toBe('active')
    expect(zones[0]!.label).toBe('关心的话题')
  })

  it('puts snapshot=candidate into "其他动态" (briefs)', () => {
    const zones = buildQualityZones([section(1, 0, 0.9, 'candidate', 'candidate')])
    expect(zones).toHaveLength(1)
    expect(zones[0]!.key).toBe('briefs')
    expect(zones[0]!.label).toBe('其他动态')
  })

  it('puts snapshot=null into "其他动态"', () => {
    const zones = buildQualityZones([section(1, 0, 0.9, 'active', null)])
    expect(zones).toHaveLength(1)
    expect(zones[0]!.key).toBe('briefs')
  })

  it('puts missing snapshot (undefined) into "其他动态"', () => {
    const zones = buildQualityZones([section(1, 0, 0.9, 'active')])
    expect(zones).toHaveLength(1)
    expect(zones[0]!.key).toBe('briefs')
  })

  it('does not give candidates sorting bonus — briefs zone sorted only by (tier, score)', () => {
    const zones = buildQualityZones([
      section(1, 1, 0.3, 'candidate', 'candidate'),
      section(2, 1, 0.7, 'active', 'active'),
      section(3, 1, 0.5, 'active', 'candidate'),
    ])
    const briefs = zones.find(z => z.key === 'briefs')!
    // Both sections in briefs sorted by tier ASC, score DESC; candidate gets no bonus.
    expect(briefs.sections.map(s => s.id)).toEqual([3, 1])
  })

  it('uses snapshot, not current topic.status, for zoning (immutable)', () => {
    // section was active at report time; topic later archived
    const zones = buildQualityZones([section(1, 0, 0.9, 'archived', 'active')])
    expect(zones[0]!.key).toBe('active')
    expect(zones[0]!.label).toBe('关心的话题')
  })

  it('no longer renders three zones or "突发的新话题" label', () => {
    const zones = buildQualityZones([
      section(1, 0, 0.9, 'active', 'active'),
      section(2, 1, 0.5, 'candidate', 'candidate'),
      section(3, 2, 0.3),
    ])
    expect(zones.map(z => z.key)).toEqual(['active', 'briefs'])
    expect(zones.flatMap(z => z.sections.map(s => s.id))).toEqual(expect.arrayContaining([1, 2, 3]))
  })

  it('uses the first highlight, then falls back to the best section', () => {
    const base = {
      id: 1,
      semantic_board_id: 1,
      period_date: '2026-06-21',
      title: '日报',
      summary: '摘要',
      status: 'done',
      cluster_count: 2,
      article_count: 2,
      event_tag_count: 2,
      created_at: '2026-06-21',
      dynamics: '',
      sections: [section(1, 2, 0.3), section(2, 0, 0.9)],
    } satisfies Omit<DailyReport, 'highlights'>

    expect(selectLeadStory({ ...base, highlights: [{ title: '头条', reason: '原因', tag_ids: [] }] })?.title).toBe('头条')
    expect(selectLeadStory({ ...base, highlights: [] })?.title).toBe('section-2')
  })

  it('builds a seven-day window, aggregates same-day nodes, and keeps identity edges only', () => {
    const nodes = [
      { id: 1, report_id: 1, period_date: '2026-06-19', cluster_label: 'A' },
      { id: 2, report_id: 2, period_date: '2026-06-21', cluster_label: 'B' },
      { id: 3, report_id: 2, period_date: '2026-06-21', cluster_label: 'C' },
      { id: 4, report_id: 3, period_date: '2026-06-01', cluster_label: 'outside' },
    ] as SectionTimelineNode[]
    const relations = [
      { from_id: 1, to_id: 2, distance: 0, relation_type: 'identity' },
      { from_id: 1, to_id: 3, distance: 0.1, relation_type: 'similarity' },
      { from_id: 4, to_id: 1, distance: 0, relation_type: 'identity' },
    ] satisfies SectionRelation[]

    const result = buildLifelineWindow(nodes, relations, '2026-06-21', 7)

    expect(result.days).toHaveLength(7)
    expect(result.days.find(day => day.key === '2026-06-20')?.sections).toHaveLength(0)
    expect(result.days.find(day => day.key === '2026-06-21')?.sections).toHaveLength(2)
    expect(result.edges).toEqual([expect.objectContaining({ fromDayKey: '2026-06-19', toDayKey: '2026-06-21', weak: true })])
  })

  it('marks edges crossing empty days as weak and adjacent edges as strong', () => {
    const nodes = [
      { id: 1, report_id: 1, period_date: '2026-06-20', cluster_label: 'A' },
      { id: 2, report_id: 2, period_date: '2026-06-21', cluster_label: 'B' },
      { id: 3, report_id: 3, period_date: '2026-06-15', cluster_label: 'C' },
    ] as SectionTimelineNode[]
    const relations = [
      { from_id: 1, to_id: 2, distance: 0, relation_type: 'identity' },
      { from_id: 3, to_id: 1, distance: 0, relation_type: 'identity' },
    ] satisfies SectionRelation[]

    const result = buildLifelineWindow(nodes, relations, '2026-06-21', 7)
    const adjacent = result.edges.find(edge => edge.fromSectionId === 1 && edge.toSectionId === 2)
    const crossing = result.edges.find(edge => edge.fromSectionId === 3 && edge.toSectionId === 1)

    expect(adjacent?.weak).toBe(false)
    expect(crossing?.weak).toBe(true)
  })

  it('uses the shared cubic bezier geometry', () => {
    expect(buildBezierPath(10, 20, 110, 60)).toBe('M10,20 C60,20 60,60 110,60')
  })
})

describe('createRequestCache', () => {
  it('deduplicates pending and successful requests', async () => {
    const loader = vi.fn(async (id: number) => `item-${id}`)
    const cache = createRequestCache(loader)

    const [first, second] = await Promise.all([cache.load(7), cache.load(7)])
    const third = await cache.load(7)

    expect([first, second, third]).toEqual(['item-7', 'item-7', 'item-7'])
    expect(loader).toHaveBeenCalledTimes(1)
  })

  it('isolates failures and retries only when forced', async () => {
    const loader = vi.fn()
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce('recovered')
    const cache = createRequestCache<number, string>(loader)

    expect(await cache.load(1)).toBeUndefined()
    expect(cache.get(1)).toEqual(expect.objectContaining({ status: 'error', error: 'network' }))
    expect(await cache.load(1)).toBeUndefined()
    expect(loader).toHaveBeenCalledTimes(1)
    expect(await cache.load(1, true)).toBe('recovered')
    expect(cache.get(1).status).toBe('success')
  })

  it('clears board-scoped entries', async () => {
    const cache = createRequestCache(async (id: number) => id)
    await cache.load(1)
    cache.clear()
    expect(cache.get(1).status).toBe('idle')
  })
})
