<script setup lang="ts">
import type { PendingArticle, TimelineAggregationArticle } from '~/types/timeline'

interface ArticleCard {
  id: number | string
  title: string
  feedName?: string
  feedIcon?: string
  pubDate?: string | Date
}

interface Props {
  article: ArticleCard
  /** Extra context line below the article */
  context?: string
  /** Note/badge line at the bottom */
  note?: string
  noteSoft?: boolean
}

withDefaults(defineProps<Props>(), {
  context: '',
  note: '',
  noteSoft: false,
})

const emit = defineEmits<{
  click: [articleId: number | string]
}>()
</script>

<template>
  <button
    type="button"
    class="topic-related-card"
    @click="emit('click', article.id)"
  >
    <p class="topic-related-card__meta">
      <FeedIcon v-if="article.feedIcon" :icon="article.feedIcon" :size="12" />
      {{ article.feedName || '来源文章' }}
    </p>
    <h3 class="topic-related-card__title">{{ article.title }}</h3>
    <p v-if="context" class="topic-related-card__context">{{ context }}</p>
    <p v-if="note" class="topic-related-card__note" :class="{ 'topic-related-card__note--soft': noteSoft }">
      {{ note }}
    </p>
  </button>
</template>

<style scoped>
.topic-related-card {
  position: relative;
  width: 100%;
  text-align: left;
  display: block;
  border-radius: 1.25rem;
  border: 1px solid var(--color-border-subtle);
  background: linear-gradient(180deg, var(--color-bg-elevated), var(--color-bg-sunken));
  padding: 1rem 1rem 1.05rem;
  text-decoration: none;
  box-shadow:
    inset 0 1px 0 var(--color-border-subtle),
    0 16px 40px rgba(3, 8, 14, 0.28);
  transition:
    transform 0.22s ease,
    border-color 0.22s ease,
    background 0.22s ease,
    box-shadow 0.22s ease;
  cursor: pointer;
}
.topic-related-card::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  border-radius: 999px;
  background: linear-gradient(180deg, var(--color-accent), rgba(92, 143, 226, 0.52));
  opacity: 0.7;
}
.topic-related-card:hover,
.topic-related-card:focus-visible {
  transform: translateY(-2px);
  border-color: var(--color-accent);
  background: linear-gradient(180deg, var(--color-bg-elevated), var(--color-bg-sunken));
  box-shadow:
    inset 0 1px 0 var(--color-border-subtle),
    0 24px 48px rgba(3, 8, 14, 0.36);
}
.topic-related-card:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
.topic-related-card__meta,
.topic-related-card__context {
  font-size: 0.74rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
}
.topic-related-card__title {
  margin-top: 0.45rem;
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.45;
  color: var(--color-text-primary);
}
.topic-related-card__context {
  margin-top: 0.65rem;
}
.topic-related-card__note {
  margin-top: 0.55rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--color-text-primary);
}
.topic-related-card__note--soft {
  color: var(--color-text-muted);
}
</style>
