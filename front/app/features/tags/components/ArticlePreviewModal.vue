<script setup lang="ts">
import { computed } from 'vue'
import { ArticleContentView } from '~/features/articles/public'
import type { Article } from '~/types'

const props = defineProps<{
  visible: boolean
  selectedPreviewArticle: Article | null
  previewArticles: Article[]
  loadingPreviewArticle: boolean
}>()

const emit = defineEmits<{
  close: []
  navigate: [article: Article]
  favorite: [articleId: string]
  'article-update': [articleId: string, updates: Partial<Article>]
}>()

const show = computed({
  get: () => props.visible,
  set: (val: boolean) => { if (!val) emit('close') }
})
</script>

<template>
  <AppDialog v-model="show" width="90vw" :close-on-overlay="true" :close-on-escape="true">
    <template #header>
      <p class="preview-header-text">
        {{ loadingPreviewArticle ? '正在准备文章预览...' : '文章预览' }}
      </p>
    </template>
    <div class="preview-body">
      <ArticleContentView
        v-if="selectedPreviewArticle"
        :article="selectedPreviewArticle"
        :articles="previewArticles"
        @navigate="(a: Article) => emit('navigate', a)"
        @favorite="(id: string) => emit('favorite', id)"
        @article-update="(id: string, updates: Partial<Article>) => emit('article-update', id, updates)"
      />
    </div>
  </AppDialog>
</template>

<style scoped>
.preview-header-text {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.preview-body {
  min-height: 0;
  flex: 1;
}
</style>
