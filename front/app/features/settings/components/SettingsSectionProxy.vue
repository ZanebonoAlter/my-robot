<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted } from 'vue'
import { useProxyConfig } from '~/composables/useProxyConfig'

const {
  proxyUrl, proxyConfigured, proxyLoading,
  proxyError, proxySuccess,
  loadProxySettings, saveProxySettings,
} = useProxyConfig()

function clearProxy() {
  proxyUrl.value = ''
}

onMounted(() => {
  loadProxySettings()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="proxyLoading && !proxyUrl && !proxyConfigured" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin" style="color: var(--color-link)" />
    </div>
    <template v-else>
      <div v-if="proxySuccess" class="p-3 rounded-lg text-sm" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
        {{ proxySuccess }}
      </div>
      <div v-if="proxyError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
        {{ proxyError }}
      </div>

      <div>
        <h3 class="font-semibold" style="color: var(--color-text-primary)">出站代理</h3>
        <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">
          feed 抓取、Firecrawl、LLM 等所有外部请求经此代理转发。留空=直连（重启后仍生效）。
        </p>
      </div>

      <div>
        <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">代理地址</label>
        <input
          :value="proxyUrl"
          type="text"
          class="w-full px-3 py-2 rounded-lg text-sm input"
          placeholder="http://127.0.0.1:7890 或 socks5://127.0.0.1:1080"
          @input="proxyUrl = ($event.target as HTMLInputElement).value"
        />
        <p class="text-xs mt-1.5" style="color: var(--color-text-muted)">
          支持协议：<code>http</code> / <code>https</code> / <code>socks5</code>，保存后即时生效。
          <button
            v-if="proxyConfigured"
            type="button"
            class="ml-2 underline hover:opacity-80"
            style="color: var(--color-link)"
            @click="clearProxy"
          >
            清除代理
          </button>
          <span v-if="proxyConfigured" class="ml-2" style="color: var(--color-success)">● 已启用</span>
        </p>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="px-4 py-2 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          style="background: var(--color-accent)"
          :disabled="proxyLoading"
          @click="saveProxySettings"
        >
          <span v-if="proxyLoading" class="flex items-center gap-2">
            <Icon icon="mdi:loading" width="16" class="animate-spin" />
            保存中...
          </span>
          <span v-else>保存设置</span>
        </button>
      </div>
    </template>
  </div>
</template>
