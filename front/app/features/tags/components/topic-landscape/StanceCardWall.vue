<script setup lang="ts">
/**
 * 态势分区卡片墙（design §5 + spec「态势分区卡片墙」）。
 *
 * 规则：
 *  - 按 stance 分组，顺序 active → stalled → emerging → pending → archived。
 *  - 空分组不渲染。
 *  - 每分组内话题按 hit_count DESC 排序。
 *  - 已归档分组默认折叠，带计数（用户可展开）。
 */
import { computed, ref } from 'vue'
import type { TopicLandscapeTopic, TopicStance } from '~/api/semanticBoards'
import TopicStanceCard from './TopicStanceCard.vue'
import { STANCE_ORDER } from './chart-options'

const props = defineProps<{
  topics: TopicLandscapeTopic[]
}>()

const emit = defineEmits<{
  selectTopic: [topicId: number]
}>()

const STANCE_LABEL: Record<TopicStance, string> = {
  emerging: '🌱 新冒头',
  pending: '🔴 待激活',
  active: '🟢 活跃',
  stalled: '⏸️ 停滞',
  archived: '⬛ 已归档',
}

interface StanceGroup {
  stance: TopicStance
  label: string
  topics: TopicLandscapeTopic[]
}

/** 按 stance 分组 + 组内 hit_count DESC + 过滤空组，按 STANCE_ORDER 排序。 */
const groups = computed<StanceGroup[]>(() => {
  const byStance = new Map<TopicStance, TopicLandscapeTopic[]>()
  for (const t of props.topics) {
    const arr = byStance.get(t.stance)
    if (arr) arr.push(t)
    else byStance.set(t.stance, [t])
  }
  const out: StanceGroup[] = []
  for (const s of STANCE_ORDER) {
    const arr = byStance.get(s)
    if (!arr || arr.length === 0) continue
    arr.sort((a, b) => b.hit_count - a.hit_count)
    out.push({ stance: s, label: STANCE_LABEL[s], topics: arr })
  }
  return out
})

const archivedOpen = ref(false)
const archivedCount = computed(() => props.topics.filter((t) => t.stance === 'archived').length)

/** details toggle 同步 archivedOpen：折叠时 v-if 卸载卡片墙 → 图表不初始化（design D4 懒挂载）。 */
function onArchivedToggle(e: Event) {
  archivedOpen.value = (e.target as HTMLDetailsElement).open
}
</script>

<template>
  <div class="scw">
    <div v-for="g in groups" :key="g.stance" class="scw-group">
      <!-- 已归档：折叠（details，默认 close，带计数） -->
      <details v-if="g.stance === 'archived'" class="scw-fold" @toggle="onArchivedToggle">
        <summary class="scw-group-head scw-group-head--fold">
          <span class="scw-group-label">{{ g.label }}</span>
          <span class="scw-group-count">{{ archivedCount }}</span>
        </summary>
        <div v-if="archivedOpen" class="scw-grid">
          <TopicStanceCard
            v-for="t in g.topics"
            :key="t.id"
            :topic="t"
            @select-topic="(id) => emit('selectTopic', id)"
          />
        </div>
      </details>

      <!-- 常规分组：标题 + 卡片网格 -->
      <template v-else>
        <div class="scw-group-head">
          <span class="scw-group-label">{{ g.label }}</span>
          <span class="scw-group-count">{{ g.topics.length }}</span>
        </div>
        <div class="scw-grid">
          <TopicStanceCard
            v-for="t in g.topics"
            :key="t.id"
            :topic="t"
            @select-topic="(id) => emit('selectTopic', id)"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.scw {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.scw-group {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.scw-group-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.scw-group-label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.scw-group-count {
  font-size: 0.62rem;
  color: var(--color-text-muted);
  background: var(--color-bg-sunken);
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
}

/* 折叠组（已归档） */
.scw-fold > summary {
  cursor: pointer;
  list-style: none;
  user-select: none;
}

.scw-fold > summary::-webkit-details-marker {
  display: none;
}

.scw-group-head--fold::before {
  content: '▸ ';
  color: var(--color-text-muted);
  font-size: 0.7rem;
}

.scw-fold[open] .scw-group-head--fold::before {
  content: '▾ ';
}

.scw-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.5rem;
}
</style>
