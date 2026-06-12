<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useTagMergePreview } from '~/features/topic-graph/composables/useTagMergePreview'
import TagMergeGroup from './TagMergeGroup.vue'

interface Props {
  visible: boolean
  scopeCategoryId?: string | null
  scopeFeedId?: string | null
  standalone?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  scopeCategoryId: null,
  scopeFeedId: null,
  standalone: true,
})
const emit = defineEmits<{ close: []; merged: [] }>()

const {
  loading, groups, error, evaluating, evalProgress, scanning, scanProgress,
  selectedKeys, merging, mergedCount, mergeProgress,
  searchingGroupId, searchQuery, searchResults, searchLoading,
  selectedCount, hasMergeableSuggestions,
  toggleSelect, selectAllInGroup, deselectAllInGroup,
  selectAllMergeable, clearSelection,
  triggerEvaluate, cancelEvaluate,
  triggerFullScan, cancelScan, mergeSelected,
  removeSuggestion, openSearch, closeSearch, onSearchInput, addTagToGroup,
  parseVerdict, formatSimilarity,
  handleClose,
} = useTagMergePreview(() => props.visible, { merged: () => emit('merged'), close: () => emit('close') })
</script>

<template>
  <Teleport to="body" :disabled="!props.standalone">
    <div v-if="visible" :class="props.standalone ? 'tag-merge-overlay' : 'tag-merge-inline'" @click.self="props.standalone ? handleClose() : undefined">
      <div :class="props.standalone ? 'tag-merge-modal' : 'tag-merge-inline__content'">
        <!-- Header -->
        <header class="tm-header">
          <div>
            <h2 class="text-lg font-semibold" style="color: var(--color-text-primary)">
              标签合并预览
              <span v-if="groups.length" class="ml-2 text-sm font-normal text-[var(--color-text-muted)]">
                ({{ groups.length }} 组)
              </span>
            </h2>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="tm-btn tm-btn--accent"
              :class="{ 'tm-btn--loading': evaluating }"
              :disabled="evaluating || groups.length === 0 || scanning"
              @click="evaluating ? cancelEvaluate() : triggerEvaluate()"
            >
              <Icon v-if="evaluating" icon="mdi:loading" width="16" class="animate-spin" />
              <Icon v-else icon="mdi:robot-outline" width="16" />
              <span>{{ evaluating ? '评估中...' : 'AI 评估' }}</span>
            </button>
            <button
              type="button"
              class="tm-btn tm-btn--accent"
              :disabled="!hasMergeableSuggestions"
              @click="selectAllMergeable()"
            >
              <Icon icon="mdi:check-all" width="16" />
              <span>按 AI 建议全选</span>
            </button>
            <button
              v-if="!scanning"
              type="button"
              class="tm-btn"
              @click="triggerFullScan"
            >
              <Icon icon="mdi:radar" width="16" />
              <span>全量扫描</span>
            </button>
            <button type="button" class="tm-close-btn" aria-label="关闭" @click="handleClose">
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
        </header>

        <!-- Batch action bar -->
        <div v-if="selectedCount > 0" class="tm-batch-bar">
          <span class="tm-batch-bar__count">已选 {{ selectedCount }} 项</span>
          <div class="flex items-center gap-2">
            <button type="button" class="tm-btn tm-btn--sm" @click="selectAllMergeable">
              <Icon icon="mdi:check-all" width="14" />
              <span>全选可合并</span>
            </button>
            <button type="button" class="tm-btn tm-btn--sm" @click="clearSelection">
              <span>清空</span>
            </button>
            <button
              type="button"
              class="tm-btn tm-btn--primary tm-btn--sm"
              :disabled="merging"
              @click="mergeSelected"
            >
              <Icon v-if="merging" icon="mdi:loading" width="14" class="animate-spin" />
              <Icon v-else icon="mdi:call-merge" width="14" />
              <span>{{ mergeProgress ? `${mergeProgress.done}/${mergeProgress.total} 已合并` : `合并选中 (${selectedCount})` }}</span>
            </button>
          </div>
        </div>

        <!-- Evaluate progress -->
        <div v-if="evaluating" class="tm-progress">
          <div class="tm-progress__bar">
            <div
              class="tm-progress__fill"
              :style="{ width: `${evalProgress?.total_groups ? (evalProgress.completed / evalProgress.total_groups * 100) : 0}%` }"
            />
          </div>
          <div class="tm-progress__info">
            <span>{{ evalProgress ? `${evalProgress.completed}/${evalProgress.total_groups} 组` : '正在启动...' }}</span>
            <span v-if="evalProgress?.current_target">正在评估「{{ evalProgress.current_target }}」</span>
          </div>
          <button type="button" class="tm-btn tm-btn--sm" @click="cancelEvaluate">
            <Icon icon="mdi:close" width="14" />
          </button>
        </div>

        <!-- Scan progress -->
        <div v-if="scanning && scanProgress" class="tm-progress">
          <div class="tm-progress__bar">
            <div
              class="tm-progress__fill"
              :style="{ width: `${scanProgress.total ? (scanProgress.scanned / scanProgress.total * 100) : 0}%` }"
            />
          </div>
          <div class="tm-progress__info">
            <span>{{ scanProgress.scanned }}/{{ scanProgress.total }} 标签</span>
            <span>发现 {{ scanProgress.new_suggestions }} 个新建议</span>
          </div>
          <button type="button" class="tm-btn tm-btn--sm" @click="cancelScan">
            <Icon icon="mdi:close" width="14" />
          </button>
        </div>

        <!-- Error -->
        <div v-if="error" class="tm-error">
          <Icon icon="mdi:alert-circle-outline" width="16" />
          <span>{{ error }}</span>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="tm-loading">
          <Icon icon="mdi:loading" width="32" class="animate-spin text-[var(--color-accent)]" />
          <p class="mt-4 text-sm text-[var(--color-text-secondary)]">加载中...</p>
        </div>

        <!-- Empty -->
        <div v-else-if="groups.length === 0 && !error" class="tm-empty">
          <Icon icon="mdi:tag-check-outline" width="32" class="text-[var(--color-text-muted)]" />
          <p class="mt-3 text-sm text-[var(--color-text-muted)]">没有发现相似标签</p>
          <p class="mt-1 text-xs text-[var(--color-text-muted)]">试试「全量扫描」查找更多相似对</p>
        </div>

        <!-- Groups -->
        <div v-else class="tm-groups">
          <TagMergeGroup
            v-for="group in groups"
            :key="group.target_tag_id"
            :group="group"
            :selected-keys="selectedKeys"
            :searching-group-id="searchingGroupId"
            :search-query="searchQuery"
            :search-results="searchResults"
            :search-loading="searchLoading"
            :parse-verdict="parseVerdict"
            :format-similarity="formatSimilarity"
            @toggle-select="toggleSelect"
            @select-all-in-group="selectAllInGroup"
            @deselect-all-in-group="deselectAllInGroup"
            @open-search="openSearch"
            @close-search="closeSearch"
            @search-input="onSearchInput"
            @add-tag-to-group="addTagToGroup"
            @remove-suggestion="removeSuggestion"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* --- Layout --- */
