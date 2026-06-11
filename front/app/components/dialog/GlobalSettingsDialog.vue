<script setup lang="ts">
import { Icon } from '@iconify/vue'
import AIRouterSettingsPanel from '~/features/ai/components/AIRouterSettingsPanel.vue'
import EmbeddingConfigPanel from '~/features/ai/components/EmbeddingConfigPanel.vue'
import EmbeddingQueuePanel from '~/features/ai/components/EmbeddingQueuePanel.vue'
import TagQueuePanel from '~/features/topic-graph/components/TagQueuePanel.vue'
import FeedSettingsPanel from '~/components/dialog/FeedSettingsPanel.vue'
import ReadingPreferencesPanel from '~/components/dialog/ReadingPreferencesPanel.vue'
import FirecrawlConfigPanel from '~/components/dialog/FirecrawlConfigPanel.vue'
import SchedulerStatusPanel from '~/components/dialog/SchedulerStatusPanel.vue'
import { useGlobalSettings } from '~/composables/useGlobalSettings'

interface Props {
  show: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const {
  activeTab, collapsedCategories, loading, error, success,
  feedsByCategory, refreshOptions, maxArticlesOptions,
  updateFeedSetting, refreshFeed,
  testAIConnection,
} = useGlobalSettings()

function close() {
  emit('update:show', false)
}
</script>

<template>
  <div
    v-if="props.show"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    @click.self="close"
  >
    <div
      class="bg-white rounded-xl shadow-xl w-full max-w-4xl mx-4 overflow-hidden max-h-[90vh] flex flex-col"
      style="max-height: calc(90vh - 2rem);"
      @click.stop
    >
      <!-- Header -->
      <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between flex-shrink-0">
        <h2 class="text-xl font-bold text-gray-900">RSS 阅读器设置</h2>
        <button class="p-2 hover:bg-gray-100 rounded-lg transition-colors" @click="close">
          <Icon icon="mdi:close" width="20" height="20" />
        </button>
      </div>

      <!-- Tabs -->
      <div class="flex border-b border-gray-200 flex-shrink-0">
        <button
          v-for="tab in ([
            { key: 'feeds', label: '订阅源配置' },
            { key: 'general', label: '通用设置' },
            { key: 'queues', label: '标签 & 队列' },
            { key: 'preferences', label: '阅读偏好' },
            { key: 'firecrawl', label: 'Firecrawl' },
            { key: 'schedulers', label: '定时任务' },
          ] as const)"
          :key="tab.key"
          class="px-6 py-3 text-sm font-medium transition-colors"
          :class="activeTab === tab.key ? 'text-blue-600 border-b-2 border-blue-600' : 'text-gray-500 hover:text-gray-700'"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Success/Error messages (dialog-level only) -->
      <div
        v-if="success"
        class="mx-6 mt-4 p-3 bg-green-50 border border-green-200 rounded-lg text-sm text-green-600 flex-shrink-0"
      >{{ success }}</div>
      <div
        v-if="error"
        class="mx-6 mt-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600 flex-shrink-0"
      >{{ error }}</div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-6 min-h-0">
        <FeedSettingsPanel
          v-if="activeTab === 'feeds'"
          :feeds-by-category="feedsByCategory"
          :collapsed-categories="collapsedCategories"
          :refresh-options="refreshOptions"
          :max-articles-options="maxArticlesOptions"
          :loading="loading"
          @update-feed="updateFeedSetting"
          @refresh-feed="refreshFeed"
          @toggle-collapse="collapsedCategories[$event] = !collapsedCategories[$event]"
        />

        <ReadingPreferencesPanel v-if="activeTab === 'preferences'" />

        <div v-if="activeTab === 'general'" class="space-y-6">
          <AIRouterSettingsPanel />
          <EmbeddingConfigPanel />
        </div>

        <div v-if="activeTab === 'queues'" class="space-y-6">
          <EmbeddingQueuePanel />
          <TagQueuePanel />
        </div>

        <FirecrawlConfigPanel v-if="activeTab === 'firecrawl'" />

        <SchedulerStatusPanel v-if="activeTab === 'schedulers'" />
      </div>

      <!-- Footer -->
      <div class="px-6 py-4 bg-gray-50 border-t border-gray-200 flex justify-end flex-shrink-0">
        <button class="btn btn-primary" @click="close">完成</button>
      </div>
    </div>
  </div>
</template>
