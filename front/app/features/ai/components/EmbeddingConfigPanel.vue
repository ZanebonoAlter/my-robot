<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useEmbeddingConfigApi, type EmbeddingConfigItem } from '~/api'

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const configs = ref<EmbeddingConfigItem[]>([])
const editValues = ref<Record<string, string>>({})
const dirtyKeys = ref<Set<string>>(new Set())

const configLabels: Record<string, { label: string; hint: string; unit?: string }> = {
  high_similarity_threshold: { label: '高相似度阈值', hint: '标签自动复用的最低相似度，高于此值直接复用', unit: '0.0-1.0' },
  low_similarity_threshold: { label: '低相似度阈值', hint: '低于此值创建新标签，中间地带也创建新标签', unit: '0.0-1.0' },
  embedding_dimension: { label: 'Embedding 维度', hint: '实际维度由模型自动决定，此值仅供参考（如 nomic-embed=768, 3-small=1536）', unit: '' },
}

function getLabel(key: string): string {
  return configLabels[key]?.label || key
}

function getHint(key: string): string {
  return configLabels[key]?.hint || ''
}

function getUnit(key: string): string {
  return configLabels[key]?.unit || ''
}

function markDirty(key: string) {
  dirtyKeys.value.add(key)
}

function pushMessage(kind: 'success' | 'error', message: string) {
  if (kind === 'success') {
    success.value = message
    error.value = null
    setTimeout(() => { success.value = null }, 2500)
  } else {
    error.value = message
    success.value = null
  }
}

async function loadConfig() {
  loading.value = true
  error.value = null
  try {
    const api = useEmbeddingConfigApi()
    const response = await api.getConfig()
    if (!response.success || !response.data) {
      throw new Error(response.error || '加载 embedding 配置失败')
    }
    configs.value = response.data.filter(c => configLabels[c.key])
    const values: Record<string, string> = {}
    for (const item of configs.value) {
      values[item.key] = item.value
    }
    editValues.value = values
    dirtyKeys.value.clear()
  } catch (err) {
    pushMessage('error', err instanceof Error ? err.message : '加载失败')
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  if (dirtyKeys.value.size === 0) {
    pushMessage('success', '没有修改')
    return
  }

  for (const key of dirtyKeys.value) {
    if (key.endsWith('_threshold')) {
      const val = parseFloat(editValues.value[key] || '')
      if (isNaN(val) || val < 0 || val > 1) {
        pushMessage('error', `${getLabel(key)} 必须是 0.0-1.0 之间的数字`)
        return
      }
    }
  }

  saving.value = true
  error.value = null
  try {
    const api = useEmbeddingConfigApi()
    for (const key of dirtyKeys.value) {
      const response = await api.updateConfig(key, editValues.value[key] || '')
      if (!response.success) {
        throw new Error(response.error || `保存 ${getLabel(key)} 失败`)
      }
    }
    dirtyKeys.value.clear()
    await loadConfig()
    pushMessage('success', 'Embedding 配置已保存')
  } catch (err) {
    pushMessage('error', err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <AppSectionHeader title="Embedding 配置" description="向量搜索阈值与维度，模型在 AI 路由中配置" icon-name="mdi:vector-square" />
      <AppButton
        variant="primary"
        size="sm"
        :disabled="saving || dirtyKeys.size === 0"
        @click="saveConfig"
      >
        {{ saving ? '保存中...' : dirtyKeys.size > 0 ? `保存 (${dirtyKeys.size})` : '已保存' }}
      </AppButton>
    </div>

    <div v-if="loading" class="py-8 flex justify-center">
      <Icon icon="mdi:loading" width="28" height="28" class="animate-spin" style="color: var(--color-accent)" />
    </div>

    <div v-else class="space-y-3">
      <div v-for="config in configs" :key="config.key" class="embedding-config-card">
        <div class="flex items-center justify-between gap-4 mb-1.5">
          <label class="embedding-config-label">{{ getLabel(config.key) }}</label>
          <span v-if="getUnit(config.key)" class="embedding-config-unit">{{ getUnit(config.key) }}</span>
        </div>
        <AppInput
          v-model="editValues[config.key]"
          type="text"
          @input="markDirty(config.key)"
        />
        <p class="embedding-config-hint">{{ getHint(config.key) }}</p>
      </div>
    </div>

    <div v-if="success" class="ai-msg ai-msg--success">
      {{ success }}
    </div>
    <div v-if="error" class="ai-msg ai-msg--error">
      {{ error }}
    </div>
  </div>
</template>

<style scoped>
.embedding-config-card {
  border-radius: 8px;
  border: 1px solid var(--color-border-subtle);
  padding: 16px;
  background: var(--color-bg-sunken);
}

.embedding-config-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
}

.embedding-config-unit {
  font-size: 11px;
  color: var(--color-text-muted);
}

.embedding-config-hint {
  margin-top: 4px;
  font-size: 11px;
  color: var(--color-text-muted);
}

.ai-msg {
  border-radius: 8px;
  padding: 12px 16px;
  font-size: 13px;
}

.ai-msg--success {
  background: rgba(61, 138, 74, 0.1);
  border: 1px solid rgba(61, 138, 74, 0.25);
  color: var(--color-success);
}

.ai-msg--error {
  background: rgba(196, 47, 60, 0.1);
  border: 1px solid rgba(196, 47, 60, 0.25);
  color: var(--color-error);
}
</style>
