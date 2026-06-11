import { ref, computed, watch } from 'vue'
import { useTopicGraphApi, type TopicCategory, type TopicGraphDetailPayload } from '~/api/topicGraph'
import { useAbstractTagApi } from '~/api/abstractTags'
import type { TagHierarchyNode } from '~/types/topicTag'
import type {
  PendingArticle, TimelineDigestSelection, TimelineAggregationArticle,
} from '~/types/timeline'
import { normalizeTopicCategory } from '~/features/topic-graph/utils/normalizeTopicCategory'

interface Keyword {
  slug: string
  label: string
  count: number
  relevance: number
}

interface SidebarProps {
  detail: TopicGraphDetailPayload | null
  selectedDigest?: TimelineDigestSelection | null
  loading?: boolean
  error?: string | null
  dataState?: string
  selectedKeyword?: string | null
  selectedTagSlug?: string | null
  pendingArticles?: PendingArticle[]
  selectedPendingNode?: boolean
  abstractNodeSlug?: string | null
  abstractNodeLabel?: string | null
  timelineGroupArticles?: TimelineAggregationArticle[]
  timelineGroupKey?: string | null
}

export function useTopicGraphSidebar(props: SidebarProps, emit: {
  openArticle: (articleId: number) => void
  highlightKeyword: (keywordSlug: string | null) => void
  tagMerged: () => void
  selectChildTag: (slug: string, label: string) => void
  selectAbstractTag: (slug: string) => void
}) {
  const internalSelectedKeyword = ref<string | null>(null)
  const activeKeywordSlug = computed(() => props.selectedKeyword !== undefined ? props.selectedKeyword : internalSelectedKeyword.value)

  const topicGraphApi = useTopicGraphApi()
  const abstractTagApi = useAbstractTagApi()

  const showMergeDialog = ref(false)
  const mergeSearchQuery = ref('')
  const mergeSearchResults = ref<Array<{ id: number; label: string; slug: string; category: string; feed_count: number }>>([])
  const mergeSearching = ref(false)
  const mergeMerging = ref(false)
  const mergeError = ref<string | null>(null)
  const mergeSuccess = ref<string | null>(null)
  let mergeSearchTimer: ReturnType<typeof setTimeout> | null = null

  const abstractChildren = ref<TagHierarchyNode[]>([])
  const abstractLoading = ref(false)

  const topicCategoryLabels: Record<TopicCategory, string> = {
    event: '事件', person: '人物', keyword: '关键词',
  }

  const deduplicatedArticles = computed(() => {
    if (!props.detail || !props.selectedDigest) return []
    const topicArticleIds = new Set(props.detail.articles.map(a => a.id))
    const matchedIds = new Set(props.selectedDigest.matchedArticleIds)
    const hasSelectedTag = props.selectedTagSlug && props.selectedTagSlug.trim() !== ''
    return props.selectedDigest.articles
      .filter(a => !hasSelectedTag || matchedIds.has(a.id) || topicArticleIds.has(a.id))
      .map(a => ({
        ...a,
        matchedTopic: matchedIds.has(a.id) || topicArticleIds.has(a.id),
        matchedBySummaryOnly: !topicArticleIds.has(a.id),
      }))
      .sort((l, r) => l.matchedTopic === r.matchedTopic ? 0 : l.matchedTopic ? -1 : 1)
  })

  const keywords = computed((): Keyword[] => {
    if (props.selectedDigest?.matchedArticlesTags?.length) {
      return props.selectedDigest.matchedArticlesTags.slice(0, 18).map(tag => ({
        slug: tag.slug, label: tag.label, count: 1, relevance: 0.5,
      }))
    }
    if (!props.detail?.related_tags?.length) return []
    const maxC = Math.max(...props.detail.related_tags.map(t => t.cooccurrence), 1)
    return props.detail.related_tags.slice(0, 18).map(tag => ({
      slug: tag.slug, label: tag.label, count: tag.cooccurrence,
      relevance: Math.max(tag.cooccurrence / maxC, 0.28),
    }))
  })

  const shouldScrollFeaturedArticles = computed(() => deduplicatedArticles.value.length > 8)

  const displayTopicCategory = computed<TopicCategory>(() => {
    if (!props.detail) return 'keyword'
    return normalizeTopicCategory(props.detail.topic.category, props.detail.topic.kind)
  })

  function handleKeywordSelect(keyword: Keyword) {
    if (activeKeywordSlug.value === keyword.slug) {
      internalSelectedKeyword.value = null
      emit.highlightKeyword(null)
    } else {
      internalSelectedKeyword.value = keyword.slug
      emit.highlightKeyword(keyword.slug)
    }
  }

  watch(() => props.selectedKeyword, (value) => {
    if (value === null) internalSelectedKeyword.value = null
  })

  watch(() => props.abstractNodeSlug, async (slug) => {
    if (!slug) { abstractChildren.value = []; return }
    abstractLoading.value = true
    try {
      const res = await abstractTagApi.fetchHierarchy(undefined, undefined, undefined, undefined, undefined)
      if (res.success && res.data) {
        const findNodeBySlug = (nodes: TagHierarchyNode[], targetSlug: string): TagHierarchyNode | null => {
          for (const node of nodes) {
            if (node.slug === targetSlug) return node
            const found = findNodeBySlug(node.children, targetSlug)
            if (found) return found
          }
          return null
        }
        const match = findNodeBySlug(res.data.nodes, slug)
        abstractChildren.value = match?.children || []
      }
    } catch { abstractChildren.value = [] }
    finally { abstractLoading.value = false }
  }, { immediate: true })

  function handleChildTagClick(child: TagHierarchyNode) {
    emit.selectChildTag(child.slug, child.label)
  }

  function handleAbstractTagClick() {
    if (props.abstractNodeSlug) emit.selectAbstractTag(props.abstractNodeSlug)
  }

  function openMergeDialog() { showMergeDialog.value = true; mergeSearchQuery.value = ''; mergeSearchResults.value = []; mergeError.value = null; mergeSuccess.value = null }
  function closeMergeDialog() { showMergeDialog.value = false }

  function onMergeSearchInput() {
    if (mergeSearchTimer) clearTimeout(mergeSearchTimer)
    mergeError.value = null
    if (!mergeSearchQuery.value.trim()) { mergeSearchResults.value = []; return }
    mergeSearchTimer = setTimeout(async () => {
      mergeSearching.value = true
      try {
        const res = await topicGraphApi.searchTags(mergeSearchQuery.value, displayTopicCategory.value, 10)
        if (res.success && res.data) {
          const currentId = props.detail?.topic?.id
          mergeSearchResults.value = (res.data as any[]).filter(t => t.id !== currentId)
        }
      } catch { mergeError.value = '搜索失败' }
      finally { mergeSearching.value = false }
    }, 300)
  }

  async function doMerge(targetTagId: number, targetLabel: string) {
    if (!props.detail?.topic?.id) return
    mergeMerging.value = true; mergeError.value = null
    try {
      const res = await topicGraphApi.mergeTags(props.detail.topic.id, targetTagId)
      if (res.success) {
        mergeSuccess.value = `已合并到「${targetLabel}」`
        setTimeout(() => { closeMergeDialog(); emit.tagMerged() }, 800)
      } else mergeError.value = res.error || '合并失败'
    } catch (e: any) { mergeError.value = e?.message || '合并失败' }
    finally { mergeMerging.value = false }
  }

  return {
    internalSelectedKeyword, activeKeywordSlug,
    showMergeDialog, mergeSearchQuery, mergeSearchResults,
    mergeSearching, mergeMerging, mergeError, mergeSuccess,
    abstractChildren, abstractLoading,
    topicCategoryLabels, deduplicatedArticles, keywords,
    shouldScrollFeaturedArticles, displayTopicCategory,
    handleKeywordSelect,
    openMergeDialog, closeMergeDialog, onMergeSearchInput, doMerge,
    handleChildTagClick, handleAbstractTagClick,
  }
}
