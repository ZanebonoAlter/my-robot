<script setup lang="ts">
interface Props {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
  type?: 'button' | 'submit' | 'reset'
}

withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  disabled: false,
  loading: false,
  type: 'button',
})
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    class="app-button"
    :class="[
      `app-button--${variant}`,
      `app-button--${size}`,
    ]"
  >
    <span v-if="loading" class="app-button__spinner" />
    <slot />
  </button>
</template>

<style scoped>
.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: none;
  cursor: pointer;
  font-weight: 500;
  border-radius: 8px;
  transition: background 0.15s, color 0.15s, opacity 0.15s;
  font-family: inherit;

  --btn-bg: var(--color-accent);
  --btn-bg-hover: var(--color-accent-hover);
  --btn-text: var(--color-text-inverted);

  background: var(--btn-bg);
  color: var(--btn-text);
}

.app-button--secondary {
  --btn-bg: var(--color-bg-hover);
  --btn-bg-hover: var(--color-bg-active);
  --btn-text: var(--color-text-primary);
}
.app-button--ghost {
  --btn-bg: transparent;
  --btn-bg-hover: var(--color-bg-hover);
  --btn-text: var(--color-text-secondary);
}
.app-button--danger {
  --btn-bg: var(--color-error);
  --btn-bg-hover: var(--color-error);
  --btn-text: #fff;
}

.app-button--sm  { padding: 4px 10px; font-size: 13px; }
.app-button--md  { padding: 6px 14px; font-size: 14px; }
.app-button--lg  { padding: 8px 18px; font-size: 15px; }

.app-button:hover   { background: var(--btn-bg-hover); }
.app-button:active  { opacity: 0.85; }
.app-button:disabled { opacity: 0.4; cursor: not-allowed; }

.app-button__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid transparent;
  border-top-color: currentColor;
  border-radius: 50%;
  animation: btn-spin 0.6s linear infinite;
}

@keyframes btn-spin {
  to { transform: rotate(360deg); }
}
</style>
