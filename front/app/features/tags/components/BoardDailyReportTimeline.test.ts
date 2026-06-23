import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BoardDailyReportTimeline from './BoardDailyReportTimeline.vue'
import type { DailyReport, DailyReportListItem } from '~/api/dailyReports'

const api = vi.hoisted(() => ({
  getBoardDailyReports: vi.fn(),
  getDailyReportDetail: vi.fn(),
  getTopicLifeline: vi.fn(),
  getArticle: vi.fn(),
}))

vi.mock('~/api/dailyReports', () => ({
  useDailyReportsApi: () => ({
    getBoardDailyReports: api.getBoardDailyReports,
    getDailyReportDetail: api.getDailyReportDetail,
    getTopicLifeline: api.getTopicLifeline,
  }),
}))

vi.mock('~/api/articles', () => ({
  useArticlesApi: () => ({ getArticle: api.getArticle }),
}))

vi.mock('@floating-ui/vue', () => ({
  useFloating: () => ({ floatingStyles: { value: {} } }),
}))

vi.mock('@floating-ui/dom', () => ({
  autoUpdate: vi.fn(),
  offset: vi.fn(),
  shift: vi.fn(),
  flip: vi.fn(),
}))

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" />' },
}))

vi.mock('./BoardThreadBrowser.vue', () => ({
  default: { name: 'BoardThreadBrowser', template: '<div data-testid="thread-browser" />' },
}))

vi.mock('./TopicDetectiveWall.client.vue', () => ({
  default: { name: 'TopicDetectiveWall', template: '<div data-testid="detective-wall" />' },
}))

const reports: DailyReportListItem[] = [
  {
    id: 60,
    semantic_board_id: 1974,
    period_date: '2026-06-21T12:00:00Z',
    title: '6 月 21 日日报',
    summary: '今日摘要',
    status: 'done',
    cluster_count: 1,
    article_count: 4,
    event_tag_count: 2,
    created_at: '2026-06-21T12:00:00Z',
  },
  {
    id: 52,
    semantic_board_id: 1974,
    period_date: '2026-06-20T12:00:00Z',
    title: '6 月 20 日日报',
    summary: '昨日摘要',
    status: 'done',
    cluster_count: 1,
    article_count: 2,
    event_tag_count: 1,
    created_at: '2026-06-20T12:00:00Z',
  },
]

function makeDetail(id: number): DailyReport {
  return {
    ...reports.find(report => report.id === id)!,
    highlights: [{ title: '头条', reason: '值得关注', tag_ids: [] }],
    dynamics: '',
    sections: [{
      id: 366,
      cluster_index: 0,
      cluster_label: '霍尔木兹海峡航运恢复',
      cluster_tag_ids: [],
      article_count: 2,
      best_tier: 0,
      avg_score: 0.9,
      persistent_topic_id: 5,
      persistent_topic: {
        id: 5,
        label: '霍尔木兹海峡航运恢复',
        status: 'active',
        color: '#b44f45',
        consecutive_hits: 3,
        can_activate: false,
      },
      threads: [{
        id: 10,
        report_id: id,
        section_id: 366,
        title: '通行量逐步恢复',
        summary: '航运风险开始回落。',
        tag_ids: [],
        confidence: 0.9,
        related_article_ids: [99],
        created_at: '2026-06-21T12:00:00Z',
      }],
    }],
  }
}

async function mountTimeline() {
  const wrapper = mount(BoardDailyReportTimeline, {
    attachTo: document.body,
    props: { boardId: 1974 },
  })
  await flushPromises()
  return wrapper
}

describe('BoardDailyReportTimeline preserved behavior', () => {
  beforeEach(() => {
    api.getBoardDailyReports.mockResolvedValue({ success: true, data: { reports } })
    api.getDailyReportDetail.mockImplementation(async (id: number) => ({ success: true, data: { report: makeDetail(id) } }))
    api.getArticle.mockResolvedValue({ success: true, data: { id: 99, title: '航运恢复观察' } })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('opens a report, navigates dates, and closes with Escape', async () => {
    const wrapper = await mountTimeline()
    const reportCard = wrapper.find('.drt-summary-card')
    ;(reportCard.element as HTMLElement).focus()
    await reportCard.trigger('click')
    await flushPromises()

    expect(document.body.querySelector('.drm-overlay')).not.toBeNull()
    expect(document.body.textContent).toContain('6 月 21 日')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }))
    await flushPromises()
    expect(document.body.textContent).toContain('6 月 20 日')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(document.body.querySelector('.drm-overlay')).toBeNull()
    expect(document.activeElement).toBe(reportCard.element)
  })

  it('keeps topic overview entry point', async () => {
    const wrapper = await mountTimeline()

    await wrapper.find('.drt-browser-toggle').trigger('click')
    expect(wrapper.find('[data-testid="thread-browser"]').exists()).toBe(true)
    await wrapper.find('.drt-browser-toggle').trigger('click')

    await wrapper.find('.drt-summary-card').trigger('click')
    await flushPromises()
    const topicHeader = document.body.querySelector('.drm-topic__header') as HTMLElement
    if (topicHeader.getAttribute('aria-expanded') !== 'true') topicHeader.click()
    await nextTick()
    expect(document.body.querySelector('.drm-thread__header')).not.toBeNull()
  })

  it('emits openArticle from a thread article', async () => {
    const wrapper = await mountTimeline()
    await wrapper.find('.drt-summary-card').trigger('click')
    await flushPromises()

    const topicHeader = document.body.querySelector('.drm-topic__header') as HTMLElement
    if (topicHeader.getAttribute('aria-expanded') !== 'true') topicHeader.click()
    await nextTick()
    ;(document.body.querySelector('.drm-thread__header') as HTMLElement).click()
    await flushPromises()
    ;(document.body.querySelector('.drm-article') as HTMLElement).click()
    await nextTick()

    expect(wrapper.emitted('openArticle')).toEqual([[99]])
  })
})
