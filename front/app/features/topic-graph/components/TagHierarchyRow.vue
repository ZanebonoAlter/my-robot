<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { ref } from 'vue'
import type { TagHierarchyNode } from '~/types/topicTag'

const props = defineProps<{
  node: TagHierarchyNode
  depth: number
  editingId: number | null
  saving: boolean
  watchedTagIds?: Set<number>
}>()

const emit = defineEmits<{
  'start-edit': [node: TagHierarchyNode]
  'cancel-edit': []
  'confirm-edit': []
  'detach': [node: TagHierarchyNode]
  'reassign': [node: TagHierarchyNode]
  'select-node': [node: TagHierarchyNode]
  'update:editing-value': [value: string]
  'toggle-watch': [node: TagHierarchyNode]
}>()

const editingValue = ref('')
const expanded = ref(true)
const clickTimer = ref<ReturnType<typeof setTimeout> | null>(null)

function getCategoryIcon(category: string): string {
  switch (category) {
    case 'event': return 'mdi:calendar-star'
    case 'person': return 'mdi:account'
    default: return 'mdi:tag'
  }
}

function handleLabelClick(node: TagHierarchyNode) {
  if (clickTimer.value) return
  clickTimer.value = setTimeout(() => {
    clickTimer.value = null
    emit('select-node', node)
  }, 250)
}

function handleLabelDblClick(node: TagHierarchyNode) {
  if (clickTimer.value) {
    clearTimeout(clickTimer.value)
    clickTimer.value = null
  }
  editingValue.value = node.label
  emit('start-edit', node)
}

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:editing-value', target.value)
}

function handleDetach(node: TagHierarchyNode) {
  emit('detach', node)
}

function handleReassign(node: TagHierarchyNode) {
  emit('reassign', node)
}

function handleChildStartEdit(node: TagHierarchyNode) { emit('start-edit', node) }
function handleChildCancelEdit() { emit('cancel-edit') }
function handleChildConfirmEdit() { emit('confirm-edit') }
function handleChildDetach(node: TagHierarchyNode) { emit('detach', node) }
function handleChildReassign(node: TagHierarchyNode) { emit('reassign', node) }
function handleChildSelect(node: TagHierarchyNode) { emit('select-node', node) }
function handleChildUpdateEditingValue(val: string) { emit('update:editing-value', val) }
function handleChildToggleWatch(node: TagHierarchyNode) { emit('toggle-watch', node) }

const isWatched = ref(false)

function syncWatchedState() {
  isWatched.value = props.watchedTagIds?.has(props.node.id) ?? false
}

syncWatchedState()

import { watch } from 'vue'
watch(() => [props.watchedTagIds, props.node.id] as const, syncWatchedState)

function handleToggleWatch(e: Event) {
  e.stopPropagation()
  isWatched.value = !isWatched.value
  emit('toggle-watch', props.node)
}
</script>

<template>
  <div :class="{ 'opacity-40': !node.isActive }">
    <div
      class="th-row"
      :style="{ paddingLeft: (depth * 20 + 8) + 'px' }"
    >
      <!-- Expand/collapse toggle -->
      <button
        v-if="node.children.length > 0"
        type="button"
        class="th-toggle"
        @click="expanded = !expanded"
      >
        <Icon :icon="expanded ? 'mdi:chevron-down' : 'mdi:chevron-right'" width="16" />
      </button>
      <span v-else class="th-toggle th-toggle--blank" />

      <!-- Category icon -->
      <Icon :icon="getCategoryIcon(node.category)" width="14" class="th-cat-icon" />

      <!-- Watch heart icon -->
      <button
        type="button"
        class="th-watch-btn"
        :class="{ 'th-watch-btn--active': isWatched }"
        :title="isWatched ? '取消关注' : '关注标签'"
        @click="handleToggleWatch"
      >
        <Icon :icon="isWatched ? 'mdi:heart' : 'mdi:heart-outline'" width="14" />
      </button>

      <!-- Label (edit mode or display mode) -->
      <div v-if="editingId === node.id" class="th-inline-edit">
        <input
          :value="editingValue"
          type="text"
          class="th-inline-input"
          maxlength="160"
          @input="handleInput"
          @keyup.enter="emit('confirm-edit')"
          @keyup.escape="emit('cancel-edit')"
        />
        <button type="button" class="th-action-btn th-action-btn--save" :disabled="saving" @click="emit('confirm-edit')">
          <Icon icon="mdi:check" width="14" />
        </button>
        <button type="button" class="th-action-btn" @click="emit('cancel-edit')">
          <Icon icon="mdi:close" width="14" />
        </button>
      </div>
      <button
        v-else
        type="button"
        class="th-label"
        @click="handleLabelClick(node)"
        @dblclick="handleLabelDblClick(node)"
      >
        {{ node.label }}
      </button>

      <!-- Low quality badge -->
      <span v-if="node.isLowQuality" class="th-badge th-badge--low-quality">低质量</span>

      <!-- Feed count badge -->
      <span v-if="node.feedCount > 0" class="th-badge">{{ node.feedCount }}</span>

      <!-- Detach button for child nodes -->
      <button
        v-if="depth > 0"
        type="button"
        class="th-detach-btn"
        title="从抽象标签分离"
        @click="handleDetach(node)"
      >
        <Icon icon="mdi:link-off" width="12" />
      </button>

      <!-- Reassign button for child nodes -->
      <button
        v-if="depth > 0"
        type="button"
        class="th-reassign-btn"
        title="归类到其他抽象层"
        @click="handleReassign(node)"
      >
        <Icon icon="mdi:arrow-right-bold" width="12" />
      </button>
    </div>

    <!-- Children (recursive) -->
    <div v-if="expanded && node.children.length > 0" class="th-children">
      <TagHierarchyRow
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :editing-id="editingId"
        :saving="saving"
        :watched-tag-ids="watchedTagIds"
        @start-edit="handleChildStartEdit"
        @cancel-edit="handleChildCancelEdit"
        @confirm-edit="handleChildConfirmEdit"
        @detach="handleChildDetach"
        @reassign="handleChildReassign"
        @select-node="handleChildSelect"
        @update:editing-value="handleChildUpdateEditingValue"
        @toggle-watch="handleChildToggleWatch"
      />
    </div>
  </div>
