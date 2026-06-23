<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useGlobalSettings } from '~/composables/useGlobalSettings'
import { useApiStore } from '~/stores/api'
import FeedMasterList from './FeedMasterList.vue'
import FeedDetailEditor from './FeedDetailEditor.vue'

const apiStore = useApiStore()
const {
  collapsedCategories, loading, error, success,
  feedsByCategory, refreshOptions, maxArticlesOptions,
  updateFeedSetting, refreshFeed,
} = useGlobalSettings()

const selectedFeedId = ref<string | undefined>()

// Fetch ALL feeds when entering settings (main page may have filtered by category)
onMounted(() => {
  apiStore.fetchFeeds({ per_page: 10000 })
})

const allFeeds = computed(() =>
  Object.values(feedsByCategory.value).flat()
)

const selectedFeed = computed(() =>
  allFeeds.value.find(f => f.id === selectedFeedId.value)
)
</script>

<template>
  <div class="feeds-section">
    <div v-if="success" class="msg msg--success">{{ success }}</div>
    <div v-if="error" class="msg msg--error">{{ error }}</div>

    <div class="feeds-layout">
      <!-- Master list (left) -->
      <div class="feeds-master">
        <FeedMasterList
          v-model:selected-feed-id="selectedFeedId"
          :feeds-by-category="feedsByCategory"
          :collapsed-categories="collapsedCategories"
          @toggle-collapse="collapsedCategories[$event] = !collapsedCategories[$event]"
        />
      </div>

      <!-- Detail editor (right) -->
      <div class="feeds-detail">
        <FeedDetailEditor
          v-if="selectedFeed"
          :key="selectedFeed.id"
          :feed="selectedFeed"
          :refresh-options="refreshOptions"
          :max-articles-options="maxArticlesOptions"
          :loading="loading"
          @update-feed="updateFeedSetting"
          @refresh-feed="refreshFeed"
        />

        <!-- Empty state -->
        <div v-else class="feeds-detail__empty">
          <Icon icon="mdi:rss" width="48" height="48" style="color: var(--color-text-muted); opacity: 0.4" />
          <p class="feeds-detail__empty-title">选择一个订阅源</p>
          <p class="feeds-detail__empty-desc">从左侧列表中选择一个订阅源来查看和编辑配置</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.feeds-section {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.feeds-layout {
  display: flex;
  flex: 1;
  min-height: 0;
  border: 1px solid var(--color-border-subtle);
  border-radius: 12px;
  overflow: hidden;
  background: var(--color-bg-elevated);
}

.feeds-master {
  width: 280px;
  flex-shrink: 0;
  border-right: 1px solid var(--color-border-subtle);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.feeds-detail {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 20px;
}

.feeds-detail__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 100%;
  min-height: 200px;
}

.feeds-detail__empty-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--color-text-secondary);
  margin: 0;
}

.feeds-detail__empty-desc {
  font-size: 13px;
  color: var(--color-text-muted);
  margin: 0;
  text-align: center;
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

/* Responsive: stack on narrow */
@media (max-width: 768px) {
  .feeds-layout {
    flex-direction: column;
  }

  .feeds-master {
    width: 100%;
    max-height: 40vh;
    border-right: none;
    border-bottom: 1px solid var(--color-border-subtle);
  }
}
</style>
