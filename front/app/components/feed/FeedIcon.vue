<script setup lang="ts">
import { ref } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps<{
  icon?: string
  color?: string
  size?: number
  feedId?: string
  articleLink?: string
}>()

// favicon retrieval is owned by the backend icon state machine. The component's
// only job here is graceful degradation: render the icon value as-is, and when
// an image URL fails to load, fall back to the mdi:rss placeholder rather than
// leaving a blank gap (the old display:none behavior).
const imgFailed = ref(false)

const isUrl = computed(() =>
  Boolean(props.icon && (props.icon.startsWith('http://') || props.icon.startsWith('https://'))),
)

const iconSize = computed(() => props.size || 20)

// Reset the failure flag when the icon prop changes (e.g. feed switched).
watch(() => props.icon, () => {
  imgFailed.value = false
})
</script>

<template>
  <img
    v-if="isUrl && !imgFailed"
    :src="icon"
    :width="iconSize"
    :height="iconSize"
    class="object-contain"
    :style="{ color }"
    @error="imgFailed = true"
  >
  <Icon
    v-else
    :icon="icon || 'mdi:rss'"
    :width="iconSize"
    :height="iconSize"
    :style="{ color }"
  />
</template>
