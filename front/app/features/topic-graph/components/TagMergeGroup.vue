<script setup lang="ts">
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import type { MergeGroup, MergeSuggestion } from '~/types/tagMerge'

interface Props {
  group: MergeGroup
  selectedKeys: Set<string>
  searchingGroupId: number | null
  searchQuery: string
  searchResults: Array<{ id: number; label: string; feed_count: number }>
  searchLoading: boolean
  parseVerdict: (verdict: string | null) => { should_merge: boolean; suggested_name: string; reason: string } | null
  formatSimilarity: (s: number) => string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  toggleSelect: [targetTagId: number, newTagId: number]
  selectAllInGroup: [group: MergeGroup]
  deselectAllInGroup: [group: MergeGroup]
  openSearch: [targetTagId: number]
  closeSearch: []
  searchInput: [query: string]
  addTagToGroup: [tagId: number]
  removeSuggestion: [sug: MergeSuggestion, group: MergeGroup]
}>()

function sugKey(targetTagId: number, newTagId: number): string {
  return `${targetTagId}:${newTagId}`
}

function isSugSelected(targetTagId: number, newTagId: number): boolean {
  return props.selectedKeys.has(sugKey(targetTagId, newTagId))
}

const isGroupAllSelected = computed(() => {
  return props.group.suggestions.length > 0 && props.group.suggestions.every(s => isSugSelected(props.group.target_tag_id, s.new_tag_id))
})
</script>

<template>
  <div class="tm-group">
    <!-- Group header: target tag -->
    <div class="tm-group__header">
      <div class="tm-group__target">
        <button
          type="button"
          class="tm-checkbox"
          :class="{ 'tm-checkbox--checked': isGroupAllSelected }"
          @click="isGroupAllSelected ? emit('deselectAllInGroup', group) : emit('selectAllInGroup', group)"
        >
          <Icon v-if="isGroupAllSelected" icon="mdi:check" width="14" />
        </button>
        <span class="tm-group__target-name">{{ group.target_label }}</span>
        <span class="tm-group__target-meta">{{ group.target_articles }} 篇文章</span>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="tm-btn tm-btn--sm"
          @click="emit('openSearch', group.target_tag_id)"
        >
          <Icon icon="mdi:plus" width="14" />
          <span>添加标签</span>
        </button>
      </div>
    </div>

    <!-- Search overlay for this group -->
    <div v-if="searchingGroupId === group.target_tag_id" class="tm-search">
      <div class="tm-search__input-row">
        <input
          :value="searchQuery"
          type="text"
          class="tm-search__input"
          placeholder="搜索标签..."
          autofocus
          @input="emit('searchInput', ($event.target as HTMLInputElement).value)"
          @keyup.escape="emit('closeSearch')"
        />
        <button type="button" class="tm-btn tm-btn--sm" @click="emit('closeSearch')">
          <Icon icon="mdi:close" width="14" />
        </button>
      </div>
      <div v-if="searchLoading" class="tm-search__loading">搜索中...</div>
      <div v-else-if="searchResults.length" class="tm-search__results">
        <button
          v-for="tag in searchResults"
          :key="tag.id"
          type="button"
          class="tm-search__item"
          @click="emit('addTagToGroup', tag.id)"
        >
          <span class="tm-search__item-label">{{ tag.label }}</span>
          <span class="tm-search__item-meta">{{ tag.feed_count }} 篇</span>
        </button>
      </div>
      <div v-else-if="searchQuery.length >= 1" class="tm-search__empty">无结果</div>
    </div>

    <!-- Suggestions list -->
    <div class="tm-suggestions">
      <div
        v-for="sug in group.suggestions"
        :key="sug.id"
        class="tm-suggestion"
        :class="{ 'tm-suggestion--selected': isSugSelected(group.target_tag_id, sug.new_tag_id) }"
        @click="emit('toggleSelect', group.target_tag_id, sug.new_tag_id)"
      >
        <div class="tm-suggestion__main">
          <button
            type="button"
            class="tm-checkbox tm-checkbox--sm"
            :class="{ 'tm-checkbox--checked': isSugSelected(group.target_tag_id, sug.new_tag_id) }"
            @click.stop="emit('toggleSelect', group.target_tag_id, sug.new_tag_id)"
          >
            <Icon v-if="isSugSelected(group.target_tag_id, sug.new_tag_id)" icon="mdi:check" width="12" />
          </button>
          <span class="tm-suggestion__label">{{ sug.new_label }}</span>
          <span class="tm-suggestion__similarity">{{ formatSimilarity(sug.similarity) }}</span>
          <span class="tm-suggestion__articles">{{ sug.new_articles }} 篇</span>
        </div>
        <!-- LLM verdict -->
        <div v-if="parseVerdict(sug.llm_verdict)" class="tm-suggestion__verdict">
          <span
            class="tm-suggestion__badge"
            :class="parseVerdict(sug.llm_verdict)!.should_merge ? 'tm-suggestion__badge--yes' : 'tm-suggestion__badge--no'"
          >
            {{ parseVerdict(sug.llm_verdict)!.should_merge ? '建议合并' : '不建议' }}
          </span>
          <span class="tm-suggestion__arrow">→</span>
          <span class="tm-suggestion__name">{{ parseVerdict(sug.llm_verdict)!.suggested_name }}</span>
        </div>
        <div v-if="parseVerdict(sug.llm_verdict)?.reason" class="tm-suggestion__reason">
          {{ parseVerdict(sug.llm_verdict)!.reason }}
        </div>
        <button
          type="button"
          class="tm-suggestion__remove"
          @click.stop="emit('removeSuggestion', sug, group)"
        >
          <Icon icon="mdi:close" width="12" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tm-group {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 1rem;
  padding: 1rem;
  background: rgba(255, 255, 255, 0.02);
  transition: border-color 0.15s ease;
}
.tm-group:hover {
  border-color: rgba(255, 255, 255, 0.12);
}
.tm-group__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
.tm-group__target {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.tm-group__target-name {
  font-size: 1rem;
  font-weight: 600;
  color: rgba(99, 179, 237, 0.92);
}
.tm-group__target-meta {
  font-size: 0.78rem;
  color: var(--color-text-muted);
}

/* Checkbox */
.tm-checkbox {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 0.3rem;
  border: 1.5px solid var(--color-border-medium);
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;
}
.tm-checkbox--sm {
  width: 1rem;
  height: 1rem;
  border-radius: 0.25rem;
}
.tm-checkbox--checked {
  border-color: rgba(99, 179, 237, 0.7);
  background: rgba(99, 179, 237, 0.2);
  color: rgba(99, 179, 237, 0.95);
}
.tm-checkbox:hover {
  border-color: var(--color-border-medium);
}

/* Buttons */
.tm-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  border-radius: 999px;
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid var(--color-border-medium);
  background: transparent;
  color: var(--color-text-muted);
  padding: 0.5rem 1.1rem;
}
.tm-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.tm-btn:hover:not(:disabled) { border-color: var(--color-border-medium); color: var(--color-text-primary); }
.tm-btn--sm { padding: 0.3rem 0.75rem; font-size: 0.78rem; min-height: 1.75rem; }

