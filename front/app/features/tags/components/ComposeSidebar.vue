<script setup lang="ts">
/**
 * 就地编排态右侧候选侧边栏（inline-compose-lane 切片②）.
 *
 * 纯展示组件：自然语言语义搜索框 + 候选话题引导区（连续命中候选）。
 * 所有逻辑由 useInlineCompose 提供（host 第③块装配），本组件只做 props→DOM/emit 接线。
 * 全 Layer 2 语义 token，跟随双主题；editorial 风格。
 *
 * 分组（spec「候选话题引导」「已中断候选单列分组」Scenario）：
 *  - 正在连续命中：brokenStreak===false（composable 已把 activatable 置顶）。
 *    每条渲染「确认启用」（disabled 当 !activatable）+「采纳」。
 *  - 已中断·近期未命中：brokenStreak===true，视觉弱化，标「近期未命中」而非「连续 0 天」，
 *    只渲染「采纳」，不渲染「确认启用」。
 *
 * 状态标记左对齐（interaction-conventions §1）：色点 + 状态文案置于标题左侧。
 * 无候选（items 空）时整个候选区隐藏（spec「无候选时侧边栏该区隐藏」）。
 *
 * 设计依据：section-lifecycle spec「编排态候选池语义搜索」「候选话题引导」Requirement。
 */
import { computed } from 'vue'
import AppInput from '~/components/ui/AppInput.vue'
import AppButton from '~/components/ui/AppButton.vue'
import type { SidebarCandidateItem } from '~/features/tags/composables/useInlineCompose'

interface Props {
  /** 候选话题列表（来自 composable.sidebarItems，已 activatable 置顶）。 */
  items: SidebarCandidateItem[]
  /** 搜索文本（v-model:queryText）。 */
  queryText: string
  /** 搜索降级提示（embedQuery 失败 / 空向量）。 */
  searchError: string | null
  /** 搜索进行中（debounce / embedQuery 进行中）。 */
  searching: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:queryText': [value: string]
  'activate': [topicId: number]
  'adopt': [item: SidebarCandidateItem]
}>()

/** 正在连续命中（brokenStreak===false），composable 已保证 activatable 在前。 */
const mainGroup = computed(() => props.items.filter(i => !i.brokenStreak))

/** 已中断·近期未命中（consecutive_hits===0）。 */
const brokenGroup = computed(() => props.items.filter(i => i.brokenStreak))

function onQuery(value: string | number): void {
  emit('update:queryText', String(value))
}
</script>

<template>
  <aside class="compose-sidebar" aria-label="编排态候选侧边栏">
    <!-- 顶部：自然语言语义搜索 -->
    <div class="cs-search">
      <div class="cs-search__field">
        <svg
          class="cs-search__icon"
          viewBox="0 0 24 24"
          width="16"
          height="16"
          aria-hidden="true"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="11" cy="11" r="7" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <AppInput
          class="cs-search__input"
          :model-value="queryText"
          placeholder="自然语言搜索未分类 section"
          @update:model-value="onQuery"
        />
      </div>
      <span v-if="searching" class="cs-search__loading">搜索中…</span>
    </div>
    <p v-if="searchError" class="cs-search__error" role="status">
      {{ searchError }}
    </p>

    <!-- 候选话题区（无候选整体隐藏） -->
    <section v-if="items.length > 0" class="cs-candidates">
      <!-- 正在连续命中 -->
      <div v-if="mainGroup.length > 0" class="cs-group">
        <h4 class="cs-group__title">正在连续命中</h4>
        <article
          v-for="item in mainGroup"
          :key="item.topic.id"
          class="cs-card"
          :class="{ 'is-activatable': item.activatable }"
        >
          <div class="cs-card__head">
            <span
              v-if="item.activatable"
              class="cs-card__flag cs-card__flag--ok"
              aria-label="可启用"
            >●</span>
            <span class="cs-card__label">{{ item.topic.label }}</span>
          </div>
          <p class="cs-card__meta">
            连续 {{ item.topic.consecutive_hits }} 天 · 含 {{ item.topic.section_count }} 条 section
          </p>
          <div class="cs-card__actions">
            <div class="cs-card__activate">
              <AppButton
                size="sm"
                :disabled="!item.activatable"
                @click="emit('activate', item.topic.id)"
              >
                确认启用
              </AppButton>
              <span v-if="!item.activatable" class="cs-card__hint">需先满足连续多天出现条件</span>
            </div>
            <AppButton size="sm" variant="secondary" @click="emit('adopt', item)">
              采纳
            </AppButton>
          </div>
        </article>
      </div>

      <!-- 已中断·近期未命中 -->
      <div v-if="brokenGroup.length > 0" class="cs-group cs-group--broken">
        <h4 class="cs-group__title">已中断·近期未命中</h4>
        <article
          v-for="item in brokenGroup"
          :key="item.topic.id"
          class="cs-card is-broken"
        >
          <div class="cs-card__head">
            <span
              class="cs-card__flag cs-card__flag--broken"
              aria-label="近期未命中"
            >○</span>
            <span class="cs-card__label">{{ item.topic.label }}</span>
            <span class="cs-card__streak">近期未命中</span>
          </div>
          <p class="cs-card__meta">含 {{ item.topic.section_count }} 条 section</p>
          <div class="cs-card__actions">
            <AppButton size="sm" variant="secondary" @click="emit('adopt', item)">
              采纳
            </AppButton>
          </div>
        </article>
      </div>
    </section>
  </aside>
</template>

<style scoped>
.compose-sidebar {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-subtle);
  border-radius: 14px;
  box-shadow: var(--shadow-strong);
  color: var(--color-text-primary);
  font-size: 14px;
}

/* ── 搜索区 ───────────────────────────────────────────────────────────── */
.cs-search {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cs-search__field {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cs-search__icon {
  flex: 0 0 auto;
  color: var(--color-text-muted);
}
.cs-search__input {
  flex: 1 1 auto;
  min-width: 0;
}
.cs-search__loading {
  font-size: 12px;
  color: var(--color-text-muted);
}
.cs-search__error {
  margin: 0;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--color-warning);
  background: var(--color-warning-subtle);
  border-radius: 6px;
}

/* ── 候选区 ───────────────────────────────────────────────────────────── */
.cs-candidates {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.cs-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.cs-group__title {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
  text-transform: none;
}
.cs-group--broken {
  opacity: 0.7;
}
.cs-group--broken .cs-group__title {
  color: var(--color-text-muted);
}

.cs-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--color-bg-hover);
  border: 1px solid var(--color-border-subtle);
}
.cs-card.is-activatable {
  border-color: var(--color-accent);
  background: var(--color-accent-subtle);
}
.cs-card.is-broken {
  background: var(--color-bg-sunken);
}

.cs-card__head {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cs-card__flag {
  font-size: 12px;
  line-height: 1;
}
.cs-card__flag--ok {
  color: var(--color-success);
}
.cs-card__flag--broken {
  color: var(--color-text-muted);
}
.cs-card__label {
  font-weight: 600;
  color: var(--color-text-primary);
}
.cs-card__streak {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-text-muted);
}
.cs-card__meta {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.cs-card__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.cs-card__activate {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cs-card__hint {
  font-size: 11px;
  color: var(--color-text-muted);
}
</style>
