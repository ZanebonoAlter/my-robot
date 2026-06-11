import { computed, ref, watch } from 'vue'
import { useTopicGraphApi, type TopicCategory, type TopicGraphDetailPayload, type TopicGraphFilters, type TopicGraphType } from '~/api/topicGraph'
import { useNotify } from '~/composables/useNotify'
import { buildDisplayedTopicGraph } from '~/features/topic-graph/utils/buildDisplayedTopicGraph'
import { buildTopicGraphViewModel } from '~/features/topic-graph/utils/buildTopicGraphViewModel'
import { normalizeTopicCategory } from '~/features/topic-graph/utils/normalizeTopicCategory'
import { useFloatingPanelDrag } from './useFloatingPanelDrag'
import { useArticlePreview } from './useArticlePreview'
import { useTopicTimeline } from './useTopicTimeline'
import { useHotspotTopics } from './useHotspotTopics'

export function useTopicGraph(initialLoad = true) {
  const { error: notify } = useNotify()
  const topicGraphApi = useTopicGraphApi()

  // ---- Drag ----
  const drag = useFloatingPanelDrag()

  // ---- Article preview ----
  const preview = useArticlePreview()

  // ---- 工具函数 ----
  function formatDateInput(date = new Date()) {
    const year = date.getFullYear()
    const month = `${date.getMonth() + 1}`.padStart(2, '0')
    const day = `${date.getDate()}`.padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  // ---- 核心状态 ----
  const selectedType = ref<TopicGraphType>('daily')
  const selectedDate = ref(formatDateInput())
  const selectedFilterCategoryId = ref<string | null>(null)
  const selectedFilterFeedId = ref<string | null>(null)
  const graphPayload = ref<Awaited<ReturnType<typeof topicGraphApi.getGraph>>['data'] | null>(null)
  const selectedTopicSlug = ref<string | null>(null)
  const selectedCategory = ref<TopicCategory | null>(null)
  const selectedKeywordSlug = ref<string | null>(null)
  const graphFocusRequestKey = ref(0)
  const detail = ref<TopicGraphDetailPayload | null>(null)
  const loadingGraph = ref(false)
  const loadingDetail = ref(false)
  const notice = ref<string | null>(null)

  // ---- Computed graph ----
  const viewModel = computed(() => graphPayload.value
    ? buildTopicGraphViewModel(graphPayload.value)
    : buildTopicGraphViewModel({
      type: selectedType.value,
      anchor_date: selectedDate.value,
      period_label: '正在载入',
      topic_count: 0,
      article_count: 0,
      feed_count: 0,
      top_topics: [],
      nodes: [],
      edges: [],
    }))

  // ---- Hotspot topics (depends on viewModel) ----
  const hotspot = useHotspotTopics(viewModel)

  // ---- Timeline (depends on detail + hotspot) ----
  const timeline = useTopicTimeline(detail, hotspot.hotspotDigests, hotspot.selectedHotspotTag)

  // ---- Computed (graph display) ----
  const displayedGraph = computed(() => buildDisplayedTopicGraph({
    graph: viewModel.value.graph,
    visibleTopicSlugs: hotspot.graphVisibleTopicSlugs.value,
  }))

  const activeTopicNode = computed(() => {
    const focusSlug = selectedKeywordSlug.value || selectedTopicSlug.value
    return displayedGraph.value.nodes.find(node => node.slug === focusSlug) || null
  })

  const highlightedNodeIds = computed(() => {
    const highlighted = new Set<string>()
    const focusSlug = selectedKeywordSlug.value || selectedTopicSlug.value
    if (!focusSlug) return []

    const focusNode = displayedGraph.value.nodes.find(node => node.slug === focusSlug)
    if (!focusNode) return []

    highlighted.add(focusNode.id)
    for (const edge of displayedGraph.value.edges) {
      const sourceId = resolveGraphLinkNodeId(edge.source)
      const targetId = resolveGraphLinkNodeId(edge.target)
      if (sourceId === focusNode.id) highlighted.add(targetId)
      if (targetId === focusNode.id) highlighted.add(sourceId)
    }
    return Array.from(highlighted)
  })

  const relatedEdgeIds = computed(() => {
    const highlightedSet = new Set(highlightedNodeIds.value)
    if (!highlightedSet.size) return []
    return displayedGraph.value.edges
      .filter(edge =>
        highlightedSet.has(resolveGraphLinkNodeId(edge.source))
        && highlightedSet.has(resolveGraphLinkNodeId(edge.target))
      )
      .map(edge => edge.id)
  })

  const pageState = computed(() => {
    if (loadingGraph.value) return 'loading'
    if (preview.selectedPreviewArticle.value) return 'article-preview'
    if (detail.value) return 'detail'
    if (graphPayload.value) return 'graph-ready'
    return 'empty'
  })

  const selectedTopicInfo = computed(() => {
    if (detail.value?.topic) {
      return {
        id: detail.value.topic.id,
        slug: detail.value.topic.slug,
        label: detail.value.topic.label,
        category: normalizeTopicCategory(detail.value.topic.category, detail.value.topic.kind),
        description: detail.value.topic.description,
      }
    }
    if (!selectedTopicSlug.value) return null
    const topic = viewModel.value.topTopics.find(item => item.slug === selectedTopicSlug.value)
    if (!topic) return null
    return {
      id: topic.id ?? 0,
      slug: topic.slug,
      label: topic.label,
      category: normalizeTopicCategory(topic.category, topic.kind),
      description: topic.description,
    }
  })

  // ---- Utility ----
  function resolveGraphLinkNodeId(node: string | { id: string }) {
    return typeof node === 'string' ? node : node.id
  }

  function buildCurrentFilters(): TopicGraphFilters | undefined {
    if (selectedFilterFeedId.value) return { feedId: selectedFilterFeedId.value }
    if (selectedFilterCategoryId.value && selectedFilterCategoryId.value !== '__uncategorized__') {
      return { categoryId: selectedFilterCategoryId.value }
    }
    return undefined
  }

  // ---- 数据加载 ----
  async function loadGraph() {
    loadingGraph.value = true
    notice.value = null

    try {
      const response = await topicGraphApi.getGraph(selectedType.value, selectedDate.value, buildCurrentFilters())
      if (!response.success || !response.data) {
        notify(response.error || '图谱加载失败')
        notice.value = response.error || '主题图谱没拉下来'
        graphPayload.value = null
        detail.value = null
        return
      }

      graphPayload.value = response.data
      selectedTopicSlug.value = response.data.top_topics[0]?.slug || null
      selectedCategory.value = response.data.top_topics[0]
        ? normalizeTopicCategory(response.data.top_topics[0].category, response.data.top_topics[0].kind)
        : null
      selectedKeywordSlug.value = null
      timeline.selectedDigestId.value = null
      timeline.previewDigestId.value = null
      hotspot.expandedTopicSlugs.value = []
      hotspot.graphVisibilityOverrides.value = {}

      if (selectedTopicSlug.value) {
        const firstTopic = response.data.top_topics[0]
        hotspot.selectedHotspotTag.value = {
          slug: selectedTopicSlug.value,
          label: firstTopic?.label || selectedTopicSlug.value,
          category: selectedCategory.value || 'keyword',
        }
        void loadTopicDetail(selectedTopicSlug.value)
        void hotspot.loadHotspotDigests(selectedTopicSlug.value, selectedCategory.value || undefined)
        void hotspot.loadPendingArticles(selectedTopicSlug.value)
      } else {
        detail.value = null
        hotspot.selectedHotspotTag.value = null
        hotspot.hotspotDigests.value = []
      }

      void loadHotspots()
    } catch (error) {
      console.error('Failed to load topic graph:', error)
      notify(error instanceof Error ? error.message : '图谱加载失败')
      notice.value = error instanceof Error ? error.message : '主题图谱加载失败'
    } finally {
      loadingGraph.value = false
    }
  }

  async function loadHotspots() {
    hotspot.loadingHotspots.value = true
    try {
      const response = await topicGraphApi.getTopicsByCategory(selectedType.value, selectedDate.value, buildCurrentFilters())
      if (response.success && response.data) {
        hotspot.hotspotData.value = response.data
      } else {
        console.error('Failed to load hotspots:', response.error)
        hotspot.hotspotData.value = null
      }
    } catch (error) {
      console.error('Failed to load hotspots:', error)
      hotspot.hotspotData.value = null
    } finally {
      hotspot.loadingHotspots.value = false
    }
  }

  async function loadTopicDetail(slug: string) {
    selectedTopicSlug.value = slug
    selectedKeywordSlug.value = null
    timeline.selectedDigestId.value = null
    timeline.previewDigestId.value = null

    const topic = viewModel.value.topTopics.find(item => item.slug === slug) || null
    if (topic?.category) {
      selectedCategory.value = normalizeTopicCategory(topic.category, topic.kind)
    }

    loadingDetail.value = true
    try {
      const response = await topicGraphApi.getTopicDetail(slug, undefined, undefined, buildCurrentFilters())
      if (response.success && response.data) {
        detail.value = response.data
        selectedCategory.value = normalizeTopicCategory(response.data.topic.category, response.data.topic.kind)
        timeline.selectedDigestId.value = null
        timeline.previewDigestId.value = null
        return
      }
      detail.value = null
      notify(response.error || '话题详情加载失败')
      notice.value = response.error || '话题详情加载失败'
    } catch (error) {
      console.error('Failed to load topic detail:', error)
      detail.value = null
      notify(error instanceof Error ? error.message : '话题详情加载失败')
      notice.value = error instanceof Error ? error.message : '话题详情加载失败'
    } finally {
      loadingDetail.value = false
    }
  }

  // ---- 事件处理 ----
  async function handleTagSelect(slug: string, category: TopicCategory) {
    hotspot.ensureTopicShownInGraph(slug)
    hotspot.expandRelatedTopics(slug)
    graphFocusRequestKey.value += 1
    selectedCategory.value = category
    selectedTopicSlug.value = slug

    let tagLabel = slug
    const allTags = [
      ...(hotspot.hotspotData.value?.events || []),
      ...(hotspot.hotspotData.value?.people || []),
      ...(hotspot.hotspotData.value?.keywords || []),
    ]
    const foundTag = allTags.find((t: any) => t.slug === slug)
    if (foundTag) tagLabel = foundTag.label

    const isAbstract = foundTag?.is_abstract ?? false
    const childSlugs = foundTag?.child_slugs ?? []
    hotspot.abstractNodeSlug.value = isAbstract ? slug : null
    hotspot.abstractNodeLabel.value = isAbstract ? tagLabel : null

    hotspot.selectedHotspotTag.value = { slug, label: tagLabel, category }
    hotspot.selectedPendingNode.value = false

    if (isAbstract && childSlugs.length > 0) {
      await hotspot.loadAbstractTagDigests(childSlugs)
    } else {
      await hotspot.loadHotspotDigests(slug, category)
    }

    void hotspot.loadPendingArticles(slug)
    void timeline.loadAggregatedArticles(slug, hotspot.hotspotData.value)
    void loadTopicDetail(slug)
    timeline.timelineOpen.value = true
  }

  async function handleChildTagSelect(childSlug: string, childLabel: string) {
    hotspot.selectedHotspotTag.value = { slug: childSlug, label: childLabel, category: 'keyword' }
    await hotspot.loadHotspotDigests(childSlug, 'keyword')
    if (!timeline.timelineOpen.value) timeline.timelineOpen.value = true
    void hotspot.loadPendingArticles(childSlug)
    void timeline.loadAggregatedArticles(childSlug, hotspot.hotspotData.value)
    void loadTopicDetail(childSlug)
    hotspot.ensureTopicShownInGraph(childSlug)
    hotspot.expandRelatedTopics(childSlug)
    graphFocusRequestKey.value += 1
    hotspot.selectedPendingNode.value = false
  }

  async function handleAbstractTagSelect(abstractSlug: string) {
    const allTags = [
      ...(hotspot.hotspotData.value?.events || []),
      ...(hotspot.hotspotData.value?.people || []),
      ...(hotspot.hotspotData.value?.keywords || []),
    ]
    const abstractTag = allTags.find((t: any) => t.slug === abstractSlug)
    const abstractLabel = abstractTag?.label || abstractSlug
    const childSlugs = abstractTag?.child_slugs || []

    hotspot.selectedHotspotTag.value = {
      slug: abstractSlug,
      label: abstractLabel,
      category: abstractTag ? normalizeTopicCategory(abstractTag.category, abstractTag.kind) : 'keyword',
    }

    if (childSlugs.length > 0) {
      await hotspot.loadAbstractTagDigests(childSlugs)
    } else {
      await hotspot.loadHotspotDigests(abstractSlug, hotspot.selectedHotspotTag.value.category)
    }

    if (!timeline.timelineOpen.value) timeline.timelineOpen.value = true
    void hotspot.loadPendingArticles(abstractSlug)
    void timeline.loadAggregatedArticles(abstractSlug, hotspot.hotspotData.value)
    void loadTopicDetail(abstractSlug)
    hotspot.ensureTopicShownInGraph(abstractSlug)
    hotspot.expandRelatedTopics(abstractSlug)
    graphFocusRequestKey.value += 1
    hotspot.selectedPendingNode.value = false
  }

  function handleSelectPending() {
    hotspot.selectedPendingNode.value = true
    timeline.selectedDigestId.value = null
    timeline.previewDigestId.value = null
  }

  function handleNodeClick(node: { slug?: string; kind: string; category?: TopicCategory; label?: string; isAbstract?: boolean }) {
    if (node.kind !== 'topic' || !node.slug) return
    hotspot.ensureTopicShownInGraph(node.slug)
    hotspot.expandRelatedTopics(node.slug)
    graphFocusRequestKey.value += 1
    if (node.category) selectedCategory.value = node.category
    hotspot.abstractNodeSlug.value = node.isAbstract ? node.slug : null
    hotspot.abstractNodeLabel.value = node.isAbstract ? (node.label || node.slug) : null
    hotspot.selectedHotspotTag.value = { slug: node.slug, label: node.label || node.slug, category: node.category || 'keyword' }
    hotspot.selectedPendingNode.value = false
    void hotspot.loadHotspotDigests(node.slug, node.category)
    void hotspot.loadPendingArticles(node.slug)
    void timeline.loadAggregatedArticles(node.slug, hotspot.hotspotData.value)
    void loadTopicDetail(node.slug)
    timeline.timelineOpen.value = true
  }

  function handleKeywordHighlight(keywordSlug: string | null) {
    if (!keywordSlug) { selectedKeywordSlug.value = null; return }
    const existsInGraph = displayedGraph.value.nodes.some(node => node.kind === 'topic' && node.slug === keywordSlug)
    selectedKeywordSlug.value = existsInGraph ? keywordSlug : null
  }

  async function openArticlePreview(articleId: number) {
    timeline.previewDigestId.value = null
    const relatedIds = detail.value?.summaries
      ?.flatMap(summary => summary.articles.map(article => article.id))
    await preview.openArticlePreview(articleId, relatedIds)
    if (!preview.selectedPreviewArticle.value) {
      notify('文章预览加载失败')
      notice.value = '文章预览加载失败'
    }
  }

  // ---- Watchers ----
  watch(selectedType, () => {
    drag.resetPosition()
    void loadGraph()
  })

  watch(selectedDate, () => {
    drag.resetPosition()
    void loadGraph()
  })

  watch([selectedFilterCategoryId, selectedFilterFeedId], () => {
    void loadGraph()
  })

  // ---- 首次加载 ----
  if (initialLoad) {
    queueMicrotask(() => { void loadGraph() })
  }

  return {
    // Core state
    selectedType, selectedDate, selectedFilterCategoryId, selectedFilterFeedId,
    selectedCategory, selectedKeywordSlug, graphFocusRequestKey,
    selectedDigestId: timeline.selectedDigestId,
    previewDigestId: timeline.previewDigestId,
    detail, loadingGraph, loadingDetail, loadingPreviewArticle: preview.loadingPreviewArticle,
    notice, selectedPreviewArticle: preview.selectedPreviewArticle,
    previewArticles: preview.previewArticles,
    selectedTopicSlug, pageState, graphPayload,

    // Hotspot
    hotspotData: hotspot.hotspotData,
    loadingHotspots: hotspot.loadingHotspots,
    hotspotSearchQueries: hotspot.hotspotSearchQueries,
    hotspotDropdownOpen: hotspot.hotspotDropdownOpen,
    hotspotShowAll: hotspot.hotspotShowAll,
    hotspotSearchRefs: hotspot.hotspotSearchRefs,
    hotspotCategories: hotspot.hotspotCategories,
    loadingHotspotDigests: hotspot.loadingHotspotDigests,
    selectedHotspotTag: hotspot.selectedHotspotTag,

    // Pending articles
    pendingArticles: hotspot.pendingArticles,
    selectedPendingNode: hotspot.selectedPendingNode,
    loadingPendingArticles: hotspot.loadingPendingArticles,

    // Timeline
    aggregationMode: timeline.aggregationMode,
    loadingAggregatedArticles: timeline.loadingAggregatedArticles,
    selectedGroupKey: timeline.selectedGroupKey,
    selectedGroupArticles: timeline.selectedGroupArticles,
    timelineAggregationGroups: timeline.timelineAggregationGroups,
    totalAggregatedCount: timeline.totalAggregatedCount,
    timelineOpen: timeline.timelineOpen,
    selectedDigest: timeline.selectedDigest,
    previewDigest: timeline.previewDigest,

    // Abstract tag
    abstractNodeSlug: hotspot.abstractNodeSlug,
    abstractNodeLabel: hotspot.abstractNodeLabel,

    // Computed
    viewModel, activeTopicNode, highlightedNodeIds, relatedEdgeIds,
    displayedGraph, selectedTopicInfo,
    timelineItems: timeline.timelineItems,
    effectiveTimelineItems: timeline.effectiveTimelineItems,

    // Timeline drag
    timelinePanelRef: drag.panelRef,
    isDragging: drag.isDragging,
    timelinePosition: drag.position,

    // Graph helpers
    isTopicShownInGraph: hotspot.isTopicShownInGraph,
    toggleTopicGraphVisibility: hotspot.toggleTopicGraphVisibility,
    expandRelatedTopics: hotspot.expandRelatedTopics,

    // Handlers
    handleNodeClick, handleKeywordHighlight,
    handleDigestSelect: timeline.handleDigestSelect,
    handlePreviewDigest: timeline.handlePreviewDigest,
    closeDigestPreview: timeline.closeDigestPreview,
    openArticlePreview, closeArticlePreview: preview.closeArticlePreview,
    handleArticleFavorite: preview.handleArticleFavorite,
    handleArticleUpdate: preview.handleArticleUpdate,
    handleTagSelect, handleChildTagSelect, handleAbstractTagSelect,
    handleTimelineGroupSelect: timeline.handleTimelineGroupSelect,
    handleAggregationModeChange: timeline.handleAggregationModeChange,
    handleSelectPending,
    toggleShowAll: hotspot.toggleShowAll,
    closeHotspotDropdown: hotspot.closeHotspotDropdown,
    startTimelineDrag: drag.startDrag,
    resetTimelinePanelPosition: drag.resetPosition,

    // Actions
    loadGraph, loadTopicDetail,
  }
}