/* Search */
.tm-search {
  margin-bottom: 0.5rem;
  padding: 0.6rem;
  border: 1px solid rgba(99, 179, 237, 0.2);
  border-radius: 0.5rem;
  background: rgba(99, 179, 237, 0.04);
}
.tm-search__input-row {
  display: flex;
  gap: 0.4rem;
  margin-bottom: 0.4rem;
}
.tm-search__input {
  flex: 1;
  background: var(--color-input-bg);
  border: 1px solid var(--color-border-medium);
  border-radius: 0.4rem;
  padding: 0.35rem 0.6rem;
  color: var(--color-text-primary);
  transition: border-color 0.15s ease;
}
.tm-search__input:focus { border-color: rgba(99, 179, 237, 0.5); }
.tm-search__input::placeholder { color: rgba(255, 255, 255, 0.3); }
.tm-search__results {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  max-height: 10rem;
  overflow-y: auto;
}
.tm-search__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.35rem 0.6rem;
  border-radius: 0.3rem;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.7);
  font-size: 0.82rem;
  cursor: pointer;
  width: 100%;
  text-align: left;
  transition: all 0.15s ease;
}
.tm-search__item:hover { background: var(--color-bg-hover); color: var(--color-text-primary); }
.tm-search__item-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tm-search__item-meta { font-size: 0.72rem; color: rgba(255, 255, 255, 0.35); flex-shrink: 0; }
.tm-search__loading, .tm-search__empty { padding: 0.4rem; font-size: 0.78rem; color: rgba(255, 255, 255, 0.35); }

/* Suggestions */
.tm-suggestions {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.tm-suggestion {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.5rem 0.6rem;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: background 0.15s ease;
  position: relative;
}
.tm-suggestion:hover { background: var(--color-bg-hover); }
.tm-suggestion--selected {
  background: rgba(99, 179, 237, 0.06);
  border-left: 2px solid rgba(99, 179, 237, 0.5);
  padding-left: calc(0.6rem - 2px);
}
.tm-suggestion__main {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}
.tm-suggestion__label {
  color: rgba(255, 255, 255, 0.7);
  font-size: 0.88rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tm-suggestion__similarity {
  font-size: 0.72rem;
  border-radius: 999px;
  background: rgba(16, 185, 129, 0.18);
  border: 1px solid rgba(16, 185, 129, 0.35);
  padding: 0.1rem 0.45rem;
  color: rgba(110, 231, 183, 0.92);
  flex-shrink: 0;
}
.tm-suggestion__articles {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.35);
  flex-shrink: 0;
}
.tm-suggestion__verdict {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.78rem;
  padding-left: 1.5rem;
}
.tm-suggestion__badge {
  font-size: 0.68rem;
  border-radius: 999px;
  padding: 0.1rem 0.5rem;
  flex-shrink: 0;
}
.tm-suggestion__badge--yes {
  background: rgba(16, 185, 129, 0.15);
  color: rgba(110, 231, 183, 0.9);
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.tm-suggestion__badge--no {
  background: rgba(248, 113, 113, 0.12);
  color: rgba(248, 113, 113, 0.8);
  border: 1px solid rgba(248, 113, 113, 0.25);
}
.tm-suggestion__arrow { color: rgba(240, 138, 75, 0.7); }
.tm-suggestion__name { color: var(--color-text-primary); font-weight: 500; }
.tm-suggestion__reason {
  font-size: 0.78rem;
  color: rgba(255, 255, 255, 0.45);
  padding-left: 1.5rem;
  line-height: 1.4;
  white-space: normal;
}
.tm-suggestion__remove {
  position: absolute;
  top: 0.5rem;
  right: 0.4rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;
  opacity: 0;
}
.tm-suggestion:hover .tm-suggestion__remove { opacity: 1; }
.tm-suggestion__remove:hover { color: rgba(248, 113, 113, 0.9); background: rgba(248, 113, 113, 0.1); }
</style>
