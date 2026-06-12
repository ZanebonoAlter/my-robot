<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { TopicCategory } from '~/api/topicGraph'
import type { TimelineAggregationMode } from '~/types/timeline'

interface TopicInfo {
  slug: string
  label: string
  category: TopicCategory
  description?: string
}

interface Props {
  topic: TopicInfo | null
  totalCount: number
  aggregationMode: TimelineAggregationMode
}

defineProps<Props>()
const emit = defineEmits<{
  'update:aggregationMode': [value: TimelineAggregationMode]
}>()

const categoryLabels: Record<TopicCategory, string> = {
  event: '事件',
  person: '人物',
  keyword: '关键词',
}
</script>

<template>
  <header class="timeline-header">
    <div class="timeline-header__topic">
      <template v-if="topic">
        <div class="timeline-header__main">
          <h2 class="timeline-header__title">{{ topic.label }}</h2>
          <span class="timeline-header__category" :class="`timeline-header__category--${topic.category}`">
            {{ categoryLabels[topic.category] }}
          </span>
          <span class="timeline-header__count">
            <Icon icon="mdi:file-document-outline" width="14" />
            {{ totalCount }} 篇日报
          </span>
        </div>
        <div class="timeline-header__agg-toggle">
          <button
            class="timeline-header__agg-btn"
            :class="{ 'timeline-header__agg-btn--active': aggregationMode === 'day' }"
            @click="emit('update:aggregationMode', 'day')"
          >
            <Icon icon="mdi:calendar-today-outline" width="14" />
            按天
          </button>
          <button
            class="timeline-header__agg-btn"
            :class="{ 'timeline-header__agg-btn--active': aggregationMode === 'hour' }"
            @click="emit('update:aggregationMode', 'hour')"
          >
            <Icon icon="mdi:clock-outline" width="14" />
            按小时
          </button>
        </div>
        <p v-if="topic.description" class="timeline-header__description">{{ topic.description }}</p>
      </template>
      <template v-else>
        <h2 class="timeline-header__title timeline-header__title--placeholder">选择题材查看日报</h2>
      </template>
    </div>
  </header>
</template>

<style scoped>
.timeline-header {
  padding: 1rem 1.25rem;
  border-radius: 1.25rem;
  border: 1px solid var(--color-border-subtle);
  background: linear-gradient(180deg, rgba(20, 30, 42, 0.9), rgba(12, 18, 26, 0.95));
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.timeline-header__topic {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.timeline-header__main {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.timeline-header__title {
  font-size: 1.35rem;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1.3;
}

.timeline-header__title--placeholder {
  color: var(--color-text-muted);
  font-weight: 500;
}

.timeline-header__category {
  font-size: 0.7rem;
  padding: 0.22rem 0.55rem;
  border-radius: 999px;
  font-weight: 600;
  letter-spacing: 0.05em;
}

.timeline-header__category--event {
  background: var(--color-tag-event-bg);
  border: 1px solid var(--color-tag-event-border);
  color: rgba(252, 211, 77, 0.9);
}

.timeline-header__category--person {
  background: var(--color-tag-person-bg);
  border: 1px solid var(--color-tag-person-border);
  color: rgba(110, 231, 183, 0.9);
}

.timeline-header__category--keyword {
  background: var(--color-tag-keyword-bg);
  border: 1px solid var(--color-tag-keyword-border);
  color: rgba(165, 180, 252, 0.9);
}

.timeline-header__count {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.8rem;
  color: var(--color-text-secondary);
  padding: 0.3rem 0.65rem;
  border-radius: 999px;
  background: var(--color-bg-hover);
  border: 1px solid var(--color-border-subtle);
}

.timeline-header__description {
  font-size: 0.82rem;
  line-height: 1.5;
  color: var(--color-text-muted);
  margin: 0;
  padding-left: 0.1rem;
}

.timeline-header__agg-toggle {
  display: inline-flex;
  gap: 0.5rem;
  align-items: center;
}

.timeline-header__agg-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.75rem;
  padding: 0.28rem 0.65rem;
  border-radius: 999px;
  border: 1px solid var(--color-border-medium);
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.2s ease;
  font-weight: 500;

  &:hover {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }

  &--active {
    background: var(--color-accent-subtle);
    border-color: var(--color-accent);
    color: var(--color-text-primary);
  }
}

@media (max-width: 640px) {
  .timeline-header {
    padding: 0.85rem 1rem;
  }

  .timeline-header__title {
    font-size: 1.15rem;
  }

  .timeline-header__description {
    font-size: 0.78rem;
  }
}
</style>