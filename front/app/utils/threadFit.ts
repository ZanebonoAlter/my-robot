/**
 * Thread-fit observability helpers for the daily-report thread surface
 * (System 3: thread ↔ section). Encodes how well a thread title fits its owning
 * section title, expressed as a cosine distance, as a boolean "demoted"
 * judgement plus a chinese label.
 *
 * `fit_distance` is a cosine distance in [0.0, 2.0]: smaller = tighter fit,
 * 0.0 = perfect fit, nil = no signal (historical thread). A thread whose
 * distance strictly exceeds {@link THREAD_FIT_DEMOTE_THRESHOLD} is considered
 * off-topic (跑题) and is flagged for de-emphasis by the UI.
 *
 * Colours are NOT resolved here — the badge component derives theme tokens, as
 * the sibling `topicAnchor.ts` / `matchQuality.ts` helpers do.
 *
 * Pure functions, no side effects, no DOM access.
 */

/** Demote threshold, calibrated against the live 2026-06-26 fit_distance
 *  distribution (86 signalled threads): min 0.000, p50 0.156, p90 0.274,
 *  p99 0.306. At 0.28 exactly 7/86 (8%) threads are demoted — the natural
 *  upper tail where titles genuinely drift from their section narrative
 *  (e.g. personnel changes filed under a market-movement section). The
 *  earlier candidate 0.20 would have demoted 35%, mostly false positives;
 *  see change `thread-fit-observability` design D3 for the calibration note.
 *  Boundary tests reference this constant so recalibration touches one place. */
export const THREAD_FIT_DEMOTE_THRESHOLD = 0.28

/**
 * Whether a thread's title fit is "demoted" (likely off-topic) relative to its
 * owning section. Returns true only when `fitDistance` is a number strictly
 * greater than {@link THREAD_FIT_DEMOTE_THRESHOLD}. Returns false for
 * undefined / null / NaN / negative / ≤ threshold — historical threads with no
 * signal and anomalous inputs are never demoted.
 */
export function isThreadFitDemoted(fitDistance?: number | null): boolean {
  if (fitDistance == null) return false
  // NaN > x and negative > threshold both evaluate to false, so those cases
  // naturally fall through to "not demoted" — no extra guard needed.
  return fitDistance > THREAD_FIT_DEMOTE_THRESHOLD
}

/** Chinese fit label for a thread. Shares the demote boundary with
 *  {@link isThreadFitDemoted} (strictly > threshold = 可能跑题); splits the
 *  remaining cases into "贴合" (a valid distance ≤ threshold) and "无贴合信号"
 *  (undefined / null / NaN). */
export function threadFitLabel(fitDistance?: number | null): string {
  if (fitDistance == null || Number.isNaN(fitDistance)) return '无贴合信号'
  if (fitDistance > THREAD_FIT_DEMOTE_THRESHOLD) return '可能跑题'
  return '贴合'
}
