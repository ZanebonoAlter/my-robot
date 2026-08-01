<script setup lang="ts">
/**
 * 就地编排态顶部浮工具条（inline-compose-lane 切片②）.
 *
 * 纯展示组件：泳道名输入 + 已勾计数（区分 unassigned / 移出）+ 聚类质量单卡
 * + 取消/保存。所有逻辑由 useInlineCompose 提供（host 第③块装配），本组件只做
 * props→DOM/emit 接线。全 Layer 2 语义 token，跟随双主题；editorial 风格浮条
 * （rounded + shadow + backdrop-blur）。
 *
 * 设计依据：section-lifecycle spec「手动建泳道编排态」「聚类质量单卡」Requirement。
 */
import { onMounted, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import AppInput from '~/components/ui/AppInput.vue'
import AppButton from '~/components/ui/AppButton.vue'

interface Props {
  /** 泳道名（v-model:laneName）。 */
  laneName: string
  /** 聚类质量：成员数（quality.memberCount）。 */
  memberCount: number
  /** 聚类质量：平均距离（quality.meanDistance，保留 3 位小数显示）。 */
  meanDistance: number
  /** 聚类质量：离群数（quality.outlierCount，>0 用警示色）。 */
  outlierCount: number
  /** 来源=unassigned 的已勾数（counts.unassigned）。 */
  unassignedCount: number
  /** 来源=active 泳道（将移出）的已勾数（counts.moveOut）。 */
  moveOutCount: number
  /** 保存进行中（禁用保存按钮 + loading 态）。 */
  saving: boolean
  /** 是否可保存（host 算好：laneName 非空 && memberCount>0）。 */
  canSave: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'update:laneName': [value: string]
  'save': []
  'cancel': []
}>()

const inputRef = ref<ComponentPublicInstance | null>(null)

// AppInput 无 autofocus 透传，挂载后手动聚焦内部 input（轻量展示行为，不改 AppInput）。
onMounted(() => {
  const root = (inputRef.value?.$el as HTMLElement | undefined) ?? null
  const input = root?.querySelector?.('input') as HTMLInputElement | null
  input?.focus?.()
})

function onLaneName(value: string | number): void {
  emit('update:laneName', String(value))
}
</script>

<template>
  <div class="compose-inline-toolbar" role="toolbar" aria-label="新建泳道编排工具条">
    <!-- 左：泳道名输入 -->
    <div class="cit-name">
      <AppInput
        ref="inputRef"
        :model-value="laneName"
        placeholder="新泳道名称"
        @update:model-value="onLaneName"
      />
    </div>

    <!-- 中：已勾计数 + 聚类质量单卡 -->
    <div class="cit-center">
      <div class="cit-count">
        <span class="cit-count__main">已勾 {{ unassignedCount + moveOutCount }} 条</span>
        <span
          v-if="moveOutCount > 0"
          class="cit-count__moveout"
        >{{ moveOutCount }} 条来自现有泳道·将移出</span>
      </div>
      <div class="cit-quality" aria-label="聚类质量">
        <span class="cit-quality__item">成员 {{ memberCount }}</span>
        <span class="cit-quality__item">平均距离 {{ meanDistance.toFixed(3) }}</span>
        <span
          class="cit-quality__item"
          :class="{ 'is-warning': outlierCount > 0 }"
        >离群 {{ outlierCount }}</span>
      </div>
    </div>

    <!-- 右：取消 + 保存 -->
    <div class="cit-actions">
      <AppButton variant="secondary" @click="emit('cancel')">
        取消
      </AppButton>
      <AppButton
        variant="primary"
        :disabled="!canSave || saving"
        :loading="saving"
        @click="emit('save')"
      >
        {{ moveOutCount > 0 ? `保存（含 ${moveOutCount} 条移出）` : '保存' }}
      </AppButton>
    </div>
  </div>
</template>

<style scoped>
.compose-inline-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 16px;
  background: var(--color-dialog-bg);
  backdrop-filter: blur(12px);
  border: 1px solid var(--color-border-subtle);
  border-radius: 14px;
  box-shadow: var(--shadow-strong);
  color: var(--color-text-primary);
  font-size: 14px;
}

.cit-name {
  flex: 0 1 220px;
  min-width: 160px;
}

.cit-center {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
  min-width: 0;
}

.cit-count {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.cit-count__main {
  font-weight: 600;
  color: var(--color-text-primary);
}
.cit-count__moveout {
  font-size: 12px;
  color: var(--color-warning);
}

.cit-quality {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  padding: 5px 10px;
  border-radius: 8px;
  background: var(--color-bg-hover);
  border: 1px solid var(--color-border-subtle);
}
.cit-quality__item {
  font-size: 13px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}
.cit-quality__item.is-warning {
  color: var(--color-warning);
  font-weight: 600;
}

.cit-actions {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
</style>
