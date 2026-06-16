/**
 * Unit tests for detective-wall pure utilities (no WebGL needed).
 *
 * @see specs/detective-wall-interaction/spec.md §BFS Lifeline Algorithm
 */
import { describe, it, expect } from 'vitest'
import { bfsLifeline, layoutCards, densityForDays, edgeKey, timelineWidth, latestDayX } from './utils'
import { STYLE } from './types'
import type { SectionTimelineNode, SectionRelation } from '~/api/dailyReports'

// --- helpers ---
function node(id: number, date: string, articleCount = 1): SectionTimelineNode {
  return {
    id,
    report_id: 0,
    period_date: date,
    cluster_label: `topic-${id}`,
    status: 'continuing',
    article_count: articleCount,
    thread_count: 1,
  }
}
function rel(from: number, to: number, distance = 1): SectionRelation {
  return { from_id: from, to_id: to, distance }
}
function nodeMap(nodes: SectionTimelineNode[]) {
  return new Map(nodes.map(n => [n.id, n] as const))
}

// ---------------------------------------------------------------------------
// edgeKey
// ---------------------------------------------------------------------------

describe('edgeKey', () => {
  it('normalizes direction (a-b == b-a)', () => {
    expect(edgeKey(3, 7)).toBe(edgeKey(7, 3))
    expect(edgeKey(3, 7)).toBe('3-7')
  })
  it('handles equal ids', () => {
    expect(edgeKey(5, 5)).toBe('5-5')
  })
})

// ---------------------------------------------------------------------------
// bfsLifeline
// ---------------------------------------------------------------------------

describe('bfsLifeline', () => {
  it('excludes nodes outside the date window', () => {
    const nodes = [node(1, '2026-01-01'), node(2, '2026-01-02'), node(3, '2026-01-10')]
    const relations = [rel(1, 2), rel(2, 3)] // 3 is linked but out of window
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, relations, nodeMap(nodes), range)

    expect(result.nodes.has(1)).toBe(true)
    expect(result.nodes.has(2)).toBe(true)
    expect(result.nodes.has(3)).toBe(false) // out of window
  })

  it('returns just the focus for an isolated node', () => {
    const nodes = [node(1, '2026-01-01'), node(2, '2026-01-02')]
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, [], nodeMap(nodes), range)

    expect(result.nodes.size).toBe(1)
    expect(result.nodes.has(1)).toBe(true)
    expect(result.edges.size).toBe(0)
  })

  it('matches edges regardless of BFS traversal direction', () => {
    // relation stored as 2->1, but BFS from 1 walks it back to 2.
    const nodes = [node(1, '2026-01-01'), node(2, '2026-01-02')]
    const relations = [rel(2, 1)] // reverse direction
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, relations, nodeMap(nodes), range)

    expect(result.nodes.has(2)).toBe(true)
    // The normalized key must match what the relation produces.
    expect(result.edges.has(edgeKey(1, 2))).toBe(true)
    expect(result.edges.has(edgeKey(2, 1))).toBe(true) // same key
  })

  it('traverses a connected component within the window', () => {
    const nodes = [
      node(1, '2026-01-01'),
      node(2, '2026-01-02'),
      node(3, '2026-01-03'),
      node(4, '2026-01-04'),
    ]
    const relations = [rel(1, 2), rel(2, 3), rel(3, 4)]
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, relations, nodeMap(nodes), range)

    expect(result.nodes.size).toBe(4)
    expect(result.edges.size).toBe(3)
  })

  it('assigns BFS depth from the start node', () => {
    // Linear chain 1→2→3→4, plus a branch 2→5.
    const nodes = [
      node(1, '2026-01-01'),
      node(2, '2026-01-02'),
      node(3, '2026-01-03'),
      node(4, '2026-01-04'),
      node(5, '2026-01-02'),
    ]
    const relations = [rel(1, 2), rel(2, 3), rel(3, 4), rel(2, 5)]
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, relations, nodeMap(nodes), range)

    expect(result.depth.get(1)).toBe(0)
    expect(result.depth.get(2)).toBe(1)
    expect(result.depth.get(3)).toBe(2)
    expect(result.depth.get(4)).toBe(3)
    expect(result.depth.get(5)).toBe(2) // same depth as node 3 (both 2-hop neighbors of 2)
  })

  it('assigns depth 0 to an isolated focus', () => {
    const nodes = [node(1, '2026-01-01'), node(2, '2026-01-02')]
    const range = { start: '2026-01-01', end: '2026-01-05' }

    const result = bfsLifeline(1, [], nodeMap(nodes), range)

    expect(result.depth.get(1)).toBe(0)
    expect(result.depth.size).toBe(1)
  })
})

