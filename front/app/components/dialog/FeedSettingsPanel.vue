<script setup lang="ts">
import { Icon } from '@iconify/vue'
import FeedIcon from '~/components/feed/FeedIcon.vue'
import type { RssFeed } from '~/types'

defineProps<{
  feedsByCategory: Record<string, RssFeed[]>
  collapsedCategories: Record<string, boolean>
  refreshOptions: { label: string; value: number }[]
  maxArticlesOptions: { label: string; value: number }[]
  loading: boolean
}>()

const emit = defineEmits<{
  'update-feed': [feedId: string, setting: 'refresh_interval' | 'max_articles' | 'ai_summary_enabled' | 'tagging_enabled' | 'firecrawl_enabled' | 'completion_on_refresh', value: number | boolean]
  'refresh-feed': [feedId: string]
  'toggle-collapse': [categoryName: string]
}>()

function formatInterval(minutes: number): string {
  const options = [
    { label: '手动刷新', value: 0 },
    { label: '每 15 分钟', value: 15 },
    { label: '每 30 分钟', value: 30 },
    { label: '每小时', value: 60 },
    { label: '每 2 小时', value: 120 },
    { label: '每 6 小时', value: 360 },
    { label: '每天', value: 1440 },
  ]
  const option = options.find(opt => opt.value === minutes)
  return option?.label || `${minutes} 分钟`
}

function formatMaxArticles(count: number): string {
  if (count <= 0 || count >= 9999) return '无限制'
  const options = [
    { label: '50 篇', value: 50 },
    { label: '100 篇', value: 100 },
    { label: '200 篇', value: 200 },
    { label: '500 篇', value: 500 },
    { label: '1000 篇', value: 1000 },
    { label: '无限制', value: 0 },
  ]
  const option = options.find(opt => opt.value === count)
  return option?.label || `${count} 篇`
}

