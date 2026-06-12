<script setup lang="ts">
import { computed } from 'vue'

export interface Keyword {
  slug: string
  label: string
  count: number
  relevance: number // 0-1
}

interface Props {
  keywords: Keyword[]
  selectedKeyword?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  selectedKeyword: null,
})

const emit = defineEmits<{
  'select': [keyword: Keyword]
}>()

// Compute keyword font size and opacity based on relevance
const processedKeywords = computed(() => {
  return props.keywords.map(keyword => {
    // Font size range: 12px - 28px
    const fontSize = 12 + keyword.relevance * 16

    // Opacity (lower relevance = more transparent)
    const opacity = 0.5 + keyword.relevance * 0.5

    return {
      ...keyword,
      fontSize,
      opacity,
    }
  }).sort((a, b) => b.relevance - a.relevance) // Sort by relevance
})

function handleClick(keyword: Keyword) {
  emit('select', keyword)
}
</script>

<template>
  <div class="keyword-cloud" data-testid="keyword-cloud">
    <button
      v-for="keyword in processedKeywords"
      :key="keyword.slug"
      class="keyword-tag"
      type="button"
      data-testid="keyword-item"
      :class="{
        'keyword-tag--selected': selectedKeyword === keyword.slug,
      }"
      :style="{
        fontSize: `${keyword.fontSize}px`,
        opacity: selectedKeyword === keyword.slug ? 1 : keyword.opacity,
      }"
      @click="handleClick(keyword)"
    >
      <span class="keyword-label">{{ keyword.label }}</span>
      <span v-if="keyword.count > 0" class="keyword-count">
        {{ keyword.count }}
      </span>
    </button>
  </div>
</template>

<style scoped>
.keyword-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  padding: 0.2rem 0;
  justify-content: flex-start;
}

.keyword-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  min-height: 2.25rem;
  border-radius: 9999px;
  border: 1px solid var(--color-border-medium);
  background: linear-gradient(180deg, var(--color-bg-sunken), var(--color-bg-base));
  color: var(--color-text-primary);
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  font-weight: 500;
  padding: 0.45rem 0.85rem;
  text-align: left;
  box-shadow: inset 0 1px 0 var(--color-border-subtle);
}

.keyword-tag:hover {
  border-color: var(--color-accent);
  background: linear-gradient(180deg, var(--color-bg-elevated), var(--color-bg-sunken));
  transform: translateY(-1px);
}

.keyword-tag:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

.keyword-tag--selected {
  background: linear-gradient(180deg, var(--color-accent-subtle), rgba(78, 132, 211, 0.14)) !important;
  border-color: var(--color-accent) !important;
  color: rgba(255, 239, 230, 0.98) !important;
  opacity: 1 !important;
  box-shadow: 0 12px 28px rgba(3, 8, 14, 0.22);
}

.keyword-label {
  font-size: 0.82rem;
  line-height: 1.2;
}

.keyword-count {
  font-size: 0.7em;
  opacity: 0.78;
  background: var(--color-bg-hover);
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
}
</style>
