import { computed, ref, watch, type Ref } from 'vue'
import { useDailyReportsApi } from '~/api/dailyReports'
import { useArticlesApi } from '~/api/articles'
import { createRequestCache, type RequestCacheEntry } from '~/features/tags/components/daily-report/dailyReportMagazine'
import type {
  DailyReport,
  DailyReportListItem,
  SectionRelation,
  SectionTimelineNode,
} from '~/api/dailyReports'

export interface TopicLifelineData {
  sections: SectionTimelineNode[]
  relations: SectionRelation[]
}

export interface ArticleTitle {
  id: number
  title: string
}

function responseError(message: string): Error {
  return new Error(message)
}

export function useDailyReportReader(boardId: Ref<number>) {
  const { getBoardDailyReports, getDailyReportDetail, getTopicLifeline } = useDailyReportsApi()
  const { getArticle } = useArticlesApi()

  const reports = ref<DailyReportListItem[]>([])
  const days = ref(7)
  const loading = ref(false)
  const currentDayIndex = ref(-1)
  const detailCache = ref(new Map<number, DailyReport>())
  const detailLoading = ref<number | null>(null)
  const detailError = ref('')
  const lifelineEntries = ref(new Map<number, RequestCacheEntry<TopicLifelineData>>())
  const articleEntries = ref(new Map<number, RequestCacheEntry<ArticleTitle>>())

  const lifelineCache = createRequestCache<number, TopicLifelineData>(async (topicId) => {
    const response = await getTopicLifeline(topicId)
    if (!response.success || !response.data) throw responseError('话题生命线加载失败')
    return response.data
  }, entries => { lifelineEntries.value = entries })

  const articleCache = createRequestCache<number, ArticleTitle>(async (articleId) => {
    const response = await getArticle(articleId)
    if (!response.success || !response.data) throw responseError(`文章 #${articleId} 加载失败`)
    return { id: articleId, title: response.data.title || '(无标题)' }
  }, entries => { articleEntries.value = entries })

  const selectedReport = computed(() => {
    if (currentDayIndex.value < 0 || currentDayIndex.value >= reports.value.length) return null
    return reports.value[currentDayIndex.value]
  })

  const selectedDetail = computed(() => {
    const report = selectedReport.value
    return report ? detailCache.value.get(report.id) ?? null : null
  })

  async function loadReports() {
    loading.value = true
    try {
      const response = await getBoardDailyReports(boardId.value, { days: days.value })
      reports.value = response.success && response.data ? response.data.reports || [] : []
    } finally {
      loading.value = false
    }
  }

  async function ensureDetail(reportId: number) {
    if (detailCache.value.has(reportId)) return detailCache.value.get(reportId)
    detailLoading.value = reportId
    detailError.value = ''
    try {
      const response = await getDailyReportDetail(reportId)
      if (!response.success || !response.data) {
        detailError.value = '日报详情加载失败'
        return undefined
      }
      detailCache.value.set(reportId, response.data.report)
      detailCache.value = new Map(detailCache.value)
      return response.data.report
    } finally {
      detailLoading.value = null
    }
  }

  async function selectReport(index: number) {
    const report = reports.value[index]
    if (!report) return undefined
    currentDayIndex.value = index
    return ensureDetail(report.id)
  }

  function getReportDetail(reportId: number): DailyReport | undefined {
    return detailCache.value.get(reportId)
  }

  async function selectReportById(reportId: number) {
    const index = reports.value.findIndex(report => report.id === reportId)
    if (index < 0) return undefined
    return selectReport(index)
  }

  async function shiftReport(offset: number) {
    const next = currentDayIndex.value + offset
    if (next < 0 || next >= reports.value.length) return undefined
    return selectReport(next)
  }

  async function loadMore() {
    days.value += 7
    await loadReports()
  }

  function getLifeline(topicId: number): RequestCacheEntry<TopicLifelineData> {
    return lifelineEntries.value.get(topicId) ?? { status: 'idle' }
  }

  async function ensureLifeline(topicId: number, retry = false) {
    return lifelineCache.load(topicId, retry)
  }

  function getArticleTitle(articleId: number): RequestCacheEntry<ArticleTitle> {
    return articleEntries.value.get(articleId) ?? { status: 'idle' }
  }

  async function ensureArticleTitles(articleIds: number[], retry = false) {
    const uniqueIds = [...new Set(articleIds)]
    await Promise.all(uniqueIds.map(articleId => articleCache.load(articleId, retry)))
  }

  async function retryArticle(articleId: number) {
    return articleCache.load(articleId, true)
  }

  function clearBoardState() {
    days.value = 7
    reports.value = []
    currentDayIndex.value = -1
    detailCache.value = new Map()
    detailLoading.value = null
    detailError.value = ''
    lifelineCache.clear()
    articleCache.clear()
  }

  watch(boardId, async () => {
    clearBoardState()
    await loadReports()
  }, { immediate: true })

  return {
    reports,
    days,
    loading,
    currentDayIndex,
    detailLoading,
    detailError,
    lifelineEntries,
    articleEntries,
    selectedReport,
    selectedDetail,
    detailCache,
    loadReports,
    ensureDetail,
    getReportDetail,
    selectReport,
    selectReportById,
    shiftReport,
    loadMore,
    getLifeline,
    ensureLifeline,
    getArticleTitle,
    ensureArticleTitles,
    retryArticle,
  }
}
