<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { SectionKey } from './SettingsWorkspace.vue'

interface SectionMeta {
  key: SectionKey
  label: string
  icon: string
}

defineProps<{
  sections: SectionMeta[]
  activeSection: SectionKey
  mobileOpen: boolean
}>()

const emit = defineEmits<{
  select: [key: SectionKey]
  close: []
}>()
</script>

<template>
  <!-- Desktop sidebar -->
  <aside class="settings-sidebar">
    <nav class="settings-sidebar__nav" data-onboarding="settings-nav">
      <button
        v-for="section in sections"
        :key="section.key"
        class="settings-sidebar__item"
        :data-onboarding="`settings-nav-${section.key}`"
        :class="{ 'settings-sidebar__item--active': activeSection === section.key }"
        @click="emit('select', section.key)"
      >
        <Icon :icon="section.icon" width="18" height="18" />
        <span>{{ section.label }}</span>
      </button>
    </nav>
  </aside>

  <!-- Mobile overlay + drawer -->
  <Teleport to="body">
    <Transition name="sidebar-overlay">
      <div
        v-if="mobileOpen"
        class="settings-sidebar-overlay"
        @click="emit('close')"
      />
    </Transition>
    <Transition name="sidebar-drawer">
      <aside v-if="mobileOpen" class="settings-sidebar-drawer">
        <nav class="settings-sidebar__nav">
          <button
            v-for="section in sections"
            :key="section.key"
            class="settings-sidebar__item"
            :class="{ 'settings-sidebar__item--active': activeSection === section.key }"
            @click="emit('select', section.key)"
          >
            <Icon :icon="section.icon" width="18" height="18" />
            <span>{{ section.label }}</span>
          </button>
        </nav>
      </aside>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* Desktop sidebar */
.settings-sidebar {
  width: 200px;
  flex-shrink: 0;
  border-right: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
  overflow-y: auto;
  padding: 12px 8px;
}

.settings-sidebar__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.settings-sidebar__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border-radius: 8px;
  transition: background 0.15s, color 0.15s;
  text-align: left;
  white-space: nowrap;
}

.settings-sidebar__item:hover {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
}

.settings-sidebar__item--active {
  background: var(--color-accent-subtle);
  color: var(--color-accent);
  font-weight: 600;
}

.settings-sidebar__item--active:hover {
  background: var(--color-accent-subtle);
  color: var(--color-accent);
}

/* Mobile overlay */
.settings-sidebar-overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: rgba(0, 0, 0, 0.3);
}

/* Mobile drawer */
.settings-sidebar-drawer {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 101;
  width: 240px;
  background: var(--color-bg-elevated);
  border-right: 1px solid var(--color-border-subtle);
  padding: 16px 12px;
  overflow-y: auto;
}

/* Transitions */
.sidebar-overlay-enter-active,
.sidebar-overlay-leave-active {
  transition: opacity 0.2s ease;
}
.sidebar-overlay-enter-from,
.sidebar-overlay-leave-to {
  opacity: 0;
}

.sidebar-drawer-enter-active,
.sidebar-drawer-leave-active {
  transition: transform 0.2s ease;
}
.sidebar-drawer-enter-from,
.sidebar-drawer-leave-to {
  transform: translateX(-100%);
}

/* Hide desktop sidebar on narrow screens */
@media (max-width: 768px) {
  .settings-sidebar {
    display: none;
  }
}
</style>
