<script setup lang="ts">
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import type { DailyReportSection } from '~/api/dailyReports'
import type { TopicWatchHit } from '~/api/topicWatches'

const props = defineProps<{
  hits: TopicWatchHit[]
  sections: Array<Pick<DailyReportSection, 'id' | 'cluster_label'>>
}>()

const emit = defineEmits<{
  locate: [sectionId: number]
}>()

const sectionTitleById = computed(() => new Map(
  props.sections.map(section => [String(section.id), section.cluster_label || '未命名动态']),
))

function hitType(hit: TopicWatchHit): 'label' | 'keyword' {
  return hit.watchType === 'keyword' ? 'keyword' : 'label'
}

function hitLabel(hit: TopicWatchHit): string {
  return hit.watchLabel || '未命名关注'
}

const keywordHits = computed(() => props.hits.filter(hit => hitType(hit) === 'keyword'))
const topicHits = computed(() => props.hits.filter(hit => hitType(hit) === 'label'))

function sectionTitle(hit: TopicWatchHit): string {
  return sectionTitleById.value.get(String(hit.sectionId)) || '（已归档动态）'
}
</script>

<template>
  <section
    v-if="keywordHits.length || topicHits.length"
    class="dri-watch-index"
    data-testid="watch-index"
    aria-label="日报追踪索引"
  >
    <section v-if="keywordHits.length" class="dri-watch-index__group" data-testid="watch-index-keywords">
      <header class="dri-watch-index__header">
        <Icon icon="mdi:pound" width="15" aria-hidden="true" />
        <h2>追踪关键字</h2>
      </header>
      <button
        v-for="hit in keywordHits"
        :key="hit.id"
        type="button"
        class="dri-watch-index__item"
        data-testid="watch-index-item"
        :data-section-id="hit.sectionId"
        @click="emit('locate', Number(hit.sectionId))"
      >
        <Icon icon="mdi:pound" width="14" aria-hidden="true" />
        <span class="dri-watch-index__watch">{{ hitLabel(hit) }}</span>
        <span class="dri-watch-index__section">{{ sectionTitle(hit) }}</span>
        <Icon icon="mdi:chevron-right" width="14" aria-hidden="true" />
      </button>
    </section>

    <section v-if="topicHits.length" class="dri-watch-index__group" data-testid="watch-index-topics">
      <header class="dri-watch-index__header">
        <Icon icon="mdi:star-four-points" width="15" aria-hidden="true" />
        <h2>追踪话题</h2>
      </header>
      <button
        v-for="hit in topicHits"
        :key="hit.id"
        type="button"
        class="dri-watch-index__item"
        data-testid="watch-index-item"
        :data-section-id="hit.sectionId"
        @click="emit('locate', Number(hit.sectionId))"
      >
        <Icon icon="mdi:star-four-points" width="14" aria-hidden="true" />
        <span class="dri-watch-index__watch">{{ hitLabel(hit) }}</span>
        <span class="dri-watch-index__section">{{ sectionTitle(hit) }}</span>
        <Icon icon="mdi:chevron-right" width="14" aria-hidden="true" />
      </button>
    </section>
  </section>
</template>

<style scoped>
.dri-watch-index {
  display: grid;
  gap: 1.35rem;
  width: 100%;
  padding: 0 0 1.5rem;
  color: var(--color-text-primary);
}

.dri-watch-index__group {
  display: grid;
  gap: 0.25rem;
  padding-bottom: 0.8rem;
  border-bottom: 1px solid var(--color-border-subtle);
}

.dri-watch-index__header {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding-bottom: 0.35rem;
  color: var(--color-text-muted);
}

.dri-watch-index__header h2 {
  margin: 0;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.dri-watch-index__item {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  min-width: 0;
  padding: 0.45rem 0.2rem;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 0.76rem;
  text-align: left;
  cursor: pointer;
}

.dri-watch-index__item:hover,
.dri-watch-index__item:focus-visible {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
  outline: none;
}

.dri-watch-index__item > .iconify,
.dri-watch-index__header > .iconify {
  flex: 0 0 auto;
  color: var(--color-accent);
}

.dri-watch-index__watch {
  min-width: 0;
  max-width: 15rem;
  overflow: hidden;
  color: var(--color-text-primary);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dri-watch-index__section {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 600px) {
  .dri-watch-index__item {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .dri-watch-index__watch {
    max-width: none;
  }

  .dri-watch-index__section {
    grid-column: 2 / -1;
  }
}
</style>
