<script setup lang="ts">
import type { PendingArticle, TimelineAggregationArticle } from '~/types/timeline'

interface ArticleCard {
  id: number | string
  title: string
  feedName?: string
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
    <p class="topic-related-card__meta">{{ article.feedName || '来源文章' }}</p>
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
  border: 1px solid var(--topic-border, rgba(123, 154, 192, 0.18));
  background: linear-gradient(180deg, rgba(18, 27, 38, 0.96), rgba(10, 16, 24, 0.98));
  padding: 1rem 1rem 1.05rem;
  text-decoration: none;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
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
  background: linear-gradient(180deg, rgba(240, 138, 75, 0.9), rgba(92, 143, 226, 0.52));
  opacity: 0.7;
}
.topic-related-card:hover,
.topic-related-card:focus-visible {
  transform: translateY(-2px);
  border-color: rgba(240, 138, 75, 0.36);
  background: linear-gradient(180deg, rgba(24, 35, 48, 0.98), rgba(12, 19, 28, 1));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 24px 48px rgba(3, 8, 14, 0.36);
}
.topic-related-card:focus-visible {
  outline: 2px solid rgba(240, 138, 75, 0.45);
  outline-offset: 2px;
}
.topic-related-card__meta,
.topic-related-card__context {
  font-size: 0.74rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--topic-ink-soft, rgba(148, 168, 188, 0.7));
}
.topic-related-card__title {
  margin-top: 0.45rem;
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.45;
  color: var(--topic-ink-strong, rgba(248, 251, 255, 0.96));
}
.topic-related-card__context {
  margin-top: 0.65rem;
}
.topic-related-card__note {
  margin-top: 0.55rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: rgba(255, 227, 203, 0.86);
}
.topic-related-card__note--soft {
  color: rgba(173, 193, 214, 0.72);
}
</style>
