import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionQualityExplore from './SectionQualityExplore.vue'
import type { DailyReportQualityEntry } from '~/api/dailyReports'

function entry(over: Partial<DailyReportQualityEntry> = {}): DailyReportQualityEntry {
  return {
    tag_id: 1,
    label: 'AI芯片',
    match_reason: 'direct_hit',
    score: 1.0,
    downgraded: false,
    ...over,
  }
}

describe('SectionQualityExplore', () => {
  it('renders one chip per breakdown entry with the reason colour and score', () => {
    const wrapper = mount(SectionQualityExplore, {
      props: {
        breakdown: [
          entry({ tag_id: 1, label: 'AI芯片', match_reason: 'direct_hit', score: 1.0 }),
          entry({ tag_id: 2, label: 'GPT-5发布', match_reason: 'max_sim', score: 0.85 }),
          entry({ tag_id: 3, label: 'AI竞赛', match_reason: 'weighted', score: 0.59 }),
        ],
      },
    })

    const chips = wrapper.findAll('.section-quality-explore__chip')
    expect(chips).toHaveLength(3)
    expect(chips[0]!.attributes('style')).toContain('var(--color-match-direct-hit)')
    expect(chips[1]!.attributes('style')).toContain('var(--color-match-max-sim)')
    expect(chips[2]!.attributes('style')).toContain('var(--color-match-weighted)')

    expect(wrapper.text()).toContain('AI芯片')
    expect(wrapper.text()).toContain('GPT-5发布')
    expect(wrapper.text()).toContain('1.00')
    expect(wrapper.text()).toContain('0.85')
  })

  it('marks a downgraded tag and appends ↓', () => {
    const wrapper = mount(SectionQualityExplore, {
      props: {
        breakdown: [entry({ tag_id: 9, label: 'X', match_reason: 'max_sim', score: 0.82, downgraded: true })],
      },
    })

    const chip = wrapper.find('.section-quality-explore__chip')
    expect(chip.classes()).toContain('section-quality-explore__chip--downgraded')
    // matchReasonColor(reason, true) emits a 50%-opacity color-mix — verified in
    // the util spec; here we assert the downgrade marker reaches the reader.
    expect(wrapper.text()).toContain('0.82↓')
  })

  it('shows the placeholder when breakdown is null (historical section)', () => {
    const wrapper = mount(SectionQualityExplore, { props: { breakdown: null } })

    expect(wrapper.text()).toContain('无质量明细')
    expect(wrapper.findAll('.section-quality-explore__chip')).toHaveLength(0)
  })

  it('shows the placeholder when breakdown is an empty array', () => {
    const wrapper = mount(SectionQualityExplore, { props: { breakdown: [] } })

    expect(wrapper.text()).toContain('无质量明细')
    expect(wrapper.findAll('.section-quality-explore__chip')).toHaveLength(0)
  })

  it('keeps the tag id stable as the chip key', () => {
    const wrapper = mount(SectionQualityExplore, {
      props: { breakdown: [entry({ tag_id: 42 })] },
    })

    expect(wrapper.find('.section-quality-explore__chip').attributes('data-tag-id')).toBe('42')
  })
})
