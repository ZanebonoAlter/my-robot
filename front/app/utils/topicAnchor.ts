/**
 * Topic-anchor tightness helpers for the daily-report section surface (System 2:
 * section ↔ persistent topic). Encodes the match tightness as a discrete tier
 * (0..4) plus a chinese label.
 *
 * `confidence` is the primary signal; `distance` only sub-divides `anchor_hit`
 * via the dual thresholds (0.05 / 0.15, aligned to the 2026-06-06 measurement).
 * Colours are NOT resolved here — the badge component derives theme tokens.
 *
 * Pure functions, no side effects, no DOM access.
 */

export const TOPIC_ANCHOR_TIGHT_THRESHOLD = 0.05
export const TOPIC_ANCHOR_LOOSE_THRESHOLD = 0.15

/** Anchor tightness tier. Higher = looser / not anchored. */
export const ANCHOR_TIERS = {
  TIGHT: 0, // anchor_hit, distance ≤ 0.05  → 极紧锚定
  STEADY: 1, // anchor_hit, distance ≤ 0.15  → 稳锚定
  LOOSE: 2, // anchor_hit, distance ≤ 0.30  → 松锚定
  NEW: 3, // auto_new                      → 新话题候选
  UNANCHORED: 4, // unmatched / missing       → 未锚定
} as const

export type AnchorTier = (typeof ANCHOR_TIERS)[keyof typeof ANCHOR_TIERS]

/**
 * Resolve a section's topic-anchor tightness tier (0..4). Confidence is the
 * primary signal: `auto_new` is always tier 3; anything that is not
 * `anchor_hit`/`auto_new` (incl. missing) is tier 4. `distance` only
 * sub-divides `anchor_hit`; a missing/zero distance under `anchor_hit` degrades
 * defensively to tier 4 (spec: missing distance → unanchored).
 */
export function topicAnchorTier(
  distance: number | undefined | null,
  confidence: string | undefined | null,
): AnchorTier {
  if (confidence === 'auto_new') return ANCHOR_TIERS.NEW
  if (confidence !== 'anchor_hit') return ANCHOR_TIERS.UNANCHORED
  if (distance == null || !Number.isFinite(distance) || distance <= 0) {
    return ANCHOR_TIERS.UNANCHORED
  }
  if (distance <= TOPIC_ANCHOR_TIGHT_THRESHOLD) return ANCHOR_TIERS.TIGHT
  if (distance <= TOPIC_ANCHOR_LOOSE_THRESHOLD) return ANCHOR_TIERS.STEADY
  return ANCHOR_TIERS.LOOSE
}

const ANCHOR_LABELS: Record<AnchorTier, string> = {
  0: '极紧锚定',
  1: '稳锚定',
  2: '松锚定',
  3: '新话题候选',
  4: '未锚定',
}

/** Chinese tightness label matching {@link topicAnchorTier}. */
export function topicAnchorLabel(
  distance: number | undefined | null,
  confidence: string | undefined | null,
): string {
  return ANCHOR_LABELS[topicAnchorTier(distance, confidence)]
}
