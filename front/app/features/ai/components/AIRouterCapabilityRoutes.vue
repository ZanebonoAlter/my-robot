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
  <div class="route-card">
    <div class="route-card__header">
      <AppSectionHeader title="能力路由" description="按顺序依次尝试，失败自动降级到下一个" icon-name="mdi:transit-connection-variant" />
      <AppButton variant="primary" size="sm" :disabled="ctx.saving" @click="ctx.saveRoutes">
        <Icon v-if="ctx.saving" icon="mdi:loading" width="12" height="12" class="animate-spin" />
        保存路由
      </AppButton>
    </div>

    <div class="route-card__sections">
      <div v-for="capability in ctx.capabilityOrder" :key="capability" class="route-section">
        <div class="route-section__title-row">
          <div class="route-section__badge" :class="ctx.routeSummary(capability).length > 0 ? 'route-section__badge--active' : 'route-section__badge--empty'">
            {{ ctx.routeLabels[capability]?.charAt(0) }}
          </div>
          <span class="route-section__name">{{ ctx.routeLabels[capability] }}</span>
          <span class="route-section__count">{{ ctx.routeSummary(capability).length }} provider</span>
        </div>

        <div v-if="ctx.routeSummary(capability).length === 0" class="route-section__empty">
          点击下方按钮添加 provider
        </div>

        <div v-else class="route-section__providers">
          <div v-for="(providerId, index) in ctx.routeSummary(capability)" :key="providerId"
            draggable="true"
            class="route-provider"
            :class="{
              'route-provider--primary': providerId === ctx.primaryProviderId,
              'route-provider--dragging': ctx.draggingCapability === capability && ctx.draggingProviderId === providerId,
            }"
            @dragstart="ctx.handleDragStart(capability, providerId)"
            @dragend="ctx.handleDragEnd"
            @dragover.prevent
            @drop.prevent="ctx.handleDropOnProvider(capability, providerId)"
          >
            <span class="route-provider__rank" :class="index === 0 ? 'route-provider__rank--first' : ''">{{ index + 1 }}</span>
            <Icon icon="mdi:drag" width="12" height="12" class="route-provider__drag" />
            <div class="flex-1 min-w-0">
              <span class="route-provider__name" :class="providerId === ctx.primaryProviderId ? 'route-provider__name--primary' : ''">{{ ctx.providerName(providerId) }}</span>
            </div>
            <span v-if="providerId === ctx.primaryProviderId" class="route-tag route-tag--primary">主</span>
            <span v-else class="route-tag route-tag--backup">备</span>
            <div class="flex items-center gap-0.5 shrink-0">
              <button class="route-icon-btn" :disabled="index === 0" @click="ctx.moveProvider(capability, providerId, -1)">
                <Icon icon="mdi:chevron-up" width="14" height="14" />
              </button>
              <button class="route-icon-btn" :disabled="index === ctx.routeSummary(capability).length - 1" @click="ctx.moveProvider(capability, providerId, 1)">
                <Icon icon="mdi:chevron-down" width="14" height="14" />
              </button>
              <button class="route-icon-btn route-icon-btn--danger" @click="ctx.removeProviderFromRoute(capability, providerId)">
                <Icon icon="mdi:close" width="13" height="13" />
              </button>
            </div>
          </div>
        </div>

        <div class="route-section__actions">
          <button v-if="ctx.primaryProviderId && !ctx.routeSummary(capability).includes(ctx.primaryProviderId)"
            class="route-chip"
            @click="ctx.addPrimaryToRoute(capability)">
            + {{ ctx.primaryProviderForm.name || '主模型' }}
          </button>
          <button v-for="provider in ctx.backupProviders" :key="provider.id"
            class="route-chip"
            :class="ctx.routeSummary(capability).includes(provider.id) ? 'route-chip--selected' : ''"
            @click="ctx.routeSummary(capability).includes(provider.id) ? ctx.removeProviderFromRoute(capability, provider.id) : ctx.addProviderToRoute(capability, provider.id)">
            {{ ctx.routeSummary(capability).includes(provider.id) ? '✓' : '+' }} {{ provider.name }}
          </button>
          <span v-if="!ctx.primaryProviderId && ctx.backupProviders.length === 0" class="route-section__hint">先在上方创建 provider</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.route-card {
  border-radius: 12px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
  overflow: hidden;
}

.route-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.route-card__sections {
  display: flex;
  flex-direction: column;
}

.route-section {
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.route-section:last-child {
  border-bottom: none;
}

.route-section__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.route-section__badge {
  width: 24px;
  height: 24px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}

.route-section__badge--active {
  background: var(--color-accent);
  color: var(--color-text-inverted);
}

.route-section__badge--empty {
  background: var(--color-bg-sunken);
  color: var(--color-text-muted);
}

.route-section__name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.route-section__count {
  font-size: 11px;
  color: var(--color-text-muted);
}

.route-section__empty {
  text-align: center;
  padding: 12px 0;
  font-size: 11px;
  color: var(--color-text-muted);
  border-radius: 8px;
  border: 1px dashed var(--color-border-medium);
  margin-bottom: 12px;
}

.route-section__providers {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 12px;
}

.route-provider {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-sunken);
  cursor: move;
  user-select: none;
  transition: opacity 0.15s, box-shadow 0.15s;
}

.route-provider--primary {
  background: var(--color-bg-hover);
}

.route-provider--dragging {
  opacity: 0.4;
  box-shadow: 0 0 0 2px var(--color-link);
}

.route-provider__rank {
  width: 20px;
  height: 20px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 700;
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
  flex-shrink: 0;
}

.route-provider__rank--first {
  background: var(--color-accent);
  color: var(--color-text-inverted);
}

.route-provider__drag {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.route-provider__name {
  font-size: 13px;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.route-provider__name--primary {
  font-weight: 500;
  color: var(--color-text-primary);
}

.route-tag {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 500;
  flex-shrink: 0;
}

.route-tag--primary {
  background: var(--color-bg-elevated);
  color: var(--color-text-secondary);
}

.route-tag--backup {
  background: rgba(45, 138, 122, 0.12);
  color: var(--color-success);
}

.route-icon-btn {
  padding: 2px;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.route-icon-btn:hover {
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
}

.route-icon-btn--danger:hover {
  background: rgba(196, 47, 60, 0.1);
  color: var(--color-error);
}

.route-icon-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.route-section__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.route-chip {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 6px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
  cursor: pointer;
  transition: background 0.15s;
}

.route-chip:hover {
  background: var(--color-bg-active);
}

.route-chip--selected {
  border-color: rgba(45, 138, 122, 0.3);
  background: rgba(45, 138, 122, 0.1);
  color: var(--color-success);
}

.route-section__hint {
  font-size: 11px;
  color: var(--color-text-muted);
  align-self: center;
}
</style>
