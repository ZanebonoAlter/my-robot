import { ref, computed } from 'vue'
import { useSemanticBoardsApi, type BoardArticle, type BoardArticleTag } from '~/api/semanticBoards'
import { useFeedsStore } from '~/stores/feeds'

export function useBoardTimeline() {
  const sbApi = useSemanticBoardsApi()
  const feedsStore = useFeedsStore()

  const timelineArticles = ref<BoardArticle[]>([])
  const timelineLoading = ref(false)
  const timelinePage = ref(1)
  const timelineHasMore = ref(false)
  const timelinePerPage = 50

  const activeFilterLabelId = ref<number | null>(null)
  const filterFeedId = ref<number | null>(null)
  const startDate = ref<string>(getDateStr(new Date()))
  const endDate = ref<string>(getDateStr(new Date()))
  const showDirectionMismatch = ref(false)
  const timelineSort = ref<'quality' | 'time'>('quality')
  const quickRange = ref<'today' | '3d' | '7d' | '30d' | null>('today')

  const selectedTagForDetail = ref<BoardArticleTag | null>(null)

  const feedOptions = computed(() => feedsStore.feeds)

  const timelineDisplayArticles = computed(() => timelineArticles.value.map((article) => {
    if (showDirectionMismatch.value) return article
    return {
      ...article,
      filtered_tags: (article.filtered_tags || []).filter(tag => !tag.direction_mismatch),
    }
  }))

  function getDateStr(d: Date): string {
    return d.toISOString().slice(0, 10)
  }

  async function loadTimelineArticles(boardId: number, append = false) {
    timelineLoading.value = true
    try {
      const page = append ? timelinePage.value + 1 : 1
      const params: Record<string, unknown> = { page, per_page: timelinePerPage }
      if (activeFilterLabelId.value !== null) params.auxiliary_label_id = activeFilterLabelId.value
      if (filterFeedId.value) params.feed_id = filterFeedId.value
      if (startDate.value) params.start_date = startDate.value
      if (endDate.value) params.end_date = endDate.value
      if (showDirectionMismatch.value) params.show_direction_mismatch = true
      if (timelineSort.value === 'time') params.sort = 'time'
      const res = await sbApi.getBoardArticles(boardId, params)
      if (res.success && res.data) {
        const newArticles = res.data
        if (append) {
          timelineArticles.value.push(...newArticles)
          timelinePage.value = page
        } else {
          timelineArticles.value = newArticles
          timelinePage.value = 1
        }
        const total = res.pagination?.total ?? 0
        timelineHasMore.value = timelineArticles.value.length < total
      } else {
        if (!append) timelineArticles.value = []
      }
    } catch {
      if (!append) timelineArticles.value = []
    } finally {
      timelineLoading.value = false
    }
  }

  function handleLoadMore(boardId: number | null) {
    if (boardId !== null && !timelineLoading.value) {
      void loadTimelineArticles(boardId, true)
    }
  }

  function handleFilterLabel(labelId: number | null, boardId: number | null) {
    activeFilterLabelId.value = labelId
    selectedTagForDetail.value = null
    timelinePage.value = 1
    if (boardId !== null) void loadTimelineArticles(boardId)
  }

  function handleFilterChange(boardId: number | null) {
    selectedTagForDetail.value = null
    timelinePage.value = 1
    if (boardId !== null) void loadTimelineArticles(boardId)
  }

  function handleSortChange(mode: 'quality' | 'time', boardId: number | null) {
    if (timelineSort.value === mode) return
    timelineSort.value = mode
    handleFilterChange(boardId)
  }

  function handleDateInputChange(boardId: number | null) {
    quickRange.value = null
    handleFilterChange(boardId)
  }

  function applyQuickRange(range: 'today' | '3d' | '7d' | '30d', boardId: number | null) {
    quickRange.value = range
    const now = new Date()
    endDate.value = getDateStr(now)
    const start = new Date()
    if (range === '3d') start.setDate(start.getDate() - 2)
    else if (range === '7d') start.setDate(start.getDate() - 6)
    else if (range === '30d') start.setDate(start.getDate() - 29)
    startDate.value = getDateStr(start)
    handleFilterChange(boardId)
  }

  function toggleMatchDetail(tag: BoardArticleTag) {
    selectedTagForDetail.value = selectedTagForDetail.value?.id === tag.id ? null : tag
  }

  function isSelectedDetailTag(tag: BoardArticleTag): boolean {
    return selectedTagForDetail.value?.id === tag.id
  }

  function matchReasonColor(reason: string, downgraded?: boolean): string {
    const colors: Record<string, string> = {
      direct_hit: '#22c55e',
      hit_rate: '#3b82f6',
      max_sim: '#f59e0b',
      weighted: '#94a3b8',
    }
    const color = colors[reason] || '#94a3b8'
    return downgraded ? color + '80' : color
  }

  function matchInfoLabel(tag: BoardArticleTag): string {
    const labels: Record<string, string> = {
      direct_hit: '直接命中',
      hit_rate: '命中率',
      max_sim: '相似度',
      weighted: '综合',
    }
    return `${labels[tag.match_reason] || tag.match_reason} ${tag.score.toFixed(2)}${tag.downgraded ? '↓' : ''}`
  }

  function strongestMatch(tags: BoardArticleTag[]): BoardArticleTag | null {
    if (!tags?.length) return null
    const [first, ...rest] = tags
    if (!first) return null
    return rest.reduce((best, t) => t.score > best.score ? t : best, first)
  }

  function resetTimeline() {
    timelineArticles.value = []
    timelinePage.value = 1
    timelineHasMore.value = false
    activeFilterLabelId.value = null
    filterFeedId.value = null
    selectedTagForDetail.value = null
    quickRange.value = 'today'
    const now = new Date()
    startDate.value = getDateStr(now)
    endDate.value = getDateStr(now)
  }

  return {
    timelineArticles, timelineLoading, timelinePage, timelineHasMore, timelinePerPage,
    activeFilterLabelId, filterFeedId, startDate, endDate,
    showDirectionMismatch, timelineSort, quickRange,
    feedOptions, timelineDisplayArticles,
    selectedTagForDetail,
    loadTimelineArticles, handleLoadMore,
    handleFilterLabel, handleFilterChange, handleSortChange,
    handleDateInputChange, applyQuickRange,
    toggleMatchDetail, isSelectedDetailTag, matchReasonColor, matchInfoLabel, strongestMatch,
    resetTimeline,
  }
}
