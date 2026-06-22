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
  /** BFS hop count from the start node (start = 0). Keys mirror `nodes`. */
  depth: Map<number, number>
}

/**
 * Date-window-constrained BFS lifeline.
 *
 * Differs from existing `graphBfsHighlight.bfsHighlight`: that one returns a
 * connected component with small/dense heuristics and NO date constraint. This
 * one strictly excludes nodes outside `dateRange`. Treats edges as undirected.
 *
 * `presetNodes` (optional) seeds the visited set alongside the start node.
 * Used to fold in every section sharing the start's persistent topic, so a
 * narrative's sections all light up together even when the embedding-graph
 * edges between them were severed by label drift. Preset nodes are treated as
 * BFS roots (depth 0) and expand outward like the start.
 *
 * @see interaction spec §BFS Lifeline Algorithm (fixed version)
 */
export function bfsLifeline(
  startNodeId: number,
  relations: SectionRelation[],
  nodeMap: Map<number, SectionTimelineNode>,
  dateRange: DateRange,
  presetNodes?: Set<number>,
): LifelineResult {
  const visited = new Set<number>([startNodeId])
  const edgeKeys = new Set<string>()
  const depth = new Map<number, number>([[startNodeId, 0]])
  const queue: number[] = [startNodeId]

  // Seed preset topic-mates as BFS roots (depth 0). They must be in-window to
  // participate; out-of-window mates are ignored just like ordinary traversal.
  if (presetNodes) {
    for (const id of presetNodes) {
      if (id === startNodeId) continue
      const node = nodeMap.get(id)
      if (!node) continue
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue
      if (visited.has(id)) continue
      visited.add(id)
      depth.set(id, 0)
      queue.push(id)
    }
  }

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
  for (const id of visited) ensure(id)

  while (queue.length > 0) {
    const current = queue.shift()!
    const neighbors = adj.get(current)
    if (!neighbors) continue

    const currentDepth = depth.get(current) ?? 0

    for (const neighborId of neighbors) {
      if (visited.has(neighborId)) continue

      const node = nodeMap.get(neighborId)
      if (!node) continue

      // Core constraint: date window. Out-of-window nodes are skipped entirely.
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue

      visited.add(neighborId)
      queue.push(neighborId)
      depth.set(neighborId, currentDepth + 1)
      edgeKeys.add(edgeKey(current, neighborId))
    }
  }

  return { nodes: visited, edges: edgeKeys, depth }
}

/**
 * Collects every section sharing the start node's persistent topic, by
 * identity key (persistent_topic_id) rather than embedding-graph reachability.
 *
 * This is the identity-key aggregation that survives cluster-label drift: two
 * sections of the same topic are grouped even when their similarity edge was
 * dropped by the 0.28 match penalty. Returns an empty set when the start has
 * no topic, so callers fall back to plain BFS.
 */
export function topicLifelineNodes(
  startNode: SectionTimelineNode,
  sections: SectionTimelineNode[],
): Set<number> {
  const topicId = startNode.persistent_topic_id
  if (topicId == null) return new Set<number>()
  const ids = new Set<number>([startNode.id])
  for (const s of sections) {
    if (s.persistent_topic_id === topicId) ids.add(s.id)
  }
  return ids
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
  mode: 'timeline' | 'lanes' = 'timeline',
): Map<number, LayoutResult> {
  const { colWidth, rowHeight, zJitter, rotationZDeg } = STYLE.layout
  const rotRad = (rotationZDeg * Math.PI) / 180

  // Unique sorted day keys.
  const days = Array.from(new Set(sections.map(s => s.period_date.slice(0, 10)))).sort()
  const dayIndex = new Map<string, number>(days.map((d, i) => [d, i]))

  if (mode === 'lanes') {
    // 话题泳道模式：Y = 话题索引（每个话题一条横向赛道），X = 天。
    // 同话题同天的多节点在赛道内 Y 方向小幅偏移，避免重叠。未归类话题
    // 归入最后一条“未分类”赛道。归档话题不参与（由上层过滤）。
    const laneKey = (s: SectionTimelineNode) =>
      s.persistent_topic ? `t${s.persistent_topic.id}` : 'unassigned'
    const laneOrder = Array.from(new Set(sections.map(laneKey)))
    const laneIndex = new Map<string, number>(laneOrder.map((k, i) => [k, i]))
    const laneSpacing = rowHeight * 2.6 // 赛道间距，大于同天堆叠跨度
    // 赛道围绕 Y=0 居中对称分布（顶部为正、底部为负），使整体中心保持在桌面以上。
    // 否则多话题时赛道从 Y=0 单向往下排，centerY 会被拉到桌面（desk.y=-1.6）以下，
    // 卡片连同灯靶/墙一起沉到桌子底下。
    const laneBaseY = ((laneOrder.length - 1) / 2) * laneSpacing

    // 每 (lane,day) 的节点计数，用于同赛道同天居中偏移
    const subCount = new Map<string, number>()
    for (const s of sections) {
      const k = `${laneKey(s)}:${s.period_date.slice(0, 10)}`
      subCount.set(k, (subCount.get(k) ?? 0) + 1)
    }
    const seen = new Map<string, number>()

    const result = new Map<number, LayoutResult>()
    // 按话题 + 天稳定顺序，保证同赛道同天节点的偏移顺序确定
    const sorted = [...sections].sort((a, b) => {
      const la = laneIndex.get(laneKey(a)) ?? 0
      const lb = laneIndex.get(laneKey(b)) ?? 0
      if (la !== lb) return la - lb
      return (dayIndex.get(a.period_date.slice(0, 10)) ?? 0) - (dayIndex.get(b.period_date.slice(0, 10)) ?? 0)
    })
    for (const s of sorted) {
      const li = laneIndex.get(laneKey(s)) ?? 0
      const x = (dayIndex.get(s.period_date.slice(0, 10)) ?? 0) * colWidth
      const laneY = laneBaseY - li * laneSpacing
      const k = `${laneKey(s)}:${s.period_date.slice(0, 10)}`
      const total = subCount.get(k) ?? 1
      const idx = seen.get(k) ?? 0
      seen.set(k, idx + 1)
      const subOffset = (idx - (total - 1) / 2) * rowHeight * 0.55
      const { z, rot } = seededJitter(s.id, zJitter, rotRad)
      result.set(s.id, {
        position: new Vector3(x, laneY + subOffset, z),
        rotationZ: rot,
      })
    }
    return result
  }

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
