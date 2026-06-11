/**
 * Feeds computed facade — only derives from apiStore, holds no independent state.
 *
 * Provides cached computed helpers (categorizedFeeds, unreadCountsByFeed, etc.)
 * so consumers don't derive these inline. Kept as a store for API consistency;
 * could become a plain composable in the future.
 */
import { defineStore } from 'pinia'
import { useApiStore } from '~/stores/api'
import type { RssFeed, Category } from '~/types'

export const useFeedsStore = defineStore('feeds', () => {
  const apiStore = useApiStore()

  const feeds = computed<RssFeed[]>(() => apiStore.feeds)

  const categories = computed<Category[]>(() => apiStore.categories)

  const feedCount = computed(() => feeds.value.length)

  const categorizedFeeds = computed(() => {
    const grouped: Record<string, RssFeed[]> = {}
    feeds.value.forEach((feed) => {
      if (!grouped[feed.category]) {
        grouped[feed.category] = []
      }
      grouped[feed.category]?.push(feed)
    })
    return grouped
  })

  const getFeedUnreadCount = (feedId: string) => {
    const feed = feeds.value.find(f => f.id === feedId)
    return feed?.unreadCount || 0
  }

  const unreadCountsByFeed = computed(() => {
    const counts: Record<string, number> = {}
    feeds.value.forEach((feed) => {
      counts[feed.id] = feed.unreadCount || 0
    })
    return counts
  })

  function getFeedsByCategory(categoryId: string) {
    return feeds.value.filter(f => f.category === categoryId)
  }

  function getCategoryBySlug(slug: string) {
    return categories.value.find(c => c.slug === slug)
  }

  /**
   * Small cross-store Interface for article mutations that need to sync feed badges.
   * Keeps articlesStore from depending on apiStore's internal feed collection.
   */
  function clearUnreadCounts(matchFeed: (feed: RssFeed) => boolean) {
    for (const feed of feeds.value) {
      if (matchFeed(feed)) {
        feed.unreadCount = 0
      }
    }
  }

  function adjustUnreadCount(feedId: string, delta: number) {
    const feed = feeds.value.find(f => f.id === feedId)
    if (!feed) return
    feed.unreadCount = Math.max(0, (feed.unreadCount || 0) + delta)
  }

  return {
    feeds,
    categories,
    feedCount,
    categorizedFeeds,
    getFeedUnreadCount,
    unreadCountsByFeed,
    getFeedsByCategory,
    getCategoryBySlug,
    clearUnreadCounts,
    adjustUnreadCount,
  }
})
