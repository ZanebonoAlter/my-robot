/**
 * Pure, testable utilities for the detective topic wall.
 *
 * Extracted from Three.js object code so they can be unit-tested without a
 * WebGL context (see bfsLifeline.test.ts / layout.test.ts / fog.test.ts).
 *
 * @see openspec/changes/detective-topic-wall/specs/detective-wall-interaction/spec.md §BFS Lifeline Algorithm
 */
import { Vector3 } from 'three'
import type { DateRange, LayoutResult, SectionRelation, SectionTimelineNode } from './types'
import { STYLE } from './types'

// ---------------------------------------------------------------------------
// Edge key normalization (undirected)
// ---------------------------------------------------------------------------

/**
 * Normalized undirected edge key. Independent of relation traversal direction,
 * so a BFS that walks `to→from` still matches the original `from→to` relation.
 *
 * Format: `${minId}-${maxId}`.
 */
export function edgeKey(a: number, b: number): string {
  return a < b ? `${a}-${b}` : `${b}-${a}`
}

// ---------------------------------------------------------------------------
// BFS lifeline (date-window constrained)
// ---------------------------------------------------------------------------

export interface LifelineResult {
  /** Node ids reachable from the start within the date window. */
  nodes: Set<number>
  /** Normalized edge keys traversed within the window. */
  edges: Set<string>
}

/**
 * Date-window-constrained BFS lifeline.
 *
 * Differs from existing `graphBfsHighlight.bfsHighlight`: that one returns a
 * connected component with small/dense heuristics and NO date constraint. This
 * one strictly excludes nodes outside `dateRange`. Treats edges as undirected.
 *
 * @see interaction spec §BFS Lifeline Algorithm (fixed version)
 */
export function bfsLifeline(
  startNodeId: number,
  relations: SectionRelation[],
  nodeMap: Map<number, SectionTimelineNode>,
  dateRange: DateRange,
): LifelineResult {
  const visited = new Set<number>([startNodeId])
  const edgeKeys = new Set<string>()
  const queue: number[] = [startNodeId]

  // Build undirected adjacency list.
  const adj = new Map<number, Set<number>>()
  const ensure = (id: number): Set<number> => {
    let set = adj.get(id)
    if (!set) {
      set = new Set()
      adj.set(id, set)
    }
    return set
  }
  for (const r of relations) {
    ensure(r.from_id).add(r.to_id)
    ensure(r.to_id).add(r.from_id)
  }
  ensure(startNodeId) // guarantee isolated focus exists

  while (queue.length > 0) {
    const current = queue.shift()!
    const neighbors = adj.get(current)
    if (!neighbors) continue

    for (const neighborId of neighbors) {
      if (visited.has(neighborId)) continue

      const node = nodeMap.get(neighborId)
      if (!node) continue

      // Core constraint: date window. Out-of-window nodes are skipped entirely.
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue

      visited.add(neighborId)
      queue.push(neighborId)
      edgeKeys.add(edgeKey(current, neighborId))
    }
  }

  return { nodes: visited, edges: edgeKeys }
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

/**
 * Deterministic-ish card layout. Same-day cards stack vertically by descending
 * article_count; X = day index × colWidth; Z = reproducible jitter; rotation.z
 * = reproducible ±rotationZDeg.
 *
 * Z jitter and rotation use a seeded pseudo-random source derived from the
 * section id, so re-layouts are stable (no flicker on re-render).
 *
 * @returns Map<sectionId, LayoutResult>
 */
export function layoutCards(
  sections: SectionTimelineNode[],
): Map<number, LayoutResult> {
  const { colWidth, rowHeight, zJitter, rotationZDeg } = STYLE.layout
  const rotRad = (rotationZDeg * Math.PI) / 180

  // Unique sorted day keys.
  const days = Array.from(new Set(sections.map(s => s.period_date.slice(0, 10)))).sort()
  const dayIndex = new Map<string, number>(days.map((d, i) => [d, i]))

  // Group sections by day.
  const byDay = new Map<string, SectionTimelineNode[]>()
  for (const s of sections) {
    const key = s.period_date.slice(0, 10)
    let list = byDay.get(key)
    if (!list) {
      list = []
      byDay.set(key, list)
    }
    list.push(s)
  }

  const result = new Map<number, LayoutResult>()
  for (const [day, list] of byDay) {
    // Descending article_count within the day.
    list.sort((a, b) => b.article_count - a.article_count)
    const x = (dayIndex.get(day) ?? 0) * colWidth

    list.forEach((s, row) => {
      const { z, rot } = seededJitter(s.id, zJitter, rotRad)
      result.set(s.id, {
        position: new Vector3(x, row * rowHeight, z),
        rotationZ: rot,
      })
    })
  }

  return result
}

/** Reproducible jitter from a numeric seed (mulberry-style hash → [0,1)). */
function seededUnit(seed: number): number {
  let t = (seed + 0x6d2b79f5) | 0
  t = Math.imul(t ^ (t >>> 15), t | 1)
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296
}

function seededJitter(seed: number, zRange: number, rotRange: number): { z: number; rot: number } {
  // Two independent draws.
  const u1 = seededUnit(seed)
  const u2 = seededUnit(seed + 1)
  const z = (u1 * 2 - 1) * zRange // [-zRange, zRange]
  const rot = (u2 * 2 - 1) * rotRange // [-rotRange, rotRange]
  return { z, rot }
}

// ---------------------------------------------------------------------------
// Fog density mapping
// ---------------------------------------------------------------------------

/** Resolve fog density for a day window; clamps to nearest supported value. */
export function densityForDays(days: number): number {
  const map = STYLE.fogDensityByDays
  if (map[days] != null) return map[days]
  // Nearest supported bucket.
  const sorted = Object.keys(map).map(Number).sort((a, b) => a - b)
  let nearest = sorted[0]!
  for (const d of sorted) {
    if (Math.abs(d - days) < Math.abs(nearest - days)) nearest = d
  }
  return map[nearest] ?? 0.05
}

// ---------------------------------------------------------------------------
// Camera shot helpers (no THREE side effects beyond Vector3 construction)
// ---------------------------------------------------------------------------

/** Total world width spanned by `n` day columns. */
export function timelineWidth(dayCount: number): number {
  return Math.max(0, dayCount - 1) * STYLE.layout.colWidth
}

/** Convenience: extract the X coordinate of the latest (rightmost) day. */
export function latestDayX(sections: SectionTimelineNode[]): number {
  const days = Array.from(new Set(sections.map(s => s.period_date.slice(0, 10)))).sort()
  const idx = days.length - 1
  return idx < 0 ? 0 : idx * STYLE.layout.colWidth
}
