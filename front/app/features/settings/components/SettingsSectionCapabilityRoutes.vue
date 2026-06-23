<script setup lang="ts">
import { provide, reactive } from 'vue'
import { useAIRouterSettings } from '~/features/ai/composables/useAIRouterSettings'
import AIRouterCapabilityRoutes from '~/features/ai/components/AIRouterCapabilityRoutes.vue'

const ctx = useAIRouterSettings()
provide('ai-router-ctx', reactive(ctx))
</script>

<template>
  <div>
    <div v-if="ctx.loading.value" class="py-12 flex justify-center" style="color: var(--color-text-muted)">
      加载中…
    </div>
    <AIRouterCapabilityRoutes v-else />
  </div>
</template>
