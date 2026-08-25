import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DailyReportWatchIndex from './DailyReportWatchIndex.vue'
import type { TopicWatchHit } from '~/api/topicWatches'

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" />' },
}))

function hit(over: Partial<TopicWatchHit> = {}): TopicWatchHit {
  return {
    id: '1',
    watchId: '10',
    sectionId: '100',
    reportId: '9',
    periodDate: '2026-06-29',
    reason: '不应显示的理由',
    watchLabel: 'ASML',
    watchType: 'keyword',
    ...over,
  }
}

describe('DailyReportWatchIndex', () => {
  it('按关键字/话题分区显示可定位的单行索引，不复制 reason 或正文', async () => {
    const wrapper = mount(DailyReportWatchIndex, {
      props: {
        hits: [
          hit(),
          hit({ id: '2', watchId: '11', sectionId: '101', watchLabel: '中东局势', watchType: 'label' }),
        ],
        sections: [
          { id: 100, cluster_label: '出口管制动态' },
          { id: 101, cluster_label: '海峡航运变化' },
        ],
      },
    })

    expect(wrapper.find('[data-testid="watch-index-keywords"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="watch-index-topics"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="watch-index-item"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('ASML')
    expect(wrapper.text()).toContain('出口管制动态')
    expect(wrapper.text()).not.toContain('不应显示的理由')

    await wrapper.find('[data-testid="watch-index-item"]').trigger('click')
    expect(wrapper.emitted('locate')).toEqual([[100]])
  })

  it('没有命中时整体隐藏，并允许只有一类命中', () => {
    const empty = mount(DailyReportWatchIndex, { props: { hits: [], sections: [] } })
    expect(empty.find('[data-testid="watch-index"]').exists()).toBe(false)

    const keywordOnly = mount(DailyReportWatchIndex, {
      props: { hits: [hit()], sections: [{ id: 100, cluster_label: '出口管制动态' }] },
    })
    expect(keywordOnly.find('[data-testid="watch-index-keywords"]').exists()).toBe(true)
    expect(keywordOnly.find('[data-testid="watch-index-topics"]').exists()).toBe(false)
  })
})
