<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { provide, reactive } from 'vue'
import { useAIRouterSettings } from '~/features/ai/composables/useAIRouterSettings'
import AIRouterBackupProviders from './AIRouterBackupProviders.vue'
import AIRouterCapabilityRoutes from './AIRouterCapabilityRoutes.vue'

const ctx = useAIRouterSettings()
provide('ai-router-ctx', reactive(ctx))

const {
  loading, saving, testing, error, success,
  primaryProviderForm, showPrimaryApiKey,
  newProviderForm, showNewProviderApiKey,
  embeddingThreshold, savingThreshold,
  editingProviderId, editProviderForm, showEditProviderApiKey,
  backupProviders, showNewProviderForm,
  savePrimaryProvider, testPrimaryProvider,
  deleteBackupProvider, isProviderLinked,
  saveThreshold,
} = ctx
</script>

<template>
  <div class="space-y-5">
    <div v-if="loading" class="py-12 flex justify-center">
      <Icon icon="mdi:loading" width="32" height="32" class="animate-spin text-ink-500" />
    </div>

    <template v-else>
      <!-- Section 1: Primary Provider -->
      <div class="rounded-xl border border-ink-100 bg-gradient-to-br from-ink-50/80 via-white to-paper-cream/60 overflow-hidden">
        <div class="px-5 py-3.5 border-b border-ink-100/60 flex items-center justify-between bg-ink-50/40">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-ink-600 to-ink-800 flex items-center justify-center shadow-sm">
              <Icon icon="mdi:star-four-points" width="16" height="16" class="text-white" />
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900">主模型</h3>
              <p class="text-[11px] text-gray-500">默认 AI 提供者，保存后自动挂载到所有能力路由</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button class="px-3 py-1.5 text-xs font-medium text-blue-700 bg-blue-50 border border-blue-200 rounded-lg hover:bg-blue-100 transition-colors disabled:opacity-50"
              :disabled="testing" @click="testPrimaryProvider">
              <Icon v-if="testing" icon="mdi:loading" width="12" height="12" class="animate-spin inline-block mr-1" /> 测试连接
            </button>
            <button class="px-3 py-1.5 text-xs font-medium text-white bg-ink-700 rounded-lg hover:bg-ink-800 transition-colors disabled:opacity-50"
              :disabled="saving" @click="savePrimaryProvider">保存</button>
          </div>
        </div>

        <div class="p-5 space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">名称</label>
              <input v-model="primaryProviderForm.name" type="text" class="input w-full text-sm" placeholder="default-primary">
            </div>
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">类型</label>
              <select v-model="primaryProviderForm.provider_type" class="input w-full text-sm">
                <option value="openai_compatible">OpenAI Compatible</option>
                <option value="ollama">Ollama (本地)</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">模型</label>
              <input v-model="primaryProviderForm.model" type="text" class="input w-full text-sm" placeholder="gpt-4o-mini">
            </div>
          </div>
          <div>
            <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">Base URL</label>
            <input v-model="primaryProviderForm.base_url" type="text" class="input w-full text-sm"
              :placeholder="primaryProviderForm.provider_type === 'ollama' ? 'http://localhost:11434/v1' : 'https://api.openai.com/v1'">
          </div>
          <div v-if="primaryProviderForm.provider_type !== 'ollama'">
            <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">API Key</label>
            <div class="relative">
              <input v-model="primaryProviderForm.api_key" :type="showPrimaryApiKey ? 'text' : 'password'" class="input w-full text-sm pr-10" placeholder="留空表示沿用已保存密钥">
              <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="showPrimaryApiKey = !showPrimaryApiKey">
                <Icon :icon="showPrimaryApiKey ? 'mdi:eye-off' : 'mdi:eye'" width="15" height="15" />
              </button>
            </div>
          </div>
          <div v-else class="rounded-lg bg-amber-50 border border-amber-200/80 px-3 py-2 text-xs text-amber-700 flex items-center gap-2">
            <Icon icon="mdi:information-outline" width="14" height="14" class="shrink-0" /> Ollama 无需 API Key，确保服务已启动
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">超时 (秒)</label>
              <input v-model.number="primaryProviderForm.timeout_seconds" type="number" min="30" class="input w-full text-sm">
            </div>
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">总结时间范围 (分钟)</label>
              <input v-model.number="primaryProviderForm.time_range" type="number" min="60" step="60" class="input w-full text-sm">
            </div>
          </div>
          <label class="flex items-center gap-2.5 cursor-pointer select-none">
            <input v-model="primaryProviderForm.enable_thinking" type="checkbox" class="rounded">
            <span class="text-sm text-gray-700">启用 Thinking（推理模型的思考过程，会消耗额外 token）</span>
          </label>
        </div>
      </div>

      <!-- Section 2: Backup Providers (extracted) -->
      <AIRouterBackupProviders />

      <!-- Section 3: Capability Routes (extracted) -->
      <AIRouterCapabilityRoutes />

      <!-- Section 4: Embedding Threshold -->
      <div class="rounded-xl border border-gray-200 bg-white overflow-hidden">
        <div class="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-violet-500 to-violet-700 flex items-center justify-center shadow-sm">
              <Icon icon="mdi:tune-variant" width="16" height="16" class="text-white" />
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900">板块匹配阈值</h3>
              <p class="text-[11px] text-gray-500">Embedding 相似度阈值，越低标签越容易匹配到板块</p>
            </div>
          </div>
          <button class="px-3 py-1.5 text-xs font-medium text-white bg-violet-600 rounded-lg hover:bg-violet-700 transition-colors disabled:opacity-50"
            :disabled="savingThreshold" @click="saveThreshold">
            <Icon v-if="savingThreshold" icon="mdi:loading" width="12" height="12" class="animate-spin inline-block mr-1" /> 保存
          </button>
        </div>
        <div class="p-5">
          <div class="flex items-center gap-4">
            <input v-model.number="embeddingThreshold" type="range" min="0.1" max="0.95" step="0.05"
              class="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
              :style="{ background: `linear-gradient(to right, #8b5cf6 0%, #8b5cf6 ${(embeddingThreshold - 0.1) / 0.85 * 100}%, #e5e7eb ${(embeddingThreshold - 0.1) / 0.85 * 100}%, #e5e7eb 100%)` }">
            <span class="text-base font-bold text-violet-700 w-12 text-right tabular-nums">{{ embeddingThreshold.toFixed(2) }}</span>
          </div>
          <div class="flex justify-between mt-1">
            <span class="text-[10px] text-gray-400">0.10 宽松</span>
            <span class="text-[10px] text-gray-400">0.95 严格</span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="success" class="rounded-lg bg-emerald-50 border border-emerald-200 px-4 py-2.5 text-xs text-emerald-700 flex items-center gap-2">
      <Icon icon="mdi:check-circle" width="14" height="14" /> {{ success }}
    </div>
    <div v-if="error" class="rounded-lg bg-red-50 border border-red-200 px-4 py-2.5 text-xs text-red-700 flex items-center gap-2">
      <Icon icon="mdi:alert-circle" width="14" height="14" /> {{ error }}
    </div>
  </div>
</template>
