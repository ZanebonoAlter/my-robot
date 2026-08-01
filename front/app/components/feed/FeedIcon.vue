<script setup lang="ts">
import { ref } from 'vue'
import { Icon } from '@iconify/vue'
import { getApiOrigin } from '~/utils/api'

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

// Same-origin relative path served by the backend (e.g. /icons/feeds/42.png).
// Resolve it against the API origin (dev: http://localhost:5000, prod: same
// origin as the page).
const isLocalPath = computed(() => Boolean(props.icon?.startsWith('/')))

const imgSrc = computed(() => {
  if (!props.icon) return ''
  return isLocalPath.value ? `${getApiOrigin()}${props.icon}` : props.icon
})

const iconSize = computed(() => props.size || 20)

// Reset the failure flag when the icon prop changes (e.g. feed switched).
watch(() => props.icon, () => {
  imgFailed.value = false
})
</script>

<template>
  <img
    v-if="(isUrl || isLocalPath) && !imgFailed"
    :src="imgSrc"
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
