<script setup lang="ts">
import { Icon } from '@iconify/vue'
import ReadingPreferencesPanel from '~/components/dialog/ReadingPreferencesPanel.vue'
import { useOnboarding } from '~/composables/useOnboarding'

const { resetOnboarding } = useOnboarding()
</script>

<template>
  <div class="space-y-6">
    <ReadingPreferencesPanel />

    <!-- 引导 -->
    <div class="rounded-xl p-5" style="background: var(--color-bg-sunken)">
      <div class="flex items-center justify-between gap-4 flex-wrap">
        <div class="flex items-start gap-3">
          <Icon icon="mdi:compass-outline" width="22" height="22" style="color: var(--color-link)" />
          <div>
            <p class="text-sm font-medium" style="color: var(--color-text-primary)">新手引导</p>
            <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">重新播放首次使用的分步引导教程。</p>
          </div>
        </div>
        <button
          type="button"
          class="px-3 py-1.5 text-sm rounded-md transition-colors"
          style="background: var(--color-bg-elevated); box-shadow: var(--shadow-subtle); color: var(--color-text-primary); font-weight: 500"
          @click="resetOnboarding"
        >
          重播引导
        </button>
      </div>
    </div>
  </div>
</template>
