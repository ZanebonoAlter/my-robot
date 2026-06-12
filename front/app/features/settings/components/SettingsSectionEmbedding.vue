<script setup lang="ts">
import { Icon } from '@iconify/vue'
import EmbeddingConfigPanel from '~/features/ai/components/EmbeddingConfigPanel.vue'
import { useAIRouterSettings } from '~/features/ai/composables/useAIRouterSettings'

const { embeddingThreshold, savingThreshold, saveThreshold, loading } = useAIRouterSettings()
</script>

<template>
  <div class="space-y-5">
    <!-- Embedding model config -->
    <EmbeddingConfigPanel />

    <!-- Board match threshold -->
    <div class="rounded-xl border overflow-hidden" style="border-color: var(--color-border-subtle); background: var(--color-bg-elevated)">
      <div class="px-5 py-3.5 border-b flex items-center justify-between" style="border-color: var(--color-border-subtle)">
        <div>
          <h3 class="text-sm font-semibold" style="color: var(--color-text-primary)">板块匹配阈值</h3>
          <p class="text-xs mt-0.5" style="color: var(--color-text-muted)">Embedding 相似度阈值，越低标签越容易匹配到板块</p>
        </div>
        <button
          class="px-3 py-1.5 text-xs font-medium text-white rounded-lg transition-colors disabled:opacity-50"
          style="background: var(--color-accent)"
          :disabled="savingThreshold"
          @click="saveThreshold"
        >
          <Icon v-if="savingThreshold" icon="mdi:loading" width="12" height="12" class="animate-spin inline-block mr-1" />
          保存
        </button>
      </div>
      <div class="p-5">
        <div class="flex items-center gap-4">
          <input
            v-model.number="embeddingThreshold"
            type="range"
            min="0.1"
            max="0.95"
            step="0.05"
            class="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
            :style="{
              background: `linear-gradient(to right, var(--color-accent) 0%, var(--color-accent) ${(embeddingThreshold - 0.1) / 0.85 * 100}%, var(--color-bg-sunken) ${(embeddingThreshold - 0.1) / 0.85 * 100}%, var(--color-bg-sunken) 100%)`
            }"
          />
          <span class="text-base font-bold w-12 text-right tabular-nums" style="color: var(--color-accent)">
            {{ embeddingThreshold.toFixed(2) }}
          </span>
        </div>
        <div class="flex justify-between mt-1">
          <span class="text-[10px]" style="color: var(--color-text-muted)">0.10 宽松</span>
          <span class="text-[10px]" style="color: var(--color-text-muted)">0.95 严格</span>
        </div>
      </div>
    </div>
  </div>
</template>