</template>

<style scoped>
.th-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.5rem;
  border-radius: 10px;
  transition: background 0.12s ease;
}
.th-row:hover { background: var(--color-bg-hover); }

.th-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: none;
  background: none;
  color: var(--color-text-muted);
  cursor: pointer;
  border-radius: 4px;
  transition: color 0.12s ease;
}
.th-toggle:hover { color: var(--color-text-secondary); }
.th-toggle--blank { visibility: hidden; }

.th-cat-icon { color: var(--color-text-muted); flex-shrink: 0; }

.th-watch-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 4px;
  background: none;
  color: var(--color-text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.12s ease, background 0.12s ease;
}
.th-watch-btn:hover { color: rgba(239, 68, 68, 0.9); background: rgba(239, 68, 68, 0.1); }
.th-watch-btn--active { color: rgba(239, 68, 68, 0.9); }
.th-watch-btn--active:hover { color: rgba(239, 68, 68, 1); background: rgba(239, 68, 68, 0.15); }

.th-label {
  border: none;
  background: none;
  color: var(--color-text-primary);
  font-size: 0.82rem;
  cursor: pointer;
  text-align: left;
  padding: 0.15rem 0.35rem;
  border-radius: 6px;
  transition: color 0.12s ease, background 0.12s ease;
}
.th-label:hover { color: var(--color-text-primary); background: var(--color-bg-hover); }

.th-inline-edit {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex: 1;
  max-width: 280px;
}

.th-inline-input {
  flex: 1;
  border: 1px solid var(--color-border-medium);
  border-radius: 6px;
  background: var(--color-input-bg);
  color: white;
  font-size: 0.82rem;
  padding: 0.2rem 0.5rem;
  outline: none;
}
.th-inline-input:focus { border-color: var(--color-accent); }

.th-action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
  cursor: pointer;
}
.th-action-btn:hover { background: var(--color-bg-hover); color: var(--color-text-primary); }
.th-action-btn--save { color: var(--color-success); }

.th-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--color-bg-hover);
  color: var(--color-text-muted);
  font-size: 0.65rem;
  font-weight: 500;
}
.th-badge--low-quality {
  background: var(--color-accent-subtle);
  color: var(--color-text-primary);
  border: 1px solid rgba(240, 138, 75, 0.2);
}

.th-detach-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: none;
  background: var(--color-bg-hover);
  color: var(--color-text-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease, color 0.12s ease;
}
.th-row:hover .th-detach-btn { opacity: 1; }
.th-detach-btn:hover { color: var(--color-accent); background: var(--color-accent-subtle); }

.th-reassign-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--color-text-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease, color 0.12s ease;
}
.th-row:hover .th-reassign-btn { opacity: 1; }
.th-reassign-btn:hover { color: var(--color-tag-keyword); background: var(--color-tag-keyword-bg); }

.th-children {
  border-left: 1px solid var(--color-border-subtle);
  margin-left: 18px;
}
</style>
