<script setup lang="ts">
import ArticleListPanelView from '~/features/shell/components/ArticleListPanelView.vue'
import type { Article } from '~/types'

defineProps<{
  articles: Article[]
  selectedCategory?: string | null
  selectedFeed?: string | null
  selectedArticle?: Article | null
  loading?: boolean
  hasMore?: boolean
  total?: number
  startDate?: string
  endDate?: string
}>()
</script>

<template>
  <ArticleListPanelView v-bind="{ ...$props, ...$attrs }" />
</template>
