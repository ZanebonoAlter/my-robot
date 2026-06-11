<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { ArticleContentView } from '~/features/articles/public'
import type { Article } from '~/types'

defineProps<{
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
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="tags-article-modal"
      @click.self="emit('close')"
    >
      <div class="tags-article-modal__panel">
        <header class="tags-article-modal__header">
          <p class="truncate text-sm text-ink-medium">
            {{ loadingPreviewArticle ? '正在准备文章预览...' : '文章预览' }}
          </p>
          <button
            class="btn-ghost min-h-11 min-w-11 px-0"
            type="button"
            aria-label="关闭文章弹窗"
            @click="emit('close')"
          >
            <Icon icon="mdi:close" width="18" />
          </button>
        </header>
        <div class="tags-article-modal__body">
          <ArticleContentView
            v-if="selectedPreviewArticle"
            :article="selectedPreviewArticle"
            :articles="previewArticles"
            @navigate="(a: Article) => emit('navigate', a)"
            @favorite="(id: string) => emit('favorite', id)"
            @article-update="(id: string, updates: Partial<Article>) => emit('article-update', id, updates)"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.tags-article-modal { position: fixed; inset: 0; z-index: 210; display: flex; align-items: stretch; justify-content: center; background: rgba(8, 12, 18, 0.7); padding: 1rem; backdrop-filter: blur(10px); }
.tags-article-modal__panel { display: flex; height: calc(100vh - 2rem); width: min(1500px, 100%); flex-direction: column; overflow: hidden; border-radius: 1.75rem; background: rgba(255, 252, 248, 0.98); box-shadow: 0 30px 100px rgba(0, 0, 0, 0.28); }
.tags-article-modal__header { display: flex; align-items: center; justify-content: space-between; gap: 1rem; border-bottom: 1px solid rgba(20, 33, 44, 0.08); padding: 1rem 1.25rem; }
.tags-article-modal__body { min-height: 0; flex: 1; }
</style>
