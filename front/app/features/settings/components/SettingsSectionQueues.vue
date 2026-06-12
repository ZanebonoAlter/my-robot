<script setup lang="ts">
import EmbeddingQueuePanel from '~/features/ai/components/EmbeddingQueuePanel.vue'
import TagQueuePanel from '~/features/topic-graph/components/TagQueuePanel.vue'
</script>

<template>
  <div class="space-y-6">
    <EmbeddingQueuePanel />
    <TagQueuePanel />
  </div>
</template>
