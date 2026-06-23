/**
 * Unit tests for the detective-wall ambiance layer pure functions.
 *
 * @see openspec/changes/detective-wall-ambiance/tasks.md §9.2
 */
import { describe, it, expect } from 'vitest'
import { directionalFogFactor } from './shaders/directionalFog'
import { lampPositionFor } from './SetDressing'
import { STYLE } from './types'

describe('directionalFogFactor', () => {
  const { density, range } = STYLE.directionalFog
  const originX = 9 // "today" column

  it('is 0 at the origin (today has no extra fog)', () => {
    expect(directionalFogFactor(originX, originX, density, range)).toBe(0)
  })

  it('is 0 for the future (x > origin)', () => {
    expect(directionalFogFactor(originX + 2, originX, density, range)).toBe(0)
    expect(directionalFogFactor(1000, originX, density, range)).toBe(0)
  })

  it('grows into the past matching 1 - exp(-density * past)', () => {
    // 10 world units into the past, density 1.2, range 12 → past = 10/12
    const past = 10 / range
    const expected = 1 - Math.exp(-density * past)
    expect(directionalFogFactor(originX - 10, originX, density, range)).toBeCloseTo(expected, 5)
  })

  it('clamps at the range for the far past', () => {
    // dx far exceeds range → past clamps to 1 → factor = 1 - exp(-density)
    const expected = 1 - Math.exp(-density * 1)
    expect(directionalFogFactor(originX - 1000, originX, density, range)).toBeCloseTo(expected, 5)
  })
})

describe('lampPositionFor', () => {
  const { offset } = STYLE.lamp
  const deskY = STYLE.desk.y

  it('places the lamp at latestDayX + offset.x, on the desk, at offset.z', () => {
    const p = lampPositionFor(9, offset, deskY)
    expect(p.x).toBeCloseTo(9 + offset.x, 5)
    expect(p.y).toBe(deskY)
    expect(p.z).toBe(offset.z)
  })

  it('moves with the today column across reflows', () => {
    const a = lampPositionFor(0, offset, deskY)
    const b = lampPositionFor(30, offset, deskY)
    expect(b.x - a.x).toBeCloseTo(30, 5)
    expect(a.y).toBe(b.y)
    expect(a.z).toBe(b.z)
  })
})
