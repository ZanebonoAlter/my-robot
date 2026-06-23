<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
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

const show = computed({
  get: () => props.editingBoard,
  set: (val: boolean) => { if (!val) emit('close') }
})
</script>

<template>
  <AppDialog v-model="show" title="编辑板块" width="480px">
    <form class="board-form" @submit.prevent="emit('save')">
      <label class="form-field">
        <span class="form-label">名称 <span class="required-mark">*</span></span>
        <AppInput
          :model-value="editLabel"
          placeholder="板块名称"
          @update:model-value="emit('update:edit-label', $event as string)"
        />
      </label>
      <label class="form-field">
        <span class="form-label">描述</span>
        <textarea
          :value="editDescription"
          class="native-textarea"
          placeholder="可选描述"
          maxlength="500"
          rows="4"
          @input="emit('update:edit-description', ($event.target as HTMLTextAreaElement).value)"
        />
      </label>
      <p v-if="editError" class="error-text">{{ editError }}</p>
    </form>

    <template #footer>
      <AppButton variant="ghost" size="sm" :disabled="editSaving" @click="emit('close')">取消</AppButton>
      <AppButton
        variant="primary"
        size="sm"
        :disabled="editSaving || !editLabel.trim()"
        @click="emit('save')"
      >
        {{ editSaving ? '保存中...' : '保存' }}
      </AppButton>
    </template>
  </AppDialog>
</template>

<style scoped>
.board-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.form-label {
  font-size: 0.72rem;
  color: var(--color-text-secondary);
  letter-spacing: 0.02em;
}

.required-mark {
  color: var(--color-accent);
}

.native-textarea {
  width: 100%;
  border: 1px solid var(--color-input-border);
  border-radius: 8px;
  background: var(--color-input-bg);
  color: var(--color-text-primary);
  font-size: 0.82rem;
  padding: 0.55rem 0.85rem;
  outline: none;
  resize: vertical;
  box-sizing: border-box;
  font-family: inherit;
}

.native-textarea::placeholder {
  color: var(--color-text-muted);
}

.native-textarea:focus {
  border-color: var(--color-input-focus);
}

.error-text {
  font-size: 0.72rem;
  color: var(--color-accent);
}
</style>
