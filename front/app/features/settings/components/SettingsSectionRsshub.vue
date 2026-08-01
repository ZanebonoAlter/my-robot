<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted } from 'vue'
import { useRsshubConfig } from '~/composables/useRsshubConfig'

const {
  rsshubBaseUrl, rsshubDefault, rsshubConfigured, rsshubLoading,
  rsshubError, rsshubSuccess,
  loadRsshubSettings, saveRsshubSettings,
} = useRsshubConfig()

function resetToDefault() {
  rsshubBaseUrl.value = rsshubDefault.value
}

onMounted(() => {
  loadRsshubSettings()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="rsshubLoading && !rsshubBaseUrl" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin" style="color: var(--color-link)" />
    </div>
    <template v-else>
      <div v-if="rsshubSuccess" class="p-3 rounded-lg text-sm" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
        {{ rsshubSuccess }}
      </div>
      <div v-if="rsshubError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
        {{ rsshubError }}
      </div>

      <div>
        <h3 class="font-semibold" style="color: var(--color-text-primary)">RSSHub 实例</h3>
        <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">
          订阅源发现从该实例拉取路由目录与订阅地址。留空使用默认自建实例。
        </p>
      </div>

      <div>
        <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">实例地址</label>
        <input
          :value="rsshubBaseUrl"
          type="text"
          class="w-full px-3 py-2 rounded-lg text-sm input"
          placeholder="http://your-rsshub-host:1200"
          @input="rsshubBaseUrl = ($event.target as HTMLInputElement).value"
        />
        <p class="text-xs mt-1.5" style="color: var(--color-text-muted)">
          默认：<code>{{ rsshubDefault }}</code>
          <button
            v-if="rsshubDefault"
            type="button"
            class="ml-2 underline hover:opacity-80"
            style="color: var(--color-link)"
            @click="resetToDefault"
          >
            恢复默认
          </button>
          <span v-if="rsshubConfigured" class="ml-2" style="color: var(--color-success)">● 已自定义</span>
        </p>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="px-4 py-2 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          style="background: var(--color-accent)"
          :disabled="rsshubLoading"
          @click="saveRsshubSettings"
        >
          <span v-if="rsshubLoading" class="flex items-center gap-2">
            <Icon icon="mdi:loading" width="16" class="animate-spin" />
            保存中...
          </span>
          <span v-else>保存设置</span>
        </button>
      </div>
    </template>
  </div>
</template>
