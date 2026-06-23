<script setup lang="ts">
interface Props {
  modelValue?: boolean
  disabled?: boolean
  label?: string
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function toggle() {
  if (!props.disabled) {
    emit('update:modelValue', !props.modelValue)
  }
}
</script>

<template>
  <label class="app-toggle" :class="{ 'is-active': modelValue, 'is-disabled': disabled }">
    <div class="app-toggle__track" @click="toggle">
      <div class="app-toggle__thumb" />
    </div>
    <span v-if="label" class="app-toggle__label">{{ label }}</span>
  </label>
</template>

<style scoped>
.app-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.app-toggle.is-disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.app-toggle__track {
  --toggle-width: 36px;
  --toggle-height: 20px;
  --toggle-radius: 10px;
  --toggle-track: var(--color-border-medium);
  --toggle-track-active: var(--color-accent);

  width: var(--toggle-width);
  height: var(--toggle-height);
  border-radius: var(--toggle-radius);
  background: var(--toggle-track);
  position: relative;
  transition: background 0.2s;
}

.app-toggle.is-active .app-toggle__track {
  background: var(--toggle-track-active);
}

.app-toggle__thumb {
  --toggle-thumb-size: 16px;
  width: var(--toggle-thumb-size);
  height: var(--toggle-thumb-size);
  border-radius: 50%;
  background: #fff;
  position: absolute;
  top: 2px;
  left: 2px;
  transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.app-toggle.is-active .app-toggle__thumb {
  transform: translateX(16px);
}

.app-toggle__label {
  font-size: 14px;
  color: var(--color-text-secondary);
}
</style>
