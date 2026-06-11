<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { inject } from 'vue'

interface AIRouterCtx {
  saving: boolean
  capabilityOrder: string[]
  routeLabels: Record<string, string>
  primaryProviderId: string | null
  primaryProviderForm: { name?: string }
  backupProviders: any[]
  draggingCapability: string | null
  draggingProviderId: string | null
  routeSummary: (cap: string) => string[]
  providerName: (id: string) => string
  saveRoutes: () => void
  addPrimaryToRoute: (cap: string) => void
  addProviderToRoute: (cap: string, id: string) => void
  removeProviderFromRoute: (cap: string, id: string) => void
  moveProvider: (cap: string, id: string, dir: number) => void
  handleDragStart: (cap: string, id: string) => void
  handleDragEnd: () => void
  handleDropOnProvider: (cap: string, id: string) => void
}

const ctx = inject<AIRouterCtx>('ai-router-ctx')!
</script>

<template>
  <div class="rounded-xl border border-gray-200 bg-white overflow-hidden">
    <div class="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-slate-600 to-slate-800 flex items-center justify-center shadow-sm">
          <Icon icon="mdi:transit-connection-variant" width="16" height="16" class="text-white" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-gray-900">能力路由</h3>
          <p class="text-[11px] text-gray-500">按顺序依次尝试，失败自动降级到下一个</p>
        </div>
      </div>
      <button class="px-3 py-1.5 text-xs font-medium text-white bg-slate-700 rounded-lg hover:bg-slate-800 transition-colors disabled:opacity-50"
        :disabled="ctx.saving" @click="ctx.saveRoutes">
        <Icon v-if="ctx.saving" icon="mdi:loading" width="12" height="12" class="animate-spin inline-block mr-1" />
        保存路由
      </button>
    </div>

    <div class="divide-y divide-gray-100">
      <div v-for="capability in ctx.capabilityOrder" :key="capability" class="px-5 py-4">
        <div class="flex items-center gap-2 mb-3">
          <div class="w-6 h-6 rounded flex items-center justify-center text-[10px] font-bold shrink-0"
            :class="ctx.routeSummary(capability).length > 0 ? 'bg-slate-700 text-white' : 'bg-gray-200 text-gray-500'">
            {{ ctx.routeLabels[capability]?.charAt(0) }}
          </div>
          <span class="text-sm font-medium text-gray-800">{{ ctx.routeLabels[capability] }}</span>
          <span class="text-[11px] text-gray-400">{{ ctx.routeSummary(capability).length }} provider</span>
        </div>

        <div v-if="ctx.routeSummary(capability).length === 0" class="text-center py-3 text-[11px] text-gray-400 rounded-lg border border-dashed border-gray-200 mb-3">
          点击下方按钮添加 provider
        </div>

        <div v-else class="space-y-1.5 mb-3">
          <div v-for="(providerId, index) in ctx.routeSummary(capability)" :key="providerId"
            draggable="true"
            class="flex items-center gap-2 px-3 py-2 rounded-lg border transition-all cursor-move select-none"
            :class="[
              providerId === ctx.primaryProviderId ? 'border-ink-200/80 bg-ink-50/50' : 'border-gray-200 bg-gray-50/50 hover:bg-gray-100/60',
              ctx.draggingCapability === capability && ctx.draggingProviderId === providerId ? 'opacity-40 ring-2 ring-blue-300' : ''
            ]"
            @dragstart="ctx.handleDragStart(capability, providerId)"
            @dragend="ctx.handleDragEnd"
            @dragover.prevent
            @drop.prevent="ctx.handleDropOnProvider(capability, providerId)"
          >
            <span class="w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold shrink-0"
              :class="index === 0 ? 'bg-ink-700 text-white' : 'bg-gray-300 text-gray-600'">{{ index + 1 }}</span>
            <Icon icon="mdi:drag" width="12" height="12" class="text-gray-300 shrink-0" />
            <div class="flex-1 min-w-0">
              <span class="text-sm truncate" :class="providerId === ctx.primaryProviderId ? 'font-medium text-ink-900' : 'text-gray-700'">{{ ctx.providerName(providerId) }}</span>
            </div>
            <span v-if="providerId === ctx.primaryProviderId" class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-ink-100 text-ink-600 shrink-0">主</span>
            <span v-else class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-teal-50 text-teal-600 shrink-0">备</span>
            <div class="flex items-center gap-0.5 shrink-0">
              <button class="p-0.5 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-30"
                :disabled="index === 0" @click="ctx.moveProvider(capability, providerId, -1)">
                <Icon icon="mdi:chevron-up" width="14" height="14" />
              </button>
              <button class="p-0.5 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-30"
                :disabled="index === ctx.routeSummary(capability).length - 1" @click="ctx.moveProvider(capability, providerId, 1)">
                <Icon icon="mdi:chevron-down" width="14" height="14" />
              </button>
              <button class="p-0.5 rounded hover:bg-red-100 text-gray-400 hover:text-red-500 transition-colors"
                @click="ctx.removeProviderFromRoute(capability, providerId)">
                <Icon icon="mdi:close" width="13" height="13" />
              </button>
            </div>
          </div>
        </div>

        <div class="flex flex-wrap gap-1.5">
          <button v-if="ctx.primaryProviderId && !ctx.routeSummary(capability).includes(ctx.primaryProviderId)"
            class="px-2 py-0.5 text-[11px] font-medium rounded border border-ink-200 bg-ink-50 text-ink-700 hover:bg-ink-100 transition-colors"
            @click="ctx.addPrimaryToRoute(capability)">
            + {{ ctx.primaryProviderForm.name || '主模型' }}
          </button>
          <button v-for="provider in ctx.backupProviders" :key="provider.id"
            class="px-2 py-0.5 text-[11px] font-medium rounded border transition-colors"
            :class="ctx.routeSummary(capability).includes(provider.id) ? 'border-teal-200 bg-teal-50 text-teal-700' : 'border-gray-200 bg-white text-gray-500 hover:bg-gray-50'"
            @click="ctx.routeSummary(capability).includes(provider.id) ? ctx.removeProviderFromRoute(capability, provider.id) : ctx.addProviderToRoute(capability, provider.id)">
            {{ ctx.routeSummary(capability).includes(provider.id) ? '✓' : '+' }} {{ provider.name }}
          </button>
          <span v-if="!ctx.primaryProviderId && ctx.backupProviders.length === 0" class="text-[11px] text-gray-400 self-center">先在上方创建 provider</span>
        </div>
      </div>
    </div>
  </div>
</template>