// ---------------------------------------------------------------------------
// layoutCards
// ---------------------------------------------------------------------------

describe('layoutCards', () => {
  it('stacks same-day cards vertically with column width spacing', () => {
    const sections = [
      node(1, '2026-01-01', 5),
      node(2, '2026-01-01', 3), // same day, fewer articles
      node(3, '2026-01-02', 1),
    ]
    const layout = layoutCards(sections)

    const p1 = layout.get(1)!.position
    const p2 = layout.get(2)!.position
    const p3 = layout.get(3)!.position

    // Same X (same day), different Y (stacked).
    expect(p1.x).toBe(p2.x)
    expect(p1.y).not.toBe(p2.y)
    // Day 2 is one column to the right.
    expect(p3.x - p1.x).toBeCloseTo(STYLE.layout.colWidth, 5)
  })

  it('keeps Z jitter and rotation within bounds', () => {
    const sections = [node(1, '2026-01-01'), node(2, '2026-01-02')]
    const layout = layoutCards(sections)
    const rotMax = (STYLE.layout.rotationZDeg * Math.PI) / 180

    for (const result of layout.values()) {
      expect(Math.abs(result.position.z)).toBeLessThanOrEqual(STYLE.layout.zJitter + 1e-9)
      expect(Math.abs(result.rotationZ)).toBeLessThanOrEqual(rotMax + 1e-9)
    }
  })

  it('is deterministic across calls (seeded jitter)', () => {
    const sections = [node(1, '2026-01-01'), node(2, '2026-01-01')]
    const a = layoutCards(sections)
    const b = layoutCards(sections)
    expect(a.get(1)!.position.z).toBe(b.get(1)!.position.z)
    expect(a.get(1)!.rotationZ).toBe(b.get(1)!.rotationZ)
  })
})

// ---------------------------------------------------------------------------
// densityForDays
// ---------------------------------------------------------------------------

describe('densityForDays', () => {
  it('maps supported day windows to spec values', () => {
    expect(densityForDays(7)).toBe(0.08)
    expect(densityForDays(14)).toBe(0.05)
    expect(densityForDays(30)).toBe(0.03)
    expect(densityForDays(60)).toBe(0.02)
  })

  it('falls back to nearest bucket for unsupported values', () => {
    // 10 is between 7 and 14; nearest is 7.
    expect(densityForDays(10)).toBe(0.08)
    // 20 is between 14 and 30; nearest is 14.
    expect(densityForDays(20)).toBe(0.05)
  })
})

// ---------------------------------------------------------------------------
// timeline width / latestDayX
// ---------------------------------------------------------------------------

describe('timelineWidth / latestDayX', () => {
  it('computes width from day count', () => {
    expect(timelineWidth(1)).toBe(0)
    expect(timelineWidth(7)).toBeCloseTo(6 * STYLE.layout.colWidth, 5)
  })

  it('finds the latest day X', () => {
    const sections = [node(1, '2026-01-03'), node(2, '2026-01-01'), node(3, '2026-01-02')]
    // 3 days → latest (index 2) × colWidth
    expect(latestDayX(sections)).toBeCloseTo(2 * STYLE.layout.colWidth, 5)
  })
})
