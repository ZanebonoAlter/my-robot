<script setup lang="ts">
import { Icon } from '@iconify/vue'

defineProps<{
  editingBoard: boolean
  editLabel: string
  editDescription: string
  editSaving: boolean
  editError: string | null
}>()

const emit = defineEmits<{
  close: []
  save: []
  'update:edit-label': [val: string]
  'update:edit-description': [val: string]
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="editingBoard" class="board-edit-overlay" @click.self="emit('close')">
      <form class="board-edit-card" @submit.prevent="emit('save')">
        <div class="board-edit-header">
          <h3 class="board-edit-title">编辑板块</h3>
          <button type="button" class="board-edit-close" :disabled="editSaving" @click="emit('close')">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="board-edit-body">
          <label class="board-edit-field">
            <span class="board-edit-label">名称 <span class="board-edit-required">*</span></span>
            <input :value="editLabel" @input="emit('update:edit-label', ($event.target as HTMLInputElement).value)" type="text" class="board-edit-input" placeholder="板块名称" maxlength="100" autofocus />
          </label>
          <label class="board-edit-field">
            <span class="board-edit-label">描述</span>
            <textarea :value="editDescription" @input="emit('update:edit-description', ($event.target as HTMLTextAreaElement).value)" class="board-edit-textarea" placeholder="可选描述" maxlength="500" rows="4" />
          </label>
          <p v-if="editError" class="board-edit-error">{{ editError }}</p>
        </div>
        <div class="board-edit-footer">
          <button type="button" class="board-edit-btn board-edit-btn--ghost" :disabled="editSaving" @click="emit('close')">取消</button>
          <button type="submit" class="board-edit-btn board-edit-btn--primary" :disabled="editSaving || !editLabel.trim()">
            {{ editSaving ? '保存中...' : '保存' }}
          </button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.board-edit-overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: center; justify-content: center; background: rgba(8, 12, 18, 0.75); backdrop-filter: blur(8px); }
.board-edit-card { width: min(480px, 90%); display: flex; flex-direction: column; border-radius: 1.25rem; border: 1px solid rgba(255, 255, 255, 0.1); background: rgba(17, 27, 38, 0.98); padding: 1.5rem; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5); }
.board-edit-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.25rem; }
.board-edit-title { font-size: 0.95rem; font-weight: 600; color: rgba(255, 255, 255, 0.9); }
.board-edit-close { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; border: none; border-radius: 8px; background: none; color: rgba(255, 255, 255, 0.4); cursor: pointer; transition: all 0.12s ease; }
.board-edit-close:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); color: rgba(255, 255, 255, 0.7); }
.board-edit-body { display: flex; flex-direction: column; gap: 1rem; }
.board-edit-field { display: flex; flex-direction: column; gap: 0.35rem; }
.board-edit-label { font-size: 0.72rem; color: rgba(255, 255, 255, 0.5); letter-spacing: 0.02em; }
.board-edit-required, .board-edit-error { color: rgba(240, 138, 75, 0.8); }
.board-edit-input, .board-edit-textarea { width: 100%; border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 10px; background: rgba(0, 0, 0, 0.25); color: rgba(255, 255, 255, 0.88); font-size: 0.82rem; padding: 0.55rem 0.85rem; outline: none; transition: border-color 0.12s ease; box-sizing: border-box; }
.board-edit-textarea { resize: vertical; }
.board-edit-input::placeholder, .board-edit-textarea::placeholder { color: rgba(255, 255, 255, 0.2); }
.board-edit-input:focus, .board-edit-textarea:focus { border-color: rgba(240, 138, 75, 0.45); }
.board-edit-error { font-size: 0.72rem; }
.board-edit-footer { display: flex; gap: 0.5rem; justify-content: flex-end; margin-top: 1.25rem; }
.board-edit-btn { border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 10px; background: none; color: rgba(255, 255, 255, 0.7); font-size: 0.82rem; padding: 0.45rem 1.1rem; cursor: pointer; transition: all 0.12s ease; }
.board-edit-btn--ghost:hover:not(:disabled) { background: rgba(255, 255, 255, 0.06); }
.board-edit-btn--primary { border-color: rgba(240, 138, 75, 0.4); color: rgba(255, 220, 200, 0.9); background: rgba(240, 138, 75, 0.12); }
.board-edit-btn--primary:hover:not(:disabled) { background: rgba(240, 138, 75, 0.2); border-color: rgba(240, 138, 75, 0.6); }
.board-edit-btn:disabled, .board-edit-close:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
