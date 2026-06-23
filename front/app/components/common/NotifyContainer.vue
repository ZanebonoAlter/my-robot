<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useNotify, type Toast } from '~/composables/useNotify'

const { toasts, dismiss } = useNotify()

const iconMap: Record<string, string> = {
  success: 'mdi:check-circle',
  error: 'mdi:alert-circle',
  warn: 'mdi:alert',
}

function iconFor(toast: Toast): string {
  return iconMap[toast.type] ?? 'mdi:information'
}

const colorMap: Record<string, string> = {
  success: 'border-emerald-500 bg-emerald-50 text-emerald-800',
  error: 'border-red-500 bg-red-50 text-red-800',
  warn: 'border-amber-500 bg-amber-50 text-amber-800',
}
</script>

<template>
  <div class="fixed top-4 right-4 z-[9999] flex flex-col gap-2 max-w-sm pointer-events-none">
    <TransitionGroup name="toast">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        :class="['pointer-events-auto flex items-start gap-2 px-4 py-3 rounded-lg border shadow-lg', colorMap[toast.type]]"
      >
        <Icon :icon="iconFor(toast)" width="20" height="20" class="mt-0.5 shrink-0" />
        <span class="text-sm flex-1">{{ toast.message }}</span>
        <button class="text-current opacity-60 hover:opacity-100 shrink-0" @click="dismiss(toast.id)">
          <Icon icon="mdi:close" width="16" height="16" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(30px);
}
</style>
