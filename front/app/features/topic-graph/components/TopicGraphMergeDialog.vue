<script setup lang="ts">
import { Icon } from '@iconify/vue'

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
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="merge-overlay" @click.self="emit('close')">
      <div class="merge-dialog">
        <div class="flex items-center justify-between gap-3 mb-4">
          <h3 class="text-lg font-semibold text-gray-100">
            合并「{{ topicLabel || '当前标签' }}」
          </h3>
          <button type="button" class="merge-close-btn" @click="emit('close')">
            <Icon icon="mdi:close" />
          </button>
        </div>
        <p class="text-sm text-gray-400 mb-3">
          搜索要合并到的目标标签。合并后当前标签的所有文章将迁移到目标标签。
        </p>
        <input
          :value="searchQuery"
          type="text"
          class="merge-input"
          placeholder="搜索标签..."
          @input="emit('searchInput', ($event.target as HTMLInputElement).value)"
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
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.merge-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(2, 6, 12, 0.75);
  backdrop-filter: blur(8px);
}
.merge-dialog {
  width: 420px;
  max-width: 92vw;
  max-height: 80vh;
  border-radius: 1.5rem;
  border: 1px solid rgba(200, 210, 225, 0.2);
  background: linear-gradient(180deg, #1a2536, #0e1520);
  box-shadow: 0 32px 80px rgba(2, 6, 12, 0.6);
  padding: 1.5rem;
  overflow-y: auto;
}
.merge-close-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  border: 1px solid rgba(200, 210, 225, 0.15);
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  font-size: 1.1rem;
}
.merge-close-btn:hover {
  color: #e5e7eb;
  border-color: rgba(200, 210, 225, 0.3);
}
.merge-input {
  width: 100%;
  padding: 0.6rem 1rem;
  border-radius: 0.75rem;
  border: 1px solid rgba(200, 210, 225, 0.2);
  background: rgba(10, 16, 23, 0.8);
  color: #f1f5f9;
  font-size: 0.9rem;
  outline: none;
  transition: border-color 0.2s ease;
}
.merge-input:focus { border-color: rgba(240, 138, 75, 0.5); }
.merge-input::placeholder { color: rgba(173, 193, 214, 0.4); }
.merge-status { margin-top: 0.75rem; font-size: 0.82rem; color: #9ca3af; }
.merge-status--error { color: #f87171; }
.merge-status--success { color: #4ade80; }
.merge-results { margin-top: 0.75rem; display: grid; gap: 0.5rem; }
.merge-result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  padding: 0.6rem 0.9rem;
  border-radius: 0.75rem;
  border: 1px solid rgba(200, 210, 225, 0.12);
  background: rgba(14, 21, 30, 0.6);
  color: #f1f5f9;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}
.merge-result-item:hover:not(:disabled) {
  border-color: rgba(240, 138, 75, 0.4);
  background: rgba(240, 138, 75, 0.1);
}
.merge-result-item:disabled { opacity: 0.5; cursor: wait; }
.merge-result-label { font-weight: 600; font-size: 0.9rem; color: #e2e8f0; }
.merge-result-meta { font-size: 0.75rem; color: #94a3b8; }
</style>
