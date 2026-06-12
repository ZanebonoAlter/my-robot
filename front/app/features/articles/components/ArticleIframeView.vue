<script setup lang="ts">
import { Icon } from '@iconify/vue'
import '~/components/article/ArticleContent.css'

defineOptions({ inheritAttrs: false })

interface Props {
  src: string | null
  loading: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'iframe-load': []
  'iframe-error': []
  'open-original': []
}>()
</script>

<template>
  <div class="iframe-mode flex-1 relative">
    <div v-if="loading" class="iframe-loading">
      <div class="text-center">
        <Icon icon="mdi:loading" width="48" height="48" class="animate-spin text-blue-600 mx-auto mb-4" />
        <p class="text-gray-600">正在加载网页...</p>
      </div>
    </div>

    <iframe
      v-if="src"
      :src="src"
      class="w-full h-full border-0"
      title="Article Content"
      sandbox="allow-same-origin allow-scripts allow-forms allow-popups"
      @load="emit('iframe-load')"
      @error="emit('iframe-error')"
    />

    <div v-else class="iframe-error">
      <div class="text-center text-gray-400">
        <Icon icon="mdi:web-off" width="64" height="64" class="mb-4 mx-auto" />
        <p>无法加载网页</p>
        <AppButton variant="primary" class="mt-4" @click="emit('open-original')">在新窗口打开</AppButton>
      </div>
    </div>
  </div>
</template>
