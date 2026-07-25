<script setup lang="ts">
import { computed } from 'vue'
import AIRouterSettingsPanel from '~/features/ai/components/AIRouterSettingsPanel.vue'
import EmbeddingQueuePanel from '~/features/ai/components/EmbeddingQueuePanel.vue'
import TagQueuePanel from '~/features/settings/components/TagQueuePanel.vue'
import FeedSettingsPanel from '~/components/dialog/FeedSettingsPanel.vue'
import FirecrawlConfigPanel from '~/components/dialog/FirecrawlConfigPanel.vue'
import SchedulerStatusPanel from '~/components/dialog/SchedulerStatusPanel.vue'
import { useGlobalSettings } from '~/composables/useGlobalSettings'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const showDialog = computed({
  get: () => props.show,
  set: (val: boolean) => { emit('update:show', val) }
})

const {
  activeTab, collapsedCategories, loading, error, success,
  feedsByCategory, refreshOptions, maxArticlesOptions,
  updateFeedSetting, refreshFeed,
  testAIConnection,
} = useGlobalSettings()
</script>

<template>
  <AppDialog v-model="showDialog" title="设置" width="900px">
    <!-- Tabs -->
    <nav class="settings-tabs">
      <button
        v-for="tab in ([
          { key: 'feeds', label: '订阅源配置' },
          { key: 'general', label: '通用设置' },
          { key: 'queues', label: '标签 & 队列' },
          { key: 'firecrawl', label: 'Firecrawl' },
          { key: 'schedulers', label: '定时任务' },
        ] as const)"
        :key="tab.key"
        class="settings-tab"
        :class="{ 'settings-tab--active': activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </nav>

    <!-- Success/Error messages -->
    <div v-if="success" class="msg msg--success">{{ success }}</div>
    <div v-if="error" class="msg msg--error">{{ error }}</div>

    <!-- Content -->
    <div class="settings-content">
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

      <div v-if="activeTab === 'general'" class="space-y-6">
        <AIRouterSettingsPanel />
      </div>

      <div v-if="activeTab === 'queues'" class="space-y-6">
        <EmbeddingQueuePanel />
        <TagQueuePanel />
      </div>

      <FirecrawlConfigPanel v-if="activeTab === 'firecrawl'" />

      <SchedulerStatusPanel v-if="activeTab === 'schedulers'" />
    </div>
  </AppDialog>
</template>

<style scoped>
.settings-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--color-border-subtle);
  margin: -20px -20px 16px;
  padding: 0 20px;
}

.settings-tab {
  padding: 10px 24px;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}

.settings-tab:hover {
  color: var(--color-text-secondary);
}

.settings-tab--active {
  color: var(--color-accent);
  border-bottom-color: var(--color-accent);
}

.settings-content {
  min-height: 0;
}

.msg {
  padding: 0.75rem;
  border-radius: 8px;
  font-size: 0.875rem;
  margin-bottom: 12px;
}

.msg--success {
  background: rgba(74, 222, 128, 0.1);
  border: 1px solid rgba(74, 222, 128, 0.2);
  color: #4ade80;
}

.msg--error {
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.2);
  color: #f87171;
}
</style>
