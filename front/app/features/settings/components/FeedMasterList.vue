<script setup lang="ts">
import { computed, ref } from 'vue'
import { Icon } from '@iconify/vue'
import FeedIcon from '~/components/feed/FeedIcon.vue'
import type { RssFeed } from '~/types'

const props = defineProps<{
  feedsByCategory: Record<string, RssFeed[]>
  collapsedCategories: Record<string, boolean>
}>()

const emit = defineEmits<{
  select: [feedId: string]
  toggleCollapse: [categoryName: string]
}>()

const selectedFeedId = defineModel<string>('selectedFeedId')
const searchQuery = ref('')

const filteredByCategory = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  const result: Record<string, RssFeed[]> = {}
  for (const [cat, feeds] of Object.entries(props.feedsByCategory)) {
    const filtered = q
      ? feeds.filter(f => f.title.toLowerCase().includes(q) || f.url.toLowerCase().includes(q))
      : feeds
    if (filtered.length > 0) result[cat] = filtered
  }
  return result
})

const totalFeeds = computed(() =>
  Object.values(props.feedsByCategory).reduce((sum, feeds) => sum + feeds.length, 0)
)

function formatStatus(feed: RssFeed): string {
  if (feed.refreshStatus === 'refreshing') return '刷新中…'
  if (feed.refreshStatus === 'error') return '刷新失败'
  if (feed.lastRefreshAt) {
    const d = new Date(feed.lastRefreshAt)
    const now = new Date()
    const diffMin = Math.floor((now.getTime() - d.getTime()) / 60000)
    if (diffMin < 1) return '刚刚刷新'
    if (diffMin < 60) return `${diffMin} 分钟前`
    const diffHr = Math.floor(diffMin / 60)
    if (diffHr < 24) return `${diffHr} 小时前`
    return `${Math.floor(diffHr / 24)} 天前`
  }
  return '未刷新'
}
</script>

<template>
  <div class="feed-master">
    <!-- Search -->
    <div class="feed-master__search">
      <Icon icon="mdi:magnify" width="16" height="16" class="feed-master__search-icon" />
      <input
        v-model="searchQuery"
        type="text"
        placeholder="搜索订阅源…"
        class="feed-master__search-input"
      />
      <span v-if="totalFeeds > 0" class="feed-master__count">{{ totalFeeds }}</span>
    </div>

    <!-- Feed list -->
    <div class="feed-master__list">
      <div v-if="totalFeeds === 0" class="feed-master__empty">
        <Icon icon="mdi:rss-off" width="32" height="32" style="color: var(--color-text-muted)" />
        <p style="color: var(--color-text-muted)">还没有订阅源</p>
      </div>

      <template v-for="(feeds, categoryName) in filteredByCategory" :key="categoryName">
        <!-- Category header -->
        <button
          class="feed-master__category"
          @click="emit('toggleCollapse', categoryName)"
        >
          <Icon
            :icon="collapsedCategories[categoryName] ? 'mdi:chevron-right' : 'mdi:chevron-down'"
            width="14"
            height="14"
          />
          <Icon icon="mdi:folder" width="14" height="14" />
          <span class="feed-master__category-name">{{ categoryName }}</span>
          <span class="feed-master__category-count">{{ feeds.length }}</span>
        </button>

        <!-- Feed items -->
        <div v-show="!collapsedCategories[categoryName]">
          <button
            v-for="feed in feeds"
            :key="feed.id"
            class="feed-master__item"
            :class="{ 'feed-master__item--active': selectedFeedId === feed.id }"
            @click="selectedFeedId = feed.id; emit('select', feed.id)"
          >
            <div
              class="feed-master__item-icon"
              :style="{ backgroundColor: (feed.color || '#999') + '18' }"
            >
              <FeedIcon :icon="feed.icon" :feed-id="feed.id" :color="feed.color" :size="16" />
            </div>
            <div class="feed-master__item-info">
              <span class="feed-master__item-title">{{ feed.title }}</span>
              <span class="feed-master__item-meta">
                {{ formatStatus(feed) }}
                <template v-if="feed.articleCount"> · {{ feed.articleCount }} 篇</template>
              </span>
            </div>
          </button>
        </div>
      </template>

      <div v-if="Object.keys(filteredByCategory).length === 0 && totalFeeds > 0" class="feed-master__empty">
        <Icon icon="mdi:magnify-close" width="32" height="32" style="color: var(--color-text-muted)" />
        <p style="color: var(--color-text-muted)">未找到匹配的订阅源</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.feed-master {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
}

.feed-master__search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.feed-master__search-icon {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.feed-master__search-input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--color-text-primary);
  font-size: 13px;
  outline: none;
}

.feed-master__search-input::placeholder {
  color: var(--color-text-muted);
}

.feed-master__count {
  font-size: 11px;
  color: var(--color-text-muted);
  background: var(--color-bg-sunken);
  padding: 1px 6px;
  border-radius: 10px;
  flex-shrink: 0;
}

.feed-master__list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.feed-master__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 16px;
  font-size: 13px;
}

.feed-master__category {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s;
}

.feed-master__category:hover {
  background: var(--color-bg-hover);
}

.feed-master__category-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feed-master__category-count {
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-muted);
}

.feed-master__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px 8px 28px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s;
  border-left: 2px solid transparent;
}

.feed-master__item:hover {
  background: var(--color-bg-hover);
}

.feed-master__item--active {
  background: var(--color-accent-subtle);
  border-left-color: var(--color-accent);
}

.feed-master__item-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.feed-master__item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.feed-master__item-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feed-master__item-meta {
  font-size: 11px;
  color: var(--color-text-muted);
}
</style>
