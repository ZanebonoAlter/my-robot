import { computed, ref, watch, type Ref } from 'vue'
import { useTopicGraphApi, type HotspotDigestCard, type TopicCategory, type TopicGraphDetailPayload } from '~/api/topicGraph'
import type { TimelineDigest, TimelineDigestSelection, TimelineAggregationGroup, TimelineAggregationMode, TimelineAggregationArticle } from '~/types/timeline'
import { normalizeTopicCategory } from '~/features/topic-graph/utils/normalizeTopicCategory'

export function useTopicTimeline(
  detail: Ref<TopicGraphDetailPayload | null>,
  hotspotDigests: Ref<HotspotDigestCard[]>,
  selectedHotspotTag: Ref<{ slug: string; label: string; category: TopicCategory } | null>,
) {
  const topicGraphApi = useTopicGraphApi()

  // ---- Aggregation state ----
  const aggregationMode = ref<TimelineAggregationMode>('day')
  const aggregatedArticles = ref<TimelineAggregationArticle[]>([])
  const loadingAggregatedArticles = ref(false)
  const selectedGroupKey = ref<string | null>(null)

  // ---- Digest selection ----
  const selectedDigestId = ref<string | null>(null)
  const previewDigestId = ref<string | null>(null)

  // ---- Floating panel ----
  const timelineOpen = ref(false)

  // ---- Timeline items (computed from detail / hotspotDigests) ----
  const timelineItems = computed((): TimelineDigest[] => {
    const summaries = detail.value?.summaries || []
    return summaries.map(summary => ({
      id: String(summary.id),
      title: summary.title,
      summary: summary.summary,
      createdAt: summary.created_at,
      feedName: summary.feed_name,
      feedIcon: summary.feed_icon,
      categoryName: summary.category_name,
      articleCount: summary.article_count,
      tags: summary.aggregated_tags.map(topic => ({
        slug: topic.slug,
        label: topic.label,
        category: normalizeTopicCategory(topic.category, topic.kind),
      })),
      articles: summary.articles.map(article => ({
        id: article.id,
        title: article.title,
        link: article.link,
      })),
    }))
  })

  const hotspotTimelineItems = computed((): TimelineDigest[] => {
    if (!hotspotDigests.value.length) return []
    return hotspotDigests.value.map(digest => ({
      id: String(digest.id),
      title: digest.title,
      summary: digest.summary,
      createdAt: digest.created_at,
      feedName: digest.feed_name,
      feedIcon: digest.feed_icon,
      categoryName: digest.category_name,
      articleCount: digest.article_count,
      tags: (digest.aggregated_tags || []).map(tag => ({
        slug: tag.slug,
        label: tag.label,
        category: normalizeTopicCategory(tag.category, tag.kind),
      })),
      articles: digest.matched_articles?.map(article => ({
        id: article.id,
        title: article.title,
        link: '',
        feedName: article.feed_name,
        feedIcon: article.feed_icon,
        feedColor: article.feed_color,
      })) || [],
    }))
  })

  const effectiveTimelineItems = computed((): TimelineDigest[] => {
    if (selectedHotspotTag.value && hotspotDigests.value.length > 0) {
      return hotspotTimelineItems.value
    }
    return timelineItems.value
  })

  const selectedDigest = computed<TimelineDigestSelection | null>(() => {
    if (!selectedDigestId.value) return null
    const digest = effectiveTimelineItems.value.find(item => item.id === selectedDigestId.value)
    if (!digest) return null

    if (selectedHotspotTag.value && hotspotDigests.value.length > 0) {
      const hotspotDigest = hotspotDigests.value.find(d => String(d.id) === selectedDigestId.value)
      if (hotspotDigest) {
        return {
          ...digest,
          matchedArticleIds: hotspotDigest.matched_articles?.map(a => a.id) || [],
          matchedArticlesTags: hotspotDigest.matched_articles_tags?.map(tag => ({
            slug: tag.slug,
            label: tag.label,
            category: normalizeTopicCategory(tag.category, tag.kind),
          })),
        }
      }
    }

    if (!detail.value) return null
    const topicArticleIds = new Set(detail.value.articles.map(article => article.id))
    return {
      ...digest,
      matchedArticleIds: digest.articles.map(a => a.id).filter(id => topicArticleIds.has(id)),
    }
  })

  const previewDigest = computed(() => {
    if (!previewDigestId.value) return null
    return effectiveTimelineItems.value.find(item => item.id === previewDigestId.value) || null
  })

  // ---- Aggregation computed ----
  const timelineAggregationGroups = computed((): TimelineAggregationGroup[] => {
    if (!aggregatedArticles.value.length) return []
    const groupMap = new Map<string, TimelineAggregationArticle[]>()

    for (const article of aggregatedArticles.value) {
      const date = new Date(article.pubDate)
      if (Number.isNaN(date.getTime())) continue

      let key: string
      let startDate: Date
      let endDate: Date

      if (aggregationMode.value === 'hour') {
        const hour = date.getHours()
        key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}-${hour}`
        startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour, 0, 0)
        endDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour + 1, 0, 0)
      } else {
        key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
        startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0)
        endDate = new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1, 0, 0, 0)
      }

      if (!groupMap.has(key)) groupMap.set(key, [])
      groupMap.get(key)!.push(article)
    }

    const sortedKeys = Array.from(groupMap.keys()).sort((a, b) => b.localeCompare(a))
    const groups: TimelineAggregationGroup[] = []
    for (const key of sortedKeys) {
      const articles = groupMap.get(key)!
      if (!articles.length) continue
      const first = articles[0]!
      const date = new Date(first.pubDate)
      let startDate: Date
      let endDate: Date
      if (aggregationMode.value === 'hour') {
        const hour = date.getHours()
        startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour, 0, 0)
        endDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour + 1, 0, 0)
      } else {
        startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0)
        endDate = new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1, 0, 0, 0)
      }
      groups.push({ key, label: '', startDate, endDate, articles })
    }
    return groups
  })

  const totalAggregatedCount = computed(() => aggregatedArticles.value.length)

  const selectedGroupArticles = computed((): TimelineAggregationArticle[] => {
    if (!selectedGroupKey.value) return aggregatedArticles.value
    const group = timelineAggregationGroups.value.find(g => g.key === selectedGroupKey.value)
    return group?.articles || []
  })

  // ---- Data loading ----
  async function loadAggregatedArticles(tagSlug: string, hotspotData?: { events: any[]; people: any[]; keywords: any[] } | null) {
    loadingAggregatedArticles.value = true
    try {
      const isAbstract = hotspotData && [
        ...hotspotData.events, ...hotspotData.people, ...hotspotData.keywords,
      ].find((t: any) => t.slug === tagSlug)?.is_abstract

      const childSlugs = hotspotData && [
        ...hotspotData.events, ...hotspotData.people, ...hotspotData.keywords,
      ].find((t: any) => t.slug === tagSlug)?.child_slugs

      const slugs = (isAbstract && childSlugs?.length) ? childSlugs : [tagSlug]
      const allArticles: TimelineAggregationArticle[] = []
      const seenIds = new Set<string>()

      for (const slug of slugs) {
        const response = await topicGraphApi.getTopicArticles({ slug, page: 1, pageSize: 100 })
        if (response.success && response.data) {
          for (const article of response.data.articles) {
            if (!seenIds.has(article.id)) {
              seenIds.add(article.id)
              allArticles.push({
                id: article.id,
                title: article.title,
                link: article.link,
                pubDate: article.pub_date,
                feedName: article.feed_name,
                feedIcon: article.feed_icon || '',
                tags: (article.tags || []).map((t: any) => ({
                  slug: t.slug, label: t.label, category: t.category,
                })),
              })
            }
          }
        }
      }

      allArticles.sort((a, b) => new Date(b.pubDate).getTime() - new Date(a.pubDate).getTime())
      aggregatedArticles.value = allArticles
    } catch (error) {
      console.error('Failed to load aggregated articles:', error)
      aggregatedArticles.value = []
    } finally {
      loadingAggregatedArticles.value = false
    }
  }

  // ---- Handlers ----
  function handleDigestSelect(digestId: string) {
    selectedDigestId.value = digestId
  }

  function handlePreviewDigest(digestId: string) {
    selectedDigestId.value = digestId
    previewDigestId.value = digestId
  }

  function closeDigestPreview() {
    previewDigestId.value = null
  }

  function handleTimelineGroupSelect(groupKey: string) {
    selectedGroupKey.value = groupKey
    selectedDigestId.value = null
  }

  function handleAggregationModeChange(mode: TimelineAggregationMode) {
    aggregationMode.value = mode
    selectedGroupKey.value = null
  }

  // ---- Watchers ----
  watch(effectiveTimelineItems, (items) => {
    if (!items.length) {
      selectedDigestId.value = null
      previewDigestId.value = null
      return
    }
    const currentExists = selectedDigestId.value && items.some(item => item.id === selectedDigestId.value)
    if (!currentExists) {
      selectedDigestId.value = items[0]?.id || null
    }
  }, { immediate: true })

  watch(timelineAggregationGroups, (groups) => {
    if (!groups.length) { selectedGroupKey.value = null; return }
    const currentExists = selectedGroupKey.value && groups.some(g => g.key === selectedGroupKey.value)
    if (!currentExists) selectedGroupKey.value = groups[0]?.key || null
  })

  return {
    aggregationMode, aggregatedArticles, loadingAggregatedArticles, selectedGroupKey,
    selectedDigestId, previewDigestId, timelineOpen,
    timelineItems, hotspotTimelineItems, effectiveTimelineItems,
    selectedDigest, previewDigest,
    timelineAggregationGroups, totalAggregatedCount, selectedGroupArticles,
    loadAggregatedArticles,
    handleDigestSelect, handlePreviewDigest, closeDigestPreview,
    handleTimelineGroupSelect, handleAggregationModeChange,
  }
}
