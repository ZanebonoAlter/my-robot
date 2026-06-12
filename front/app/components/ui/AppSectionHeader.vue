<script setup lang="ts">
import type { Component } from 'vue'

interface Props {
  title: string
  description?: string
  icon?: Component
}

defineProps<Props>()
</script>

<template>
  <div class="app-section-header">
    <div v-if="icon" class="app-section-header__icon">
      <component :is="icon" />
    </div>
    <h3 class="app-section-header__title">{{ title }}</h3>
    <p v-if="description" class="app-section-header__desc">{{ description }}</p>
  </div>
</template>

<style scoped>
.app-section-header {
  margin-bottom: 16px;
}

.app-section-header__icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--color-accent-subtle);
  color: var(--color-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
}

.app-section-header__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

.app-section-header__desc {
  font-size: 13px;
  color: var(--color-text-muted);
  margin-top: 2px;
}
</style>
