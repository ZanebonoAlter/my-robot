<script setup lang="ts">
import { computed } from 'vue'

interface MergeSearchResult {
  id: number
  label: string
  category: string
  feed_count: number
}

interface Props {
  show: boolean
  topicLabel?: string
  searchQuery: string
  searchResults: MergeSearchResult[]
  searching: boolean
  merging: boolean
  error: string | null
  success: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  searchInput: [query: string]
  doMerge: [tagId: number, tagLabel: string]
}>()

const showDialog = computed({
  get: () => props.show,
  set: (val: boolean) => { if (!val) emit('close') }
})
</script>

<template>
  <AppDialog v-model="showDialog" :title="`合并「${topicLabel || '当前标签'}」`" width="420px">
    <p class="merge-desc">
      搜索要合并到的目标标签。合并后当前标签的所有文章将迁移到目标标签。
    </p>
    <AppInput
      :model-value="searchQuery"
      placeholder="搜索标签..."
      @update:model-value="emit('searchInput', $event as string)"
    />
    <div v-if="searching" class="merge-status">搜索中...</div>
    <div v-else-if="error" class="merge-status merge-status--error">{{ error }}</div>
    <div v-else-if="success" class="merge-status merge-status--success">{{ success }}</div>
    <div v-else-if="searchResults.length" class="merge-results">
      <button
        v-for="tag in searchResults"
        :key="tag.id"
        type="button"
        class="merge-result-item"
        :disabled="merging"
        @click="emit('doMerge', tag.id, tag.label)"
      >
        <span class="merge-result-label">{{ tag.label }}</span>
        <span class="merge-result-meta">{{ tag.category }} · {{ tag.feed_count }} feeds</span>
      </button>
    </div>
    <div v-else-if="searchQuery.trim()" class="merge-status">无匹配结果</div>
  </AppDialog>
</template>

<style scoped>
.merge-desc {
  font-size: 0.82rem;
  color: var(--color-text-muted);
  margin-bottom: 0.75rem;
}

.merge-status {
  margin-top: 0.75rem;
  font-size: 0.82rem;
  color: var(--color-text-muted);
}

.merge-status--error { color: #f87171; }
.merge-status--success { color: #4ade80; }

.merge-results {
  margin-top: 0.75rem;
  display: grid;
  gap: 0.5rem;
}

.merge-result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  padding: 0.6rem 0.9rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.merge-result-item:hover:not(:disabled) {
  border-color: var(--color-accent);
  background: var(--color-accent-subtle);
}

.merge-result-item:disabled {
  opacity: 0.5;
  cursor: wait;
}

.merge-result-label {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--color-text-primary);
}

.merge-result-meta {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}
</style>
