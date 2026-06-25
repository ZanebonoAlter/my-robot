import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionTierBadge from './SectionTierBadge.vue'

describe('SectionTierBadge', () => {
  it('renders a filled direct-hit (green) dot for bestTier=0 with no text', () => {
    const wrapper = mount(SectionTierBadge, { props: { bestTier: 0 } })
    const dot = wrapper.find('.section-tier-badge')

    expect(dot.classes()).toContain('section-tier-badge--solid')
    expect(dot.classes()).not.toContain('section-tier-badge--hollow')
    expect(dot.attributes('data-tier')).toBe('0')
    expect(dot.attributes('style')).toContain('var(--color-match-direct-hit)')
    // spec: 正文徽章仅色彩无数字 — no score / percent / match-method text anywhere.
    expect(wrapper.text()).toBe('')
  })

  it('renders a filled hit-rate (blue) dot for bestTier=1', () => {
    const dot = mount(SectionTierBadge, { props: { bestTier: 1 } }).find('.section-tier-badge')

    expect(dot.classes()).toContain('section-tier-badge--solid')
    expect(dot.attributes('data-tier')).toBe('1')
    expect(dot.attributes('style')).toContain('var(--color-match-hit-rate)')
  })

  it('renders a filled max-sim (orange) dot for bestTier=2', () => {
    const dot = mount(SectionTierBadge, { props: { bestTier: 2 } }).find('.section-tier-badge')

    expect(dot.classes()).toContain('section-tier-badge--solid')
    expect(dot.attributes('data-tier')).toBe('2')
    expect(dot.attributes('style')).toContain('var(--color-match-max-sim)')
  })

  it('renders a hollow weighted (gray) dot for bestTier=3', () => {
    const wrapper = mount(SectionTierBadge, { props: { bestTier: 3 } })
    const dot = wrapper.find('.section-tier-badge')

    expect(dot.classes()).toContain('section-tier-badge--hollow')
    expect(dot.classes()).not.toContain('section-tier-badge--solid')
    expect(dot.attributes('data-tier')).toBe('3')
    expect(dot.attributes('style')).toContain('var(--color-match-weighted)')
    // hollow = transparent fill, colour lives on the ring border.
    expect(dot.attributes('style')).toContain('transparent')
    expect(wrapper.text()).toBe('')
  })

  it('falls back to the hollow weighted dot for an unknown tier', () => {
    const dot = mount(SectionTierBadge, { props: { bestTier: 99 } }).find('.section-tier-badge')

    expect(dot.classes()).toContain('section-tier-badge--hollow')
    expect(dot.attributes('style')).toContain('var(--color-match-weighted)')
  })

  it('never leaks score / percentage / match-method wording for any tier', () => {
    for (const bestTier of [0, 1, 2, 3]) {
      const wrapper = mount(SectionTierBadge, { props: { bestTier } })
      expect(wrapper.text()).toBe('')
    }
  })
})
