<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { inject } from 'vue'

interface AIRouterCtx {
  saving: boolean
  backupProviders: any[]
  showNewProviderForm: boolean
  newProviderForm: any
  editingProviderId: string | null
  editProviderForm: any
  showEditProviderApiKey: boolean
  showNewProviderApiKey: boolean
  saveNewProvider: () => void
  startEditingProvider: (p: any) => void
  cancelEditingProvider: () => void
  saveEditedProvider: () => void
  deleteBackupProvider: (p: any) => void
  isProviderLinked: (id: string) => boolean
}

const ctx = inject<AIRouterCtx>('ai-router-ctx')!
</script>

<template>
  <div class="rounded-xl border border-gray-200 bg-white overflow-hidden">
    <div class="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-teal-500 to-teal-700 flex items-center justify-center shadow-sm">
          <Icon icon="mdi:server-network" width="16" height="16" class="text-white" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-gray-900">备用模型池</h3>
          <p class="text-[11px] text-gray-500">挂到能力路由做 failover，主模型挂了自动切</p>
        </div>
      </div>
      <button
        class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 transition-colors flex items-center gap-1"
        @click="ctx.showNewProviderForm = !ctx.showNewProviderForm"
      >
        <Icon :icon="ctx.showNewProviderForm ? 'mdi:chevron-up' : 'mdi:plus'" width="14" height="14" />
        {{ ctx.showNewProviderForm ? '收起' : '新增' }}
      </button>
    </div>

    <div class="p-5 space-y-4">
      <!-- New Provider Form -->
      <div v-if="ctx.showNewProviderForm" class="rounded-lg border border-dashed border-gray-300 p-4 bg-gray-50/60 space-y-3">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <input v-model="ctx.newProviderForm.name" type="text" class="input w-full text-sm" placeholder="名称">
          <input v-model="ctx.newProviderForm.model" type="text" class="input w-full text-sm" placeholder="模型名">
          <select v-model="ctx.newProviderForm.provider_type" class="input w-full text-sm">
            <option value="openai_compatible">OpenAI Compatible</option>
            <option value="ollama">Ollama (本地)</option>
          </select>
          <input v-model="ctx.newProviderForm.base_url" type="text" class="input w-full text-sm"
            :placeholder="ctx.newProviderForm.provider_type === 'ollama' ? 'http://localhost:11434/v1' : 'https://api.example.com/v1'">
          <div v-if="ctx.newProviderForm.provider_type !== 'ollama'" class="relative md:col-span-2">
            <input v-model="ctx.newProviderForm.api_key" :type="ctx.showNewProviderApiKey ? 'text' : 'password'" class="input w-full text-sm pr-10" placeholder="API Key">
            <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="ctx.showNewProviderApiKey = !ctx.showNewProviderApiKey">
              <Icon :icon="ctx.showNewProviderApiKey ? 'mdi:eye-off' : 'mdi:eye'" width="15" height="15" />
            </button>
          </div>
          <div v-else class="md:col-span-2 rounded-lg bg-amber-50 border border-amber-200/80 px-3 py-2 text-xs text-amber-700">Ollama 模式无需 API Key</div>
          <label class="flex items-center gap-2 text-sm text-gray-700 self-center">
            <input v-model="ctx.newProviderForm.enable_thinking" type="checkbox" class="rounded"> Thinking
          </label>
        </div>
        <div class="flex justify-end">
          <button class="px-3 py-1.5 text-xs font-medium text-white bg-teal-600 rounded-lg hover:bg-teal-700 transition-colors disabled:opacity-50" :disabled="ctx.saving" @click="ctx.saveNewProvider">添加</button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="ctx.backupProviders.length === 0" class="text-center py-6 text-xs text-gray-400">
        还没有备用模型，先加一个
      </div>

      <!-- Provider List -->
      <div v-else class="space-y-2">
        <div v-for="provider in ctx.backupProviders" :key="provider.id" class="rounded-lg border border-gray-150 bg-gray-50/40 px-4 py-3">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3 min-w-0">
              <div class="w-7 h-7 rounded-md bg-gradient-to-br from-gray-200 to-gray-300 flex items-center justify-center shrink-0">
                <Icon icon="mdi:cube-outline" width="14" height="14" class="text-gray-600" />
              </div>
              <div class="min-w-0">
                <div class="text-sm font-medium text-gray-900 truncate">{{ provider.name }}</div>
                <div class="text-[11px] text-gray-500 truncate">{{ provider.model }} · {{ provider.base_url }}</div>
              </div>
            </div>
            <div class="flex items-center gap-1.5 shrink-0">
              <span class="text-[10px] px-1.5 py-0.5 rounded-full font-medium" :class="provider.enabled ? 'bg-emerald-50 text-emerald-600' : 'bg-gray-100 text-gray-400'">
                {{ provider.enabled ? '启用' : '停用' }}
              </span>
              <button class="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors" @click="ctx.startEditingProvider(provider)">
                <Icon icon="mdi:pencil-outline" width="14" height="14" />
              </button>
              <button class="p-1 rounded hover:bg-red-100 text-gray-400 hover:text-red-600 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                :disabled="ctx.isProviderLinked(provider.id)" @click="ctx.deleteBackupProvider(provider)">
                <Icon icon="mdi:trash-can-outline" width="14" height="14" />
              </button>
            </div>
          </div>
          <p v-if="ctx.isProviderLinked(provider.id)" class="mt-2 text-[11px] text-amber-600 pl-10">还挂在某条路由上，先移除再删</p>

          <!-- Edit Form -->
          <div v-if="ctx.editingProviderId === provider.id" class="mt-3 pt-3 border-t border-gray-200 space-y-3">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <input v-model="ctx.editProviderForm.name" type="text" class="input w-full text-sm" placeholder="名称">
              <input v-model="ctx.editProviderForm.model" type="text" class="input w-full text-sm" placeholder="模型名">
              <select v-model="ctx.editProviderForm.provider_type" class="input w-full text-sm">
                <option value="openai_compatible">OpenAI Compatible</option>
                <option value="ollama">Ollama (本地)</option>
              </select>
              <input v-model="ctx.editProviderForm.base_url" type="text" class="input w-full text-sm"
                :placeholder="ctx.editProviderForm.provider_type === 'ollama' ? 'http://localhost:11434/v1' : 'https://api.example.com/v1'">
              <div v-if="ctx.editProviderForm.provider_type !== 'ollama'" class="relative md:col-span-2">
                <input v-model="ctx.editProviderForm.api_key" :type="ctx.showEditProviderApiKey ? 'text' : 'password'" class="input w-full text-sm pr-10" placeholder="留空表示沿用已保存密钥">
                <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="ctx.showEditProviderApiKey = !ctx.showEditProviderApiKey">
                  <Icon :icon="ctx.showEditProviderApiKey ? 'mdi:eye-off' : 'mdi:eye'" width="15" height="15" />
                </button>
              </div>
              <div v-else class="md:col-span-2 rounded-lg bg-amber-50 border border-amber-200/80 px-3 py-2 text-xs text-amber-700">Ollama 模式无需 API Key</div>
              <input v-model.number="ctx.editProviderForm.timeout_seconds" type="number" min="30" class="input w-full text-sm" placeholder="Timeout (秒)">
              <label class="flex items-center gap-2 text-sm text-gray-700 self-center">
                <input v-model="ctx.editProviderForm.enabled" type="checkbox" class="rounded"> 启用
              </label>
              <label class="flex items-center gap-2 text-sm text-gray-700 self-center">
                <input v-model="ctx.editProviderForm.enable_thinking" type="checkbox" class="rounded"> Thinking
              </label>
            </div>
            <div class="flex justify-end gap-2">
              <button class="px-3 py-1.5 text-xs rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50" @click="ctx.cancelEditingProvider">取消</button>
              <button class="px-3 py-1.5 text-xs rounded-lg bg-ink-700 text-white hover:bg-ink-800 disabled:opacity-50" :disabled="ctx.saving" @click="ctx.saveEditedProvider">保存</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