function getIntervalColor(minutes: number): string {
  if (minutes === 0) return 'text-gray-500'
  if (minutes <= 30) return 'text-green-600'
  if (minutes <= 120) return 'text-blue-600'
  return 'text-ink-600'
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="Object.keys(feedsByCategory).length === 0" class="text-center text-gray-500 py-8">
      <Icon icon="mdi:rss-off" width="48" height="48" class="mx-auto mb-2 opacity-50" />
      <p>还没有订阅源</p>
    </div>

    <div
      v-for="(feeds, categoryName) in feedsByCategory"
      :key="categoryName"
      class="space-y-3"
    >
      <h3
        class="text-sm font-semibold text-gray-700 flex items-center gap-2 cursor-pointer hover:text-gray-900 select-none"
        @click="emit('toggle-collapse', categoryName)"
      >
        <Icon :icon="collapsedCategories[categoryName] ? 'mdi:chevron-right' : 'mdi:chevron-down'" width="16" height="16" />
        <Icon icon="mdi:folder" width="16" height="16" />
        {{ categoryName }}
        <span class="text-xs font-normal text-gray-400">({{ feeds.length }})</span>
      </h3>

      <div v-show="!collapsedCategories[categoryName]" class="space-y-2">
        <div
          v-for="feed in feeds"
          :key="feed.id"
          class="border border-gray-200 rounded-lg p-4 hover:border-gray-300 transition-colors"
        >
          <div class="flex items-start gap-3">
            <div
              class="w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0"
              :style="{ backgroundColor: feed.color + '15' }"
            >
              <FeedIcon
                :icon="feed.icon"
                :feed-id="feed.id"
                :color="feed.color"
                :size="20"
              />
            </div>

            <div class="flex-1 min-w-0">
              <div class="flex items-start justify-between gap-2 mb-3">
                <div>
                  <h4 class="font-medium text-gray-900 truncate">{{ feed.title }}</h4>
                  <p class="text-xs text-gray-500 truncate">{{ feed.url }}</p>
                </div>
                <button
                  class="p-1.5 hover:bg-gray-100 rounded-lg transition-colors"
                  :title="'立即刷新'"
                  :disabled="loading"
                  @click="emit('refresh-feed', feed.id)"
                >
                  <Icon
                    :icon="loading ? 'mdi:loading' : 'mdi:refresh'"
                    :class="{ 'animate-spin': loading }"
                    width="16"
                    height="16"
                    class="text-gray-500"
                  />
                </button>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-gray-600 mb-1">自动刷新</label>
                  <select
                    :value="feed.refreshInterval"
                    class="w-full text-xs px-2 py-1.5 border border-gray-200 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    @change="emit('update-feed', feed.id, 'refresh_interval', Number(($event.target as HTMLSelectElement).value))"
                  >
                    <option v-for="option in refreshOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-600 mb-1">最大文章数</label>
                  <select
                    :value="feed.maxArticles"
                    class="w-full text-xs px-2 py-1.5 border border-gray-200 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    @change="emit('update-feed', feed.id, 'max_articles', Number(($event.target as HTMLSelectElement).value))"
                  >
                    <option v-for="option in maxArticlesOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </div>
              </div>

              <div class="space-y-2 mt-3 pt-3 border-t border-gray-100">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Icon icon="mdi:web" width="14" height="14" class="text-blue-500" />
                    <span class="text-xs font-medium text-gray-700">Firecrawl</span>
                  </div>
                  <button
                    class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
                    :class="feed.firecrawlEnabled ? 'bg-blue-500' : 'bg-gray-300'"
                    @click="emit('update-feed', feed.id, 'firecrawl_enabled', !feed.firecrawlEnabled)"
                  >
                    <span class="inline-block h-3 w-3 transform rounded-full bg-white transition-transform" :class="feed.firecrawlEnabled ? 'translate-x-5' : 'translate-x-1'" />
                  </button>
                </div>
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Icon icon="mdi:tag-multiple" width="14" height="14" class="text-amber-500" />
                    <span class="text-xs font-medium text-gray-700">打标签</span>
                  </div>
                  <button
                    class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
                    :class="feed.taggingEnabled !== false ? 'bg-amber-500' : 'bg-gray-300'"
                    @click="emit('update-feed', feed.id, 'tagging_enabled', !(feed.taggingEnabled !== false))"
                  >
                    <span class="inline-block h-3 w-3 transform rounded-full bg-white transition-transform" :class="feed.taggingEnabled !== false ? 'translate-x-5' : 'translate-x-1'" />
                  </button>
                </div>
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Icon icon="mdi:text-box-plus" width="14" height="14" class="text-green-500" />
                    <span class="text-xs font-medium text-gray-700">内容补全</span>
                  </div>
                  <button
                    class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors"
                    :class="feed.completionOnRefresh !== false ? 'bg-green-500' : 'bg-gray-300'"
                    @click="emit('update-feed', feed.id, 'completion_on_refresh', !(feed.completionOnRefresh !== false))"
                  >
                    <span class="inline-block h-3 w-3 transform rounded-full bg-white transition-transform" :class="feed.completionOnRefresh !== false ? 'translate-x-5' : 'translate-x-1'" />
                  </button>
                </div>
              </div>

              <div class="flex items-center gap-3 mt-2 text-xs">
                <span :class="getIntervalColor(feed.refreshInterval || 0)">
                  <Icon icon="mdi:clock" width="12" height="12" class="inline-block mr-1" />
                  {{ formatInterval(feed.refreshInterval || 0) }}
                </span>
                <span class="text-gray-500">
                  <Icon icon="mdi:file-document-multiple" width="12" height="12" class="inline-block mr-1" />
                  {{ formatMaxArticles(feed.maxArticles ?? 100) }}
                </span>
                <span class="text-gray-500">
                  <Icon icon="mdi:article" width="12" height="12" class="inline-block mr-1" />
                  {{ feed.articleCount }} 篇
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
