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
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin text-blue-500" />
    </div>
    <template v-else>
      <div v-if="firecrawlSuccess" class="p-3 bg-green-50 border border-green-200 rounded-lg text-sm text-green-600">
        {{ firecrawlSuccess }}
      </div>
      <div v-if="firecrawlError" class="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
        {{ firecrawlError }}
      </div>

      <div class="flex items-center justify-between">
        <div>
          <h3 class="font-semibold text-gray-800">Firecrawl 爬虫</h3>
          <p class="text-sm text-gray-500 mt-0.5">使用 Firecrawl 获取文章正文内容以提高阅读体验</p>
        </div>
        <label class="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            :checked="firecrawlEnabled"
            class="sr-only peer"
            @change="firecrawlEnabled = !firecrawlEnabled"
          />
          <div class="w-9 h-5 bg-gray-200 rounded-full peer peer-checked:bg-blue-600 peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all" />
        </label>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">API URL</label>
          <input
            :value="firecrawlApiUrl"
            type="text"
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="http://localhost:11235"
            @input="firecrawlApiUrl = ($event.target as HTMLInputElement).value"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            API Key
            <span class="text-gray-400 font-normal">（可选）</span>
          </label>
          <div class="relative">
            <input
              :type="firecrawlApiKeyVisible ? 'text' : 'password'"
              :value="firecrawlApiKey"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm pr-10 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              placeholder="fkc-..."
              @input="firecrawlApiKey = ($event.target as HTMLInputElement).value"
            />
            <button
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              @click="firecrawlApiKeyVisible = !firecrawlApiKeyVisible"
            >
              <Icon :icon="firecrawlApiKeyVisible ? 'mdi:eye-off' : 'mdi:eye'" width="18" />
            </button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">爬取模式</label>
          <select
            :value="firecrawlMode"
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            @change="firecrawlMode = ($event.target as HTMLSelectElement).value"
          >
            <option value="scrape">爬取（scrape）</option>
            <option value="crawl">抓取（crawl）</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">超时时间（秒）</label>
          <input
            :value="firecrawlTimeout"
            type="number"
            min="10"
            max="300"
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            @input="firecrawlTimeout = Number(($event.target as HTMLInputElement).value)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">最大内容长度</label>
          <input
            :value="firecrawlMaxContentLength"
            type="number"
            min="1000"
            max="500000"
            step="5000"
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            @input="firecrawlMaxContentLength = Number(($event.target as HTMLInputElement).value)"
          />
        </div>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
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
