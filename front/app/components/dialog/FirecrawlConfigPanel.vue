<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted } from 'vue'
import { useFirecrawlConfig } from '~/composables/useFirecrawlConfig'

const {
  firecrawlEnabled, firecrawlApiUrl, firecrawlApiKey, firecrawlMode,
  firecrawlTimeout, firecrawlMaxContentLength, firecrawlApiKeyVisible, firecrawlLoading,
  firecrawlError, firecrawlSuccess,
  loadFirecrawlSettings, saveFirecrawlSettings,
} = useFirecrawlConfig()

onMounted(() => {
  loadFirecrawlSettings()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="firecrawlLoading && !firecrawlEnabled && !firecrawlApiUrl" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin" style="color: var(--color-link)" />
    </div>
    <template v-else>
      <div v-if="firecrawlSuccess" class="p-3 rounded-lg text-sm" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
        {{ firecrawlSuccess }}
      </div>
      <div v-if="firecrawlError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
        {{ firecrawlError }}
      </div>

      <div class="flex items-center justify-between">
        <div>
          <h3 class="font-semibold" style="color: var(--color-text-primary)">Firecrawl 爬虫</h3>
          <p class="text-sm mt-0.5" style="color: var(--color-text-secondary)">使用 Firecrawl 获取文章正文内容以提高阅读体验</p>
        </div>
        <AppToggle v-model="firecrawlEnabled" />
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">API URL</label>
          <input
            :value="firecrawlApiUrl"
            type="text"
            class="w-full px-3 py-2 rounded-lg text-sm input"
            placeholder="http://localhost:11235"
            @input="firecrawlApiUrl = ($event.target as HTMLInputElement).value"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">
            API Key
            <span style="color: var(--color-text-muted)">（可选）</span>
          </label>
          <div class="relative">
            <input
              :type="firecrawlApiKeyVisible ? 'text' : 'password'"
              :value="firecrawlApiKey"
              class="w-full px-3 py-2 rounded-lg text-sm pr-10 input"
              placeholder="fkc-..."
              @input="firecrawlApiKey = ($event.target as HTMLInputElement).value"
            />
            <button
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 hover:opacity-80"
              style="color: var(--color-text-muted)"
              @click="firecrawlApiKeyVisible = !firecrawlApiKeyVisible"
            >
              <Icon :icon="firecrawlApiKeyVisible ? 'mdi:eye-off' : 'mdi:eye'" width="18" />
            </button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">爬取模式</label>
          <select
            :value="firecrawlMode"
            class="w-full px-3 py-2 rounded-lg text-sm input"
            @change="firecrawlMode = ($event.target as HTMLSelectElement).value"
          >
            <option value="scrape">爬取（scrape）</option>
            <option value="crawl">抓取（crawl）</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">超时时间（秒）</label>
          <input
            :value="firecrawlTimeout"
            type="number"
            min="10"
            max="300"
            class="w-full px-3 py-2 rounded-lg text-sm input"
            @input="firecrawlTimeout = Number(($event.target as HTMLInputElement).value)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--color-text-secondary)">最大内容长度</label>
          <input
            :value="firecrawlMaxContentLength"
            type="number"
            min="1000"
            max="500000"
            step="5000"
            class="w-full px-3 py-2 rounded-lg text-sm input"
            @input="firecrawlMaxContentLength = Number(($event.target as HTMLInputElement).value)"
          />
        </div>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="px-4 py-2 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          style="background: var(--color-accent)"
          :disabled="firecrawlLoading"
          @click="saveFirecrawlSettings"
        >
          <span v-if="firecrawlLoading" class="flex items-center gap-2">
            <Icon icon="mdi:loading" width="16" class="animate-spin" />
            保存中...
          </span>
          <span v-else>保存设置</span>
        </button>
      </div>
    </template>
  </div>
</template>
