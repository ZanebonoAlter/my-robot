<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useTheme } from '~/composables/useTheme'

const { toggleTheme, isDark } = useTheme()

interface Props {
  /** 按钮变体 */
  variant?: 'icon' | 'button'
  /** 按钮尺寸 */
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'icon',
  size: 'md',
})

const iconSize = computed(() => {
  return props.size === 'sm' ? 16 : 20
})
</script>

<template>
  <button
    :class="[
      'theme-toggle',
      `theme-toggle--${variant}`,
      `theme-toggle--${size}`,
    ]"
    :title="isDark ? '切换为浅色模式' : '切换为深色模式'"
    :aria-label="isDark ? '切换为浅色模式' : '切换为深色模式'"
    @click="toggleTheme"
  >
    <Icon
      :icon="isDark ? 'mdi:white-balance-sunny' : 'mdi:weather-night'"
      :width="iconSize"
      :height="iconSize"
    />
    <span v-if="variant === 'button'" class="theme-toggle__label">
      {{ isDark ? '浅色' : '深色' }}
    </span>
  </button>
</template>

<style scoped>
.theme-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: none;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.theme-toggle:hover {
  color: var(--color-text-primary);
  background: var(--color-bg-hover);
}

.theme-toggle--icon {
  border-radius: 8px;
  padding: 0.5rem;
}

.theme-toggle--icon.theme-toggle--sm {
  padding: 0.35rem;
  border-radius: 6px;
}

.theme-toggle--button {
  gap: 0.4rem;
  border: 1px solid var(--color-border-medium);
  border-radius: 999px;
  padding: 0.4rem 0.75rem;
  font-size: 0.75rem;
}

.theme-toggle--button:hover {
  border-color: var(--color-border-strong);
}

.theme-toggle__label {
  white-space: nowrap;
}
</style>
