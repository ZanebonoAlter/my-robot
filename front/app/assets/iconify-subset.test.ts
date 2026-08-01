import { describe, expect, it } from 'vitest'
import { readIconSubset, scanAppIconNames } from '../../scripts/iconify-scan.mjs'

// The generated subset (app/assets/iconify-subset.json) must cover every
// iconify icon referenced in app/ source, otherwise those icons fall back to
// runtime network loading (api.iconify.design) which is exactly what the
// localization change removes.
//
// Uses the same extraction rule as the generator (scripts/iconify-scan.mjs),
// so generation and verification can never drift.
describe('iconify subset consistency', () => {
  it('subset covers all mdi icons referenced in app source', () => {
    const { mdi } = scanAppIconNames()
    const subset = readIconSubset()
    const subsetNames = new Set(
      Object.keys(subset.icons ?? {}).map((name) => `mdi:${name}`),
    )

    const missing = mdi.filter((name) => !subsetNames.has(name))
    const hint =
      missing.length > 0
        ? `subset is missing ${missing.length} icon(s): ${missing.join(', ')}\nrun 'pnpm generate:icons' and commit the regenerated app/assets/iconify-subset.json`
        : ''

    expect(missing.length, hint).toBe(0)
  })
})
