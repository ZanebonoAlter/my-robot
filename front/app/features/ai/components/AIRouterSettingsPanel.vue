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
  editingProviderId, editProviderForm, showEditProviderApiKey,
  backupProviders, showNewProviderForm,
  savePrimaryProvider, testPrimaryProvider,
  deleteBackupProvider, isProviderLinked,
} = ctx
</script>

<template>
  <div class="space-y-5">
    <div v-if="loading" class="py-12 flex justify-center">
      <Icon icon="mdi:loading" width="32" height="32" class="animate-spin text-[var(--color-text-secondary)]" />
    </div>

    <template v-else>
      <!-- Section 1: Primary Provider -->
      <div class="rounded-xl border border-[var(--color-border-subtle)] overflow-hidden" style="background: var(--color-bg-elevated)">
        <div class="px-5 py-3.5 border-b border-[var(--color-border-subtle)] flex items-center justify-between" style="background: var(--color-bg-hover)">
          <AppSectionHeader title="主模型" description="默认 AI 提供者，保存后自动挂载到所有能力路由" icon-name="mdi:star-four-points" />
          <div class="flex items-center gap-2">
            <button class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors disabled:opacity-50"
              style="color: var(--color-link); background: var(--color-link-subtle); border: 1px solid var(--color-link-border)"
              :disabled="testing" @click="testPrimaryProvider">
              <Icon v-if="testing" icon="mdi:loading" width="12" height="12" class="animate-spin inline-block mr-1" /> 测试连接
            </button>
            <button class="px-3 py-1.5 text-xs font-medium text-white rounded-lg transition-colors disabled:opacity-50"
              style="background: var(--color-accent)"
              :disabled="saving" @click="savePrimaryProvider">保存</button>
          </div>
        </div>

        <div class="p-5 space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">名称</label>
              <AppInput v-model="primaryProviderForm.name" type="text" placeholder="default-primary" />
            </div>
            <div>
              <label class="block text-[11px] font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted)">类型</label>
              <select v-model="primaryProviderForm.provider_type" class="ai-select">
                <option value="openai_compatible">OpenAI Compatible</option>
                <option value="ollama">Ollama (本地)</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">模型</label>
              <AppInput v-model="primaryProviderForm.model" type="text" placeholder="gpt-4o-mini" />
            </div>
          </div>
          <div>
            <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">Base URL</label>
            <AppInput v-model="primaryProviderForm.base_url" type="text"
              :placeholder="primaryProviderForm.provider_type === 'ollama' ? 'http://localhost:11434/v1' : 'https://api.openai.com/v1'" />
          </div>
          <div v-if="primaryProviderForm.provider_type !== 'ollama'">
            <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">API Key</label>
            <div class="relative">
              <AppInput v-model="primaryProviderForm.api_key" :type="showPrimaryApiKey ? 'text' : 'password'" placeholder="留空表示沿用已保存密钥" />
              <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="showPrimaryApiKey = !showPrimaryApiKey">
                <Icon :icon="showPrimaryApiKey ? 'mdi:eye-off' : 'mdi:eye'" width="15" height="15" />
              </button>
            </div>
          </div>
          <div v-else class="rounded-lg px-3 py-2 text-xs flex items-center gap-2" style="background: var(--color-warning-bg, rgba(196, 136, 60, 0.1)); border: 1px solid var(--color-warning-border, rgba(196, 136, 60, 0.25)); color: var(--color-warning)">
            <Icon icon="mdi:information-outline" width="14" height="14" class="shrink-0" /> Ollama 无需 API Key，确保服务已启动
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">超时 (秒)</label>
              <AppInput v-model="primaryProviderForm.timeout_seconds" type="number" min="30" />
            </div>
            <div>
              <label class="block text-[11px] font-medium text-gray-500 uppercase tracking-wider mb-1">总结时间范围 (分钟)</label>
              <AppInput v-model="primaryProviderForm.time_range" type="number" min="60" step="60" />
            </div>
          </div>
          <div class="flex items-center gap-2.5">
            <AppToggle v-model="primaryProviderForm.enable_thinking" />
            <span class="text-sm text-gray-700">启用 Thinking（推理模型的思考过程，会消耗额外 token）</span>
          </div>
        </div>
      </div>

      <!-- Section 2: Backup Providers (extracted) -->
      <AIRouterBackupProviders />

      <!-- Section 3: Capability Routes (extracted) -->
      <AIRouterCapabilityRoutes />
    </template>

    <div v-if="success" class="rounded-lg px-4 py-2.5 text-xs flex items-center gap-2" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
      <Icon icon="mdi:check-circle" width="14" height="14" /> {{ success }}
    </div>
    <div v-if="error" class="rounded-lg px-4 py-2.5 text-xs flex items-center gap-2" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
      <Icon icon="mdi:alert-circle" width="14" height="14" /> {{ error }}
    </div>
  </div>
</template>

<style scoped>
/* Override hardcoded Tailwind gray classes with theme tokens */
:deep(.text-gray-500) {
  color: var(--color-text-muted) !important;
}

:deep(.text-gray-400) {
  color: var(--color-text-muted) !important;
}

:deep(.text-gray-700) {
  color: var(--color-text-secondary) !important;
}

:deep(.hover\:text-gray-600:hover) {
  color: var(--color-text-secondary) !important;
}

.ai-select {
  width: 100%;
  padding: 7px 10px;
  font-size: 13px;
  border: 1px solid var(--color-input-border);
  border-radius: 8px;
  background: var(--color-input-bg);
  color: var(--color-text-primary);
  outline: none;
  transition: border-color 0.15s;
}

.ai-select:focus {
  border-color: var(--color-input-focus);
}
</style>