.tag-merge-overlay {
  position: fixed;
  inset: 0;
  z-index: 78;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: var(--color-bg-overlay);
  backdrop-filter: blur(10px);
}
.tag-merge-modal {
  width: min(48rem, 100%);
  max-height: calc(100vh - 2rem);
  overflow-y: auto;
  border-radius: 1.75rem;
  background: linear-gradient(180deg, var(--color-bg-elevated), var(--color-bg-sunken));
  box-shadow: 0 30px 100px rgba(0, 0, 0, 0.32);
  padding: 1.5rem;
}
.tag-merge-inline {
  width: 100%;
}
.tag-merge-inline__content {
  width: 100%;
  max-height: 60vh;
  overflow-y: auto;
  border-radius: 1rem;
  background: linear-gradient(180deg, var(--color-bg-elevated), var(--color-bg-sunken));
  border: 1px solid var(--color-border-subtle);
  padding: 1.5rem;
}

/* --- Header --- */
.tm-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
.tm-close-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.75rem;
  min-width: 2.75rem;
  border: 1px solid var(--color-border-subtle);
  border-radius: 999px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.15s ease;
}
.tm-close-btn:hover {
  border-color: var(--color-border-medium);
  color: var(--color-text-primary);
}

/* --- Batch bar --- */
.tm-batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.6rem 1rem;
  margin-bottom: 0.75rem;
  border-radius: 0.75rem;
  background: var(--color-link-subtle);
  border: 1px solid var(--color-link-border);
}
.tm-batch-bar__count {
  font-size: 0.85rem;
  color: var(--color-link);
  font-weight: 500;
}

/* --- Buttons --- */
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
  color: var(--color-text-secondary);
  padding: 0.5rem 1.1rem;
}
.tm-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.tm-btn:hover:not(:disabled) { border-color: var(--color-border-medium); color: var(--color-text-primary); }
.tm-btn--primary {
  background: linear-gradient(135deg, var(--color-accent), var(--color-accent-hover));
  color: rgba(255, 245, 235, 0.95);
  border-color: rgba(240, 138, 75, 0.4);
}
.tm-btn--primary:hover:not(:disabled) {
  background: linear-gradient(135deg, var(--color-accent-hover), var(--color-accent-hover));
  box-shadow: 0 6px 20px rgba(240, 138, 75, 0.25);
}
.tm-btn--accent {
  border-color: var(--color-link-border);
  color: var(--color-link);
}
.tm-btn--accent:hover:not(:disabled) {
  border-color: var(--color-link-border);
  color: var(--color-link);
  background: var(--color-link-subtle);
}
.tm-btn--sm { padding: 0.3rem 0.75rem; font-size: 0.78rem; min-height: 1.75rem; }

/* --- Progress --- */
.tm-progress {
  padding: 12px 16px;
  margin-bottom: 12px;
  background: var(--color-bg-hover);
  border-radius: 8px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.tm-progress__bar {
  flex: 1;
  height: 4px;
  background: var(--color-border-medium);
  border-radius: 2px;
  overflow: hidden;
}
.tm-progress__fill {
  height: 100%;
  background: var(--color-accent);
  transition: width 0.3s ease;
}
.tm-progress__info {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

/* --- States --- */
.tm-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 3rem 1rem;
}
.tm-error {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-accent);
  background: var(--color-accent-subtle);
  padding: 0.75rem 1rem;
  color: var(--color-text-primary);
  font-size: 0.85rem;
  margin-bottom: 1rem;
}
.tm-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 2.5rem 1rem;
}

/* --- Groups --- */
.tm-groups {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
</style>
