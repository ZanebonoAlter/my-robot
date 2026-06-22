import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { useDailyReportReader } from './useDailyReportReader'

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

describe('useDailyReportReader', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getBoardDailyReports.mockResolvedValue({ success: true, data: { reports: [] } })
    api.getTopicLifeline.mockResolvedValue({ success: true, data: { sections: [], relations: [] } })
    api.getArticle.mockImplementation(async (id: number) => ({ success: true, data: { id: String(id), title: `article-${id}` } }))
  })

  it('deduplicates lifeline and article requests', async () => {
    const reader = useDailyReportReader(ref(1974))
    await flushPromises()

    await Promise.all([reader.ensureLifeline(5), reader.ensureLifeline(5)])
    await reader.ensureLifeline(5)
    await reader.ensureArticleTitles([99, 99])
    await reader.ensureArticleTitles([99])

    expect(api.getTopicLifeline).toHaveBeenCalledTimes(1)
    expect(api.getArticle).toHaveBeenCalledTimes(1)
    expect(reader.getArticleTitle(99).data?.title).toBe('article-99')
  })

  it('keeps a failed item local and supports retry', async () => {
    let article99Attempts = 0
    api.getArticle.mockImplementation(async (id: number) => {
      if (id === 99 && article99Attempts++ === 0) return { success: false }
      return { success: true, data: { id: String(id), title: id === 99 ? 'recovered' : `article-${id}` } }
    })
    const reader = useDailyReportReader(ref(1974))
    await flushPromises()

    await reader.ensureArticleTitles([99, 100])
    expect(reader.getArticleTitle(99).status).toBe('error')
    expect(reader.getArticleTitle(100).status).toBe('success')

    await reader.retryArticle(99)
    expect(reader.getArticleTitle(99).data?.title).toBe('recovered')
  })

  it('clears board-scoped caches when the board changes', async () => {
    const boardId = ref(1974)
    const reader = useDailyReportReader(boardId)
    await flushPromises()
    await reader.ensureLifeline(5)
    await reader.ensureArticleTitles([99])

    boardId.value = 1980
    await flushPromises()

    expect(reader.days.value).toBe(7)
    expect(reader.getLifeline(5).status).toBe('idle')
    expect(reader.getArticleTitle(99).status).toBe('idle')
    expect(api.getBoardDailyReports).toHaveBeenLastCalledWith(1980, { days: 7 })
  })
})
