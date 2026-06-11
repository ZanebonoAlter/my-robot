import { computed, ref, watch, onBeforeUnmount, type Ref } from 'vue'
import { useTopicGraphApi, type TopicCategory, type TopicsByCategoryPayload } from '~/api/topicGraph'
import type { TopicGraphViewModel } from '~/features/topic-graph/utils/buildTopicGraphViewModel'
import { collectRelatedTopicSlugs } from '~/features/topic-graph/utils/buildDisplayedTopicGraph'
import { normalizeTopicCategory } from '~/features/topic-graph/utils/normalizeTopicCategory'

interface HotspotTag {
  slug: string
  label: string
  category: TopicCategory
}

export function useHotspotTopics(viewModel: Ref<TopicGraphViewModel>) {
  const topicGraphApi = useTopicGraphApi()

  // ---- Hotspot topic data ----
  const hotspotData = ref<TopicsByCategoryPayload | null>(null)
  const loadingHotspots = ref(false)

  // ---- Hotspot digests ----
  const hotspotDigests = ref<any[]>([])
  const loadingHotspotDigests = ref(false)
  const selectedHotspotTag = ref<HotspotTag | null>(null)

  // ---- Pending articles ----
  const pendingArticles = ref<any[]>([])
  const selectedPendingNode = ref(false)
  const loadingPendingArticles = ref(false)

  // ---- Graph visibility ----
  const graphVisibilityOverrides = ref<Record<string, boolean>>({})
  const expandedTopicSlugs = ref<string[]>([])

  // ---- Hotspot search / dropdown UI ----
  const hotspotSearchQueries = ref<Record<string, string>>({ event: '', person: '', keyword: '' })
  const hotspotDropdownOpen = ref<Record<string, boolean>>({ event: false, person: false, keyword: false })
  const hotspotShowAll = ref<Record<string, boolean>>({ event: false, person: false, keyword: false })
  const hotspotSearchRefs = ref<Record<string, HTMLDivElement | null>>({ event: null, person: null, keyword: null })

  // ---- Abstract tag tracking (notifies peers) ----
  const abstractNodeSlug = ref<string | null>(null)
  const abstractNodeLabel = ref<string | null>(null)

  // ---- Graph visibility computed ----
  const defaultGraphTopicSlugs = computed(() => {
    const slugs = new Set<string>()
    const lowQualitySlugs = new Set<string>()
    viewModel.value.topTopics.forEach((topic) => {
      if (topic.is_low_quality && topic.slug) lowQualitySlugs.add(topic.slug)
    })
    viewModel.value.graph.nodes.forEach((node) => {
      if (node.kind === 'topic' && node.slug && !lowQualitySlugs.has(node.slug)) {
        slugs.add(node.slug)
      }
    })
    return slugs
  })

  const graphVisibleTopicSlugs = computed(() => {
    const slugs = new Set(defaultGraphTopicSlugs.value)
    expandedTopicSlugs.value.forEach(slug => slugs.add(slug))
    Object.entries(graphVisibilityOverrides.value).forEach(([slug, visible]) => {
      if (visible) { slugs.add(slug); return }
      slugs.delete(slug)
    })
    return slugs
  })

  // ---- Graph visibility helpers ----
  function isTopicShownInGraph(slug: string) {
    return graphVisibleTopicSlugs.value.has(slug)
  }

  function ensureTopicShownInGraph(slug: string) {
    if (isTopicShownInGraph(slug)) return
    graphVisibilityOverrides.value = { ...graphVisibilityOverrides.value, [slug]: true }
  }

  function toggleTopicGraphVisibility(slug: string) {
    const nextVisible = !isTopicShownInGraph(slug)
    const defaultVisible = defaultGraphTopicSlugs.value.has(slug)
    const nextOverrides = { ...graphVisibilityOverrides.value }
    if (nextVisible === defaultVisible) {
      delete nextOverrides[slug]
    } else {
      nextOverrides[slug] = nextVisible
    }
    graphVisibilityOverrides.value = nextOverrides
  }

  function expandRelatedTopics(slug: string) {
    const relatedSlugs = collectRelatedTopicSlugs(viewModel.value.graph, slug)
    const nextExpanded = new Set(expandedTopicSlugs.value)
    const nextOverrides = { ...graphVisibilityOverrides.value }
    nextExpanded.add(slug)
    nextOverrides[slug] = true
    relatedSlugs.forEach((relatedSlug) => {
      nextExpanded.add(relatedSlug)
      nextOverrides[relatedSlug] = true
    })
    expandedTopicSlugs.value = Array.from(nextExpanded)
    graphVisibilityOverrides.value = nextOverrides
  }

  // ---- Hotspot topic helpers ----
  function filterTopics(topics: any[], query: string) {
    if (!query.trim()) return topics
    const lowerQuery = query.toLowerCase()
    return topics.filter((topic: any) =>
      topic.label.toLowerCase().includes(lowerQuery)
      || topic.slug.toLowerCase().includes(lowerQuery)
    )
  }

  function sortTopicsByFrequency<T extends { score: number; quality_score?: number; is_low_quality?: boolean }>(topics: T[]) {
    return [...topics].sort((left, right) => {
      const leftLowQuality = left.is_low_quality ? 1 : 0
      const rightLowQuality = right.is_low_quality ? 1 : 0
      if (leftLowQuality !== rightLowQuality) return leftLowQuality - rightLowQuality
      const leftQuality = left.quality_score ?? left.score ?? 0
      const rightQuality = right.quality_score ?? right.score ?? 0
      if (rightQuality === leftQuality) return (right.score ?? 0) - (left.score ?? 0)
      return rightQuality - leftQuality
    })
  }

  function buildFallbackTopics(category: TopicCategory) {
    return sortTopicsByFrequency(
      viewModel.value.topTopics.filter(topic =>
        normalizeTopicCategory(topic.category, topic.kind) === category
      )
    )
  }

  function buildCategoryTopicState(category: TopicCategory, query: string, showAll: boolean) {
    const categoryTopics = category === 'event'
      ? hotspotData.value?.events
      : category === 'person'
        ? hotspotData.value?.people
        : hotspotData.value?.keywords
    const sourceTopics = sortTopicsByFrequency(categoryTopics || buildFallbackTopics(category))
    const filteredTopics = filterTopics(sourceTopics, query || '')
    const displayTopics = showAll ? filteredTopics : filteredTopics.slice(0, 8)
    return {
      topics: filteredTopics,
      filteredTopics,
      displayTopics,
      hasMore: filteredTopics.length > displayTopics.length,
      hiddenLowQualityCount: filteredTopics.filter((t: any) => t.is_low_quality && !t.is_abstract).length,
      showAll,
    }
  }

  const hotspotCategories = computed(() => ([
    {
      key: 'event' as const,
      label: '事件',
      icon: 'mdi:calendar-alert-outline',
      headerClass: 'topic-category-header--event',
      ...buildCategoryTopicState('event', hotspotSearchQueries.value.event || '', !!hotspotShowAll.value.event),
    },
    {
      key: 'person' as const,
      label: '人物',
      icon: 'mdi:account-voice-outline',
      headerClass: 'topic-category-header--person',
      ...buildCategoryTopicState('person', hotspotSearchQueries.value.person || '', !!hotspotShowAll.value.person),
    },
    {
      key: 'keyword' as const,
      label: '关键词',
      icon: 'mdi:key-variant',
      headerClass: 'topic-category-header--keyword',
      ...buildCategoryTopicState('keyword', hotspotSearchQueries.value.keyword || '', !!hotspotShowAll.value.keyword),
    },
  ]))

  function toggleShowAll(categoryKey: string) {
    hotspotShowAll.value[categoryKey] = !hotspotShowAll.value[categoryKey]
  }

  function closeHotspotDropdown(categoryKey: string) {
    hotspotDropdownOpen.value[categoryKey] = false
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as Node
    Object.keys(hotspotSearchRefs.value).forEach((key) => {
      const container = hotspotSearchRefs.value[key]
      if (container && !container.contains(target)) {
        hotspotDropdownOpen.value[key] = false
      }
    })
  }

  watch(() => Object.values(hotspotDropdownOpen.value).some(Boolean), (isAnyOpen) => {
    if (isAnyOpen) {
      document.addEventListener('click', handleClickOutside, true)
    } else {
      document.removeEventListener('click', handleClickOutside, true)
    }
  })

  onBeforeUnmount(() => {
    document.removeEventListener('click', handleClickOutside, true)
  })

  // ---- Data loading ----
  async function loadHotspotDigests(tagSlug: string, kind?: TopicCategory) {
    loadingHotspotDigests.value = true
    try {
      const response = await topicGraphApi.getDigestsByArticleTag(tagSlug, undefined, undefined, 20, kind)
      if (response.success && response.data) {
        hotspotDigests.value = response.data.digests || []
      } else {
        hotspotDigests.value = []
      }
    } catch (error) {
      console.error('Failed to load hotspot digests:', error)
      hotspotDigests.value = []
    } finally {
      loadingHotspotDigests.value = false
    }
  }

  async function loadAbstractTagDigests(childSlugs: string[]) {
    loadingHotspotDigests.value = true
    try {
      const results = await Promise.all(
        childSlugs.map(slug => topicGraphApi.getDigestsByArticleTag(slug, undefined, undefined, 20))
      )
      const seenIds = new Set<number>()
      const merged: any[] = []
      for (const response of results) {
        if (!response.success || !response.data) continue
        for (const digest of response.data.digests || []) {
          if (!seenIds.has(digest.id)) {
            seenIds.add(digest.id)
            merged.push(digest)
          }
        }
      }
      merged.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      hotspotDigests.value = merged
    } catch (error) {
      console.error('Failed to load abstract tag digests:', error)
      hotspotDigests.value = []
    } finally {
      loadingHotspotDigests.value = false
    }
  }

  async function loadPendingArticles(tagSlug: string) {
    loadingPendingArticles.value = true
    try {
      const response = await topicGraphApi.getPendingArticlesByTag(tagSlug)
      if (response.success && response.data) {
        pendingArticles.value = (response.data.articles || []).map((article: any) => ({
          id: article.id,
          title: article.title,
          link: article.link,
          pubDate: article.pub_date || article.pubDate,
          feedName: article.feed_name || article.feedName,
          feedIcon: article.feed_icon || article.feedIcon,
          feedColor: article.feed_color || article.feedColor,
        }))
      } else {
        pendingArticles.value = []
      }
    } catch (error) {
      console.error('Failed to load pending articles:', error)
      pendingArticles.value = []
    } finally {
      loadingPendingArticles.value = false
    }
  }

  return {
    hotspotData, loadingHotspots,
    hotspotDigests, loadingHotspotDigests, selectedHotspotTag,
    pendingArticles, selectedPendingNode, loadingPendingArticles,
    graphVisibilityOverrides, expandedTopicSlugs,
    defaultGraphTopicSlugs, graphVisibleTopicSlugs,
    hotspotSearchQueries, hotspotDropdownOpen, hotspotShowAll, hotspotSearchRefs,
    hotspotCategories,
    abstractNodeSlug, abstractNodeLabel,
    isTopicShownInGraph, toggleTopicGraphVisibility, expandRelatedTopics, ensureTopicShownInGraph,
    toggleShowAll, closeHotspotDropdown,
    loadHotspotDigests, loadAbstractTagDigests, loadPendingArticles,
  }
}
