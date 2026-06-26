import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionAnchorBadge from './SectionAnchorBadge.vue'

describe('SectionAnchorBadge', () => {
  it('tier 0 → solid accent dot, no text', () => {
    const w = mount(SectionAnchorBadge, { props: { tier: 0 } })
    const dot = w.find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--solid')
    expect(dot.attributes('data-anchor-tier')).toBe('0')
    expect(dot.attributes('style')).toContain('var(--color-accent)')
    expect(dot.attributes('style')).not.toContain('transparent')
    expect(w.text()).toBe('')
  })

  it('tier 1 → solid steady dot (55% accent, verified in browser)', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 1 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--solid')
    expect(dot.classes()).not.toContain('section-anchor-badge--hollow')
    expect(dot.attributes('data-anchor-tier')).toBe('1')
    // happy-dom silently drops color-mix() inline values (CSSStyleDeclaration
    // discards them), so the 55% opacity cannot be asserted in unit tests —
    // it is verified visually / in the real browser. The tier mapping itself
    // is covered by data-anchor-tier + the topicAnchorTier pure-function spec
    // (Task 1, which exhaustively tests the 0.05 / 0.15 boundaries).
  })

  it('tier 2 → solid loose dot (30% accent, verified in browser)', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 2 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--solid')
    expect(dot.classes()).not.toContain('section-anchor-badge--hollow')
    expect(dot.attributes('data-anchor-tier')).toBe('2')
    // Same happy-dom color-mix limitation as tier 1 — see note there.
  })

  it('tier 3 (auto_new) → hollow accent ring', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 3 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--hollow')
    expect(dot.classes()).not.toContain('section-anchor-badge--solid')
    expect(dot.attributes('data-anchor-tier')).toBe('3')
    expect(dot.attributes('style')).toContain('transparent')
    expect(dot.attributes('style')).toContain('var(--color-accent)')
  })

  it('tier 4 (unanchored) → hollow gray ring', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 4 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--hollow')
    expect(dot.attributes('data-anchor-tier')).toBe('4')
    expect(dot.attributes('style')).toContain('transparent')
    expect(dot.attributes('style')).toContain('var(--color-match-weighted)')
  })

  it('never leaks any text / numbers for any tier', () => {
    for (const tier of [0, 1, 2, 3, 4]) {
      expect(mount(SectionAnchorBadge, { props: { tier } }).text()).toBe('')
    }
  })

  it('exposes an aria-label carrying the chinese tightness word', () => {
    const w0 = mount(SectionAnchorBadge, { props: { tier: 0 } })
    expect(w0.find('.section-anchor-badge').attributes('aria-label')).toContain('极紧锚定')
    const w4 = mount(SectionAnchorBadge, { props: { tier: 4 } })
    expect(w4.find('.section-anchor-badge').attributes('aria-label')).toContain('未锚定')
  })
})
