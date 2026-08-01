import { describe, expect, it } from 'vitest'
import { matchReasonColor, matchInfoLabel } from './matchQuality'

describe('matchReasonColor', () => {
  it('maps each known reason to its theme token', () => {
    expect(matchReasonColor('direct_hit')).toBe('var(--color-match-direct-hit)')
    expect(matchReasonColor('hit_rate')).toBe('var(--color-match-hit-rate)')
    expect(matchReasonColor('max_sim')).toBe('var(--color-match-max-sim)')
    expect(matchReasonColor('weighted')).toBe('var(--color-match-weighted)')
  })

  it('falls back to the weighted token for an unknown reason', () => {
    expect(matchReasonColor('unknown')).toBe('var(--color-match-weighted)')
    expect(matchReasonColor('')).toBe('var(--color-match-weighted)')
  })

  it('returns the bare token when not downgraded', () => {
    expect(matchReasonColor('max_sim', false)).toBe('var(--color-match-max-sim)')
    expect(matchReasonColor('max_sim')).toBe('var(--color-match-max-sim)')
  })

  it('halves the token opacity via color-mix when downgraded', () => {
    // tokens cannot take a hex alpha suffix, so a downgraded match mixes the
    // token to 50% — preserving the prior `color + '80'` visual.
    expect(matchReasonColor('max_sim', true)).toBe(
      'color-mix(in srgb, var(--color-match-max-sim) 50%, transparent)',
    )
    expect(matchReasonColor('direct_hit', true)).toBe(
      'color-mix(in srgb, var(--color-match-direct-hit) 50%, transparent)',
    )
  })
})

describe('matchInfoLabel', () => {
  it('builds a chinese reason word + score', () => {
    expect(matchInfoLabel({ match_reason: 'direct_hit', score: 1 })).toBe('直接命中 1.00')
    expect(matchInfoLabel({ match_reason: 'hit_rate', score: 0.42 })).toBe('命中率 0.42')
    expect(matchInfoLabel({ match_reason: 'max_sim', score: 0.85 })).toBe('相似度 0.85')
    expect(matchInfoLabel({ match_reason: 'weighted', score: 0.59 })).toBe('综合 0.59')
  })

  it('appends the downgrade marker only when downgraded', () => {
    expect(matchInfoLabel({ match_reason: 'max_sim', score: 0.82, downgraded: true })).toBe('相似度 0.82↓')
    expect(matchInfoLabel({ match_reason: 'max_sim', score: 0.82, downgraded: false })).toBe('相似度 0.82')
    expect(matchInfoLabel({ match_reason: 'max_sim', score: 0.82 })).toBe('相似度 0.82')
  })

  it('falls back to the raw reason for unknown values', () => {
    expect(matchInfoLabel({ match_reason: 'mystery', score: 0.1 })).toBe('mystery 0.10')
  })
})
