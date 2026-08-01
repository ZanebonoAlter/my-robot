<script setup lang="ts">
/**
 * 单张话题态势卡片（design §5）。
 *
 * 展示：label + stance 图标 + 关键数字 + mini-lifeline + 强吸引角标。
 * 特殊态：
 *  - pending（待激活）：红色描边 + 角标「待激活」，hover 提示去话题管理激活。
 *  - is_vacuum=true：🌀 强吸引角标，附 vacuum_strong 数值，tooltip 解释语义。
 *
 * 卡片 click → 父层跳「话题总览」并选中该 topic（本组件只 emit，不直接跳转）。
 */
import { computed } from 'vue'
import type { TopicLandscapeTopic, TopicStance } from '~/api/semanticBoards'
import MiniLifelineChart from './MiniLifelineChart.vue'

const props = defineProps<{
  topic: TopicLandscapeTopic
}>()

const emit = defineEmits<{
  selectTopic: [topicId: number]
}>()

/** stance → 图标 + 中文（design §2 统一）。 */
const STANCE_META: Record<TopicStance, { icon: string; label: string }> = {
  emerging: { icon: '🌱', label: '新冒头' },
  pending: { icon: '🔴', label: '待激活' },
  active: { icon: '🟢', label: '活跃' },
  stalled: { icon: '⏸️', label: '停滞' },
  archived: { icon: '⬛', label: '已归档' },
}

const stanceMeta = computed(() => STANCE_META[props.topic.stance] ?? { icon: '•', label: props.topic.stance })
const isPending = computed(() => props.topic.stance === 'pending')

/** 关键数字行：按态势语义选取文案（活跃连续命中 / 停滞 N 天未命中 / 其余命中数）。 */
const keyLine = computed(() => {
  const t = props.topic
  switch (t.stance) {
    case 'active':
      return `连续 ${t.consecutive_hits} · 命中 ${t.hit_count}`
    case 'stalled':
      return `${t.days_since_last} 天未命中 · 累计 ${t.hit_count}`
    case 'pending':
      return `命中 ${t.hit_count} · 够格转正`
    case 'emerging':
      return `命中 ${t.hit_count}`
    case 'archived':
      return `累计 ${t.hit_count} · 已归档`
    default:
      return `命中 ${t.hit_count}`
  }
})

const lastSeenLabel = computed(() =>
  props.topic.last_seen_date ? `最近命中 ${props.topic.last_seen_date}` : '',
)

function onClick() {
  emit('selectTopic', props.topic.id)
}
</script>

<template>
  <button
    type="button"
    class="tsc"
    :class="{ 'tsc--pending': isPending }"
    :title="isPending ? '够格转正，去话题管理激活' : `查看「${topic.label}」话题总览`"
    @click="onClick"
  >
    <!-- 待激活角标 -->
    <span v-if="isPending" class="tsc-badge">待激活</span>
    <!-- 强吸引角标（与主态势正交） -->
    <span
      v-if="topic.is_vacuum"
      class="tsc-vacuum"
      title="强吸引：该话题近期吸走大量 section"
    >🌀 {{ topic.vacuum_strong }}</span>

    <div class="tsc-head">
      <span class="tsc-icon">{{ stanceMeta.icon }}</span>
      <span class="tsc-label" :title="topic.label">{{ topic.label }}</span>
    </div>

    <div class="tsc-key">{{ keyLine }}</div>

    <!-- emerging 卡片不渲染节奏图（节奏信息由总览气泡图承载，design D4） -->
    <MiniLifelineChart v-if="topic.stance !== 'emerging'" :points="topic.lifeline" />

    <div v-if="lastSeenLabel" class="tsc-foot">{{ lastSeenLabel }}</div>
  </button>
</template>

<style scoped>
.tsc {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.55rem 0.6rem;
  border-radius: 10px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.12s ease, background 0.12s ease, transform 0.1s ease;
  font-family: inherit;
  width: 100%;
  box-sizing: border-box;
}

.tsc:hover {
  border-color: var(--color-accent);
  background: var(--color-bg-hover);
}

.tsc:active {
  transform: translateY(1px);
}

/* 待激活：红色描边突出 */
.tsc--pending {
  border-color: rgba(239, 68, 68, 0.7);
  box-shadow: 0 0 0 1px rgba(239, 68, 68, 0.25) inset;
}

.tsc--pending:hover {
  border-color: rgba(239, 68, 68, 0.9);
}

.tsc-head {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.tsc-icon {
  font-size: 0.85rem;
  flex: 0 0 auto;
}

.tsc-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.tsc-key {
  font-size: 0.68rem;
  color: var(--color-text-secondary);
  line-height: 1.3;
}

.tsc-foot {
  font-size: 0.6rem;
  color: var(--color-text-muted);
}

.tsc-badge {
  position: absolute;
  top: -7px;
  right: 8px;
  font-size: 0.58rem;
  font-weight: 700;
  color: #fff;
  background: rgba(239, 68, 68, 0.9);
  padding: 1px 6px;
  border-radius: 999px;
  letter-spacing: 0.02em;
}

.tsc-vacuum {
  position: absolute;
  top: -7px;
  left: 8px;
  font-size: 0.6rem;
  font-weight: 600;
  color: var(--color-warning);
  background: var(--color-warning-subtle);
  padding: 1px 6px;
  border-radius: 999px;
  line-height: 1.4;
}
</style>
