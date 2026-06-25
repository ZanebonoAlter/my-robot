/**
 * Shared match-quality helpers for the match-score surface (TagsPage) and the
 * daily-report quality explore panel.
 *
 * Colours are derived from theme tokens so they follow the editorial/dark
 * themes. A downgraded match halves the token's opacity via `color-mix` — this
 * preserves the existing 50%-opacity visual (previously `color + '80'` hex
 * alpha) without hard-coded hex, since alpha can no longer be appended to a
 * `var()` token.
 */

/** Structural shape for a matched tag. Accepts both {@link BoardArticleTag}
 *  (tags surface) and a `quality_breakdown` entry (daily-report explore panel). */
export interface MatchInfoEntry {
  match_reason: string
  score: number
  downgraded?: boolean
}

const MATCH_REASON_TOKENS: Record<string, string> = {
  direct_hit: 'var(--color-match-direct-hit)',
  hit_rate: 'var(--color-match-hit-rate)',
  max_sim: 'var(--color-match-max-sim)',
  weighted: 'var(--color-match-weighted)',
}

/** Theme-token colour for a match reason. Pass `downgraded` for a 50%-opacity
 *  variant (used for downgraded chip borders / labels). */
export function matchReasonColor(reason: string, downgraded?: boolean): string {
  const token = MATCH_REASON_TOKENS[reason] ?? 'var(--color-match-weighted)'
  return downgraded
    ? `color-mix(in srgb, ${token} 50%, transparent)`
    : token
}

/** Human-readable label + score for a matched tag, e.g. "相似度 0.85↓". */
export function matchInfoLabel(tag: MatchInfoEntry): string {
  const labels: Record<string, string> = {
    direct_hit: '直接命中',
    hit_rate: '命中率',
    max_sim: '相似度',
    weighted: '综合',
  }
  return `${labels[tag.match_reason] || tag.match_reason} ${tag.score.toFixed(2)}${tag.downgraded ? '↓' : ''}`
}
