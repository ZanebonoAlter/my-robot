import { ref } from 'vue'
import { useNotify } from '~/composables/useNotify'
import {
  useBoardEnrichmentApi,
  type ContextRow,
  type ContextGranularity,
  type ResultSummaryRow,
  type ReviewRow,
  type DataSourceRow,
  type CreateReviewBody,
  type UpsertDataSourceBody,
} from '~/api/boardEnrichment'
import { useDailyReportsApi, type BoardTopicListItem } from '~/api/dailyReports'

/**
 * 数据增强面板的 composable（仿 useBoardCRUD 模式）。
 *
 * 维度拆分：
 *  - topic 维度（表 1/2/3）：contexts / results / reviews，挂在 selectedTopicId 上；
 *  - board 维度（数据源绑定）：dataSources，挂在 boardId 上。
 *
 * 面板组件直接 `const en = useBoardEnrichment()`，board/topic 切换时显式调
 * loadTopics / selectTopic / loadDataSources（不自动 watch，保持与 useBoardCRUD 一致
 * 的显式调用风格，便于测试与可控加载）。
 */
export function useBoardEnrichment() {
  const api = useBoardEnrichmentApi()
  const reportsApi = useDailyReportsApi()
  const { success: notifySuccess, error: notifyError } = useNotify()

  // ── topic selector ──────────────────────────────────────────────────────
  const topics = ref<BoardTopicListItem[]>([])
  const topicsLoading = ref(false)
  const selectedTopicId = ref<number | null>(null)

  // ── table 1: contexts ───────────────────────────────────────────────────
  const contexts = ref<ContextRow[]>([])
  const contextsLoading = ref(false)
  const regenerating = ref<ContextGranularity | null>(null)

  // ── table 2: results ────────────────────────────────────────────────────
  const results = ref<ResultSummaryRow[]>([])
  const resultsLoading = ref(false)
  const triggering = ref(false)

  // ── table 3: reviews ────────────────────────────────────────────────────
  const reviews = ref<ReviewRow[]>([])
  const reviewsLoading = ref(false)

  // ── board data sources ──────────────────────────────────────────────────
  const dataSources = ref<DataSourceRow[]>([])
  const dataSourcesLoading = ref(false)

  const error = ref<string | null>(null)

  // ── topic selector actions ──────────────────────────────────────────────
  async function loadTopics(boardId: number) {
    topicsLoading.value = true
    error.value = null
    const res = await reportsApi.listBoardTopics(boardId)
    if (res.success && res.data) {
      topics.value = res.data.topics ?? []
      const stillValid = selectedTopicId.value !== null && topics.value.some(t => t.id === selectedTopicId.value)
      if (!stillValid) {
        const firstActive = topics.value.find(t => t.status === 'active')
        selectedTopicId.value = topics.value.length ? (firstActive?.id ?? topics.value[0]?.id ?? null) : null
      }
    } else {
      topics.value = []
      error.value = res.error || '加载话题失败'
    }
    topicsLoading.value = false
  }

  async function selectTopic(topicId: number) {
    selectedTopicId.value = topicId
    await loadAllTopicTables(topicId)
  }

  // ── table loaders ───────────────────────────────────────────────────────
  async function loadContexts(topicId: number) {
    contextsLoading.value = true
    const res = await api.listContexts(topicId)
    if (res.success && res.data) {
      contexts.value = res.data
    } else {
      contexts.value = []
    }
    contextsLoading.value = false
  }

  async function loadResults(topicId: number) {
    resultsLoading.value = true
    const res = await api.listResults(topicId)
    if (res.success && res.data) {
      results.value = res.data
    } else {
      results.value = []
    }
    resultsLoading.value = false
  }

  async function loadReviews(topicId: number) {
    reviewsLoading.value = true
    const res = await api.listReviews(topicId)
    if (res.success && res.data) {
      reviews.value = res.data
    } else {
      reviews.value = []
    }
    reviewsLoading.value = false
  }

  async function loadAllTopicTables(topicId: number) {
    await Promise.all([loadContexts(topicId), loadResults(topicId), loadReviews(topicId)])
  }

  // ── table 1 actions ─────────────────────────────────────────────────────
  async function saveContext(topicId: number, granularity: ContextGranularity, content: string): Promise<boolean> {
    const res = await api.updateContext(topicId, granularity, { content })
    if (res.success && res.data) {
      const idx = contexts.value.findIndex(c => c.granularity === granularity)
      if (idx >= 0) contexts.value[idx] = res.data
      else contexts.value.push(res.data)
      notifySuccess('已保存上下文')
      return true
    }
    notifyError(res.error || '保存失败')
    return false
  }

  async function regenerateContext(topicId: number, granularity: ContextGranularity): Promise<boolean> {
    regenerating.value = granularity
    try {
      const res = await api.regenerateContext(topicId, granularity)
      if (res.success && res.data) {
        const idx = contexts.value.findIndex(c => c.granularity === granularity)
        if (idx >= 0) contexts.value[idx] = res.data
        else contexts.value.push(res.data)
        notifySuccess(`${granularity} 上下文已重生成`)
        return true
      }
      notifyError(res.error || '重生成失败')
      return false
    } finally {
      regenerating.value = null
    }
  }

  // ── table 2 actions ─────────────────────────────────────────────────────
  async function triggerEnrichment(topicId: number): Promise<boolean> {
    triggering.value = true
    try {
      const res = await api.triggerEnrichment(topicId)
      if (res.success) {
        notifySuccess(res.data?.review_generated ? '增强完成，已生成新 review' : '增强完成')
        await loadResults(topicId)
        await loadReviews(topicId)
        return true
      }
      // 400 = 板块未开启增强
      notifyError(res.error || '触发失败：需先在板块编辑开启增强开关')
      return false
    } finally {
      triggering.value = false
    }
  }

  // ── table 3 actions ─────────────────────────────────────────────────────
  async function saveReviewDeviation(topicId: number, reviewId: number, deviation: string): Promise<boolean> {
    const res = await api.updateReviewDeviation(topicId, reviewId, { deviation_summary: deviation })
    if (res.success && res.data) {
      const idx = reviews.value.findIndex(r => r.id === reviewId)
      if (idx >= 0) reviews.value[idx] = res.data
      notifySuccess('已保存偏差说明')
      return true
    }
    notifyError(res.error || '保存失败')
    return false
  }

  async function applyReview(topicId: number, reviewId: number): Promise<boolean> {
    const res = await api.applyReview(topicId, reviewId)
    if (res.success && res.data) {
      const idx = reviews.value.findIndex(r => r.id === reviewId)
      if (idx >= 0) reviews.value[idx] = res.data
      notifySuccess('已采纳')
      return true
    }
    notifyError(res.error || '采纳失败')
    return false
  }

  async function createReview(topicId: number, body: CreateReviewBody): Promise<boolean> {
    const res = await api.createReview(topicId, body)
    if (res.success && res.data) {
      reviews.value.unshift(res.data)
      notifySuccess('已添加批注')
      return true
    }
    notifyError(res.error || '添加批注失败')
    return false
  }

  // ── board data source actions ───────────────────────────────────────────
  async function loadDataSources(boardId: number) {
    dataSourcesLoading.value = true
    const res = await api.listDataSources(boardId)
    if (res.success && res.data) {
      dataSources.value = res.data
    } else {
      dataSources.value = []
    }
    dataSourcesLoading.value = false
  }

  async function saveDataSource(boardId: number, body: UpsertDataSourceBody): Promise<boolean> {
    const res = await api.upsertDataSource(boardId, body)
    if (res.success && res.data) {
      await loadDataSources(boardId)
      notifySuccess('已保存数据源')
      return true
    }
    notifyError(res.error || '保存失败')
    return false
  }

  async function removeDataSource(boardId: number, sourceType: string): Promise<boolean> {
    const res = await api.deleteDataSource(boardId, sourceType)
    if (res.success) {
      dataSources.value = dataSources.value.filter(d => d.source_type !== sourceType)
      notifySuccess('已删除数据源')
      return true
    }
    notifyError(res.error || '删除失败')
    return false
  }

  return {
    // topic selector
    topics, topicsLoading, selectedTopicId,
    loadTopics, selectTopic,
    // table 1
    contexts, contextsLoading, regenerating,
    loadContexts, saveContext, regenerateContext,
    // table 2
    results, resultsLoading, triggering,
    loadResults, triggerEnrichment,
    // table 3
    reviews, reviewsLoading,
    loadReviews, saveReviewDeviation, applyReview, createReview,
    // data sources
    dataSources, dataSourcesLoading,
    loadDataSources, saveDataSource, removeDataSource,
    // misc
    error, loadAllTopicTables,
  }
}
