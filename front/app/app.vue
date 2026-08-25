<script setup lang="ts">
import { Icon } from '@iconify/vue'

// 初始化主题系统
useTheme()

const apiStore = useApiStore()
const loading = ref(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    await apiStore.initialize()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载数据失败'
    console.error('初始化错误:', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="h-screen flex items-center justify-center">
    <div class="text-center">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin mx-auto mb-4" style="color: var(--color-text-secondary)" />
      <p style="color: var(--color-text-secondary)">正在加载...</p>
    </div>
  </div>

  <div v-else-if="error" class="h-screen flex items-center justify-center">
    <div class="text-center max-w-md">
      <Icon icon="mdi:alert-circle" width="48" height="48" class="text-[var(--color-error)] mx-auto mb-4" />
      <h2 class="text-xl font-bold mb-2" style="color: var(--color-text-primary)">加载失败</h2>
      <p class="mb-4" style="color: var(--color-text-secondary)">{{ error }}</p>
      <AppButton variant="primary" @click="$router.go(0)">
        重新加载
      </AppButton>
    </div>
  </div>

  <NuxtPage v-else />

  <!-- AI 模型未就绪全局提示（用户意图运行但健康门未通过时） -->
  <AiHealthBanner />

  <!-- 全局 Toast 通知 -->
  <NotifyContainer />
</template>
