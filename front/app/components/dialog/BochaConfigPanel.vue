<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted } from 'vue'
import { useBochaConfig } from '~/composables/useBochaConfig'

const {
  bochaEnabled, bochaEndpoint, bochaApiKey, bochaApiKeyConfigured,
  bochaApiKeyVisible, bochaLoading,
  bochaError, bochaSuccess,
  loadBochaSettings, saveBochaSettings,
} = useBochaConfig()

onMounted(() => {
  loadBochaSettings()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="bochaLoading && !bochaEnabled && !bochaEndpoint" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin" style="color: var(--color-link)" />
    </div>
    <template v-else>
      <div v-if="bochaSuccess" class="p-3 rounded-lg text-sm" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
        {{ bochaSuccess }}
      </div>
      <div v-if="bochaError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
        {{ bochaError }}
      </div>

      <div class="flex items-center justify-between">
        <div>
          <h3 class="font-semibold" style="color: var(--color-text-primary)">博查搜索</h3>
          <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">数据增强（看板分析）的联网检索后端，未配置 Key 时 web_search 自动降级</p>
        </div>
        <AppToggle v-model="bochaEnabled" />
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">API Endpoint</label>
          <input
            :value="bochaEndpoint"
            type="text"
            class="w-full px-3 py-2 rounded-lg text-sm input"
            placeholder="https://api.bochaai.com/v1/web-search"
            @input="bochaEndpoint = ($event.target as HTMLInputElement).value"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">API Key</label>
          <div class="relative">
            <input
              :type="bochaApiKeyVisible ? 'text' : 'password'"
              :value="bochaApiKey"
              class="w-full px-3 py-2 rounded-lg text-sm pr-10 input"
              :placeholder="bochaApiKeyConfigured ? '已配置，留空保持不变' : 'sk-...'"
              @input="bochaApiKey = ($event.target as HTMLInputElement).value"
            />
            <button
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 hover:opacity-80"
              style="color: var(--color-text-muted)"
              @click="bochaApiKeyVisible = !bochaApiKeyVisible"
            >
              <Icon :icon="bochaApiKeyVisible ? 'mdi:eye-off' : 'mdi:eye'" width="18" />
            </button>
          </div>
          <p v-if="bochaApiKeyConfigured" class="text-xs mt-1" style="color: var(--color-text-muted)">
            当前已配置 Key，留空保存不修改；输入新值则覆盖
          </p>
        </div>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="px-4 py-2 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          style="background: var(--color-accent)"
          :disabled="bochaLoading"
          @click="saveBochaSettings"
        >
          <span v-if="bochaLoading" class="flex items-center gap-2">
            <Icon icon="mdi:loading" width="16" class="animate-spin" />
            保存中...
          </span>
          <span v-else>保存设置</span>
        </button>
      </div>
    </template>
  </div>
</template>
