<script setup lang="ts">
/**
 * AI 模型未就绪全局 banner。
 *
 * 显示条件：用户意图为运行（analysisPaused=false）但健康门判定 NOT 健康（aiHealthy=false）。
 * 用户主动暂停时不显示（已知暂停，无需再提示健康）。
 *
 * 约束（flow/scheduler.md 业务约束第 7 条）：本 banner 只做提示，不禁用/不改写
 * 暂停/启动按钮；按钮与 favicon 始终跟 analysisPaused 用户意图。
 */
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useSchedulerStatus } from '~/composables/useSchedulerStatus'
import { useHealthReprobe } from '~/composables/useHealthReprobe'

const { analysisPaused, aiHealthy, loadSchedulersStatus } = useSchedulerStatus()
const { reprobing, reprobeHealth } = useHealthReprobe()

const visible = computed(() => !analysisPaused.value && !aiHealthy.value)

async function handleReprobe() {
  await reprobeHealth()
  // Banner 可见性由 scheduler status 的 ai_healthy 驱动，重探后刷新以关闭横幅。
  await loadSchedulersStatus()
}
</script>

<template>
  <div v-if="visible" class="ai-health-banner" role="alert">
    <Icon icon="mdi:alert" width="16" height="16" class="shrink-0" />
    <span class="ai-health-banner__text">AI 模型未就绪（LLM/Embedding 未连通），分析暂停运行</span>
    <button type="button" class="ai-health-banner__action" :disabled="reprobing" @click="handleReprobe">
      {{ reprobing ? '检测中…' : '重新检测' }}
    </button>
    <NuxtLink to="/settings?section=ai-health" class="ai-health-banner__action">去配置</NuxtLink>
  </div>
</template>

<style scoped>
.ai-health-banner {
  position: fixed;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9998;
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: min(92vw, 640px);
  padding: 8px 14px;
  border-radius: 10px;
  font-size: 13px;
  background: var(--color-warning-bg, rgba(196, 136, 60, 0.12));
  border: 1px solid var(--color-warning-border, rgba(196, 136, 60, 0.35));
  color: var(--color-warning);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  backdrop-filter: blur(8px);
}

.ai-health-banner__text {
  flex: 1;
  min-width: 0;
}

.ai-health-banner__action {
  flex-shrink: 0;
  font-weight: 600;
  color: var(--color-link);
  text-decoration: underline;
  white-space: nowrap;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  font-size: 13px;
}

.ai-health-banner__action:disabled {
  opacity: 0.6;
  cursor: default;
}

.ai-health-banner__action:hover {
  opacity: 0.8;
}
</style>
