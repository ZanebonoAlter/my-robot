/**
 * Topic focus-view helpers — pure, testable logic extracted from BoardThreadBrowser.
 *
 * The focus view reuses the lanes data model (board section timeline) and only
 * re-projects a single topic's nodes into a dedicated layout. These functions
 * keep that projection free of Vue reactivity so they can be unit-tested in
 * isolation (see topicFocus.test.ts).
 */
import type { SectionTimelineNode } from '~/api/dailyReports'

/** Scenario: 横向时间轴 — keep only the sections of one persistent topic. */
export function filterFocusNodes(
  sections: SectionTimelineNode[],
  topicId: number,
): SectionTimelineNode[] {
  return sections.filter(s => s.persistent_topic_id === topicId)
}

/** Scenario: 拖拽平移时间轴 — true when the pointer travelled beyond the
 *  threshold and the upcoming click should therefore be swallowed. Absolute
 *  distance is used so direction is irrelevant. */
export function isDragMove(deltaX: number, threshold: number): boolean {
  return Math.abs(deltaX) > threshold
}

/** Scenario: sticky 标题始终置顶 / 空话题降级 — aggregate the dynamics count,
 *  the earliest/latest date and the day-span for the sticky header. An empty
 *  node list yields an empty meta (no throw) so the view can degrade. */
export interface FocusMeta {
  /** dynamics count (number of section nodes for this topic). */
  count: number
  firstDate: string | null
  lastDate: string | null
  /** inclusive day span between firstDate and lastDate. */
  spanDays: number
  /** true when the topic has no nodes in the current window (fallback signal). */
  empty: boolean
}

export function buildFocusMeta(nodes: SectionTimelineNode[]): FocusMeta {
  if (nodes.length === 0) {
    return { count: 0, firstDate: null, lastDate: null, spanDays: 0, empty: true }
  }
  // Read-only: never mutate caller order — derive min/max by scanning.
  let first = nodes[0]!.period_date.slice(0, 10)
  let last = first
  for (const n of nodes) {
    const d = n.period_date.slice(0, 10)
    if (d < first) first = d
    if (d > last) last = d
  }
  return {
    count: nodes.length,
    firstDate: first,
    lastDate: last,
    spanDays: dayDiff(first, last),
    empty: false,
  }
}

/** Day difference between two 'YYYY-MM-DD' strings (UTC to avoid DST skew). */
function dayDiff(a: string, b: string): number {
  const ta = Date.parse(`${a}T00:00:00Z`)
  const tb = Date.parse(`${b}T00:00:00Z`)
  if (Number.isNaN(ta) || Number.isNaN(tb)) return 0
  return Math.round((tb - ta) / 86_400_000)
}
