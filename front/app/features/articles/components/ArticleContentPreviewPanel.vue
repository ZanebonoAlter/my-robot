<script setup lang="ts">
import { Icon } from '@iconify/vue'
import dayjs from 'dayjs'
import type { Article, RssFeed } from '~/types'
import ArticleTagList from '~/features/articles/components/ArticleTagList.vue'
import { getStatusToneClasses } from '~/features/articles/composables/useArticleProcessingStatus'
import '~/components/article/ArticleContent.css'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  article: Article | null
  mergedArticle: Record<string, any> | null
  highlightedTagSlugs: string[]
  aiEnabled: boolean

  showProcessingPanel: boolean
  showFirecrawlStatus: boolean
  showSummaryStatus: boolean
  showManualFirecrawlAction: boolean
  showManualSummaryAction: boolean
  actionBusy: boolean
  manualFirecrawlLoading: boolean
  manualSummaryLoading: boolean
  manualFirecrawlLabel: string
  manualSummaryLabel: string
  manualActionError: string | null
  detailLines: string[]
  showContentSourceToggle: boolean
  activeContentSource: string | null
  renderedStoredSummary: string
  manualTaggingLoading: boolean
  manualTaggingLabel: string
  taggingError: string | null
  showDescription: boolean
  displayContent: string
  articleImageUrl: string | null
  articlePubDate: string
  articleAuthor: string | null
  articleRead: boolean
  articleTitleFull: string

  handleManualFirecrawl: () => void
  handleManualSummary: () => void
  handleManualTagging: () => void
  handleTagWatchToggle: (payload: { id: number; slug: string }) => void
  openOriginal: () => void
}>(), {
  article: null,
  mergedArticle: null,
  highlightedTagSlugs: () => [],
  aiEnabled: false,
  showProcessingPanel: false,
  showFirecrawlStatus: false,
  showSummaryStatus: false,
  showManualFirecrawlAction: false,
  showManualSummaryAction: false,
  actionBusy: false,
  manualFirecrawlLoading: false,
  manualSummaryLoading: false,
  manualFirecrawlLabel: '',
  manualSummaryLabel: '',
  manualActionError: null,
  detailLines: () => [],
  showContentSourceToggle: false,
  activeContentSource: null,
  renderedStoredSummary: '',
  manualTaggingLoading: false,
  manualTaggingLabel: '',
  taggingError: null,
  showDescription: false,
  displayContent: '',
  articleImageUrl: null,
  articlePubDate: '',
  articleAuthor: null,
  articleRead: false,
  articleTitleFull: '',
  handleManualFirecrawl: () => {},
  handleManualSummary: () => {},
  handleManualTagging: () => {},
  handleTagWatchToggle: () => {},
  openOriginal: () => {},
})

const emit = defineEmits<{
  'update:activeContentSource': [source: string]
}>()
</script>

<template>
  <!-- Processing Panel -->
  <div
    v-if="showProcessingPanel && mergedArticle"
    class="mb-6 rounded-2xl border border-ink-200 bg-white/80 p-4 shadow-subtle"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="flex flex-wrap items-center gap-2">
        <span
          v-if="showFirecrawlStatus"
          class="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-semibold"
          :class="getStatusToneClasses(mergedArticle.firecrawlStatus === 'completed' ? 'success' : 'info')"
        >
          <Icon icon="mdi:web-sync" width="14" height="14" :class="{ 'animate-spin': mergedArticle.firecrawlStatus === 'processing' }" />
          {{ mergedArticle.firecrawlStatus === 'completed' ? '全文已抓取' : mergedArticle.firecrawlStatus === 'processing' ? '正在抓取全文...' : mergedArticle.firecrawlStatus === 'failed' ? '全文抓取失败' : '等待中' }}
        </span>
        <span
          v-if="showSummaryStatus"
          class="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-semibold"
          :class="getStatusToneClasses(mergedArticle.summaryStatus === 'completed' ? 'success' : 'info')"
        >
          <Icon icon="mdi:brain" width="14" height="14" :class="{ 'animate-spin': mergedArticle.summaryStatus === 'pending' }" />
          {{ mergedArticle.summaryStatus === 'completed' ? 'AI 总结完成' : mergedArticle.summaryStatus === 'pending' ? '正在生成总结...' : mergedArticle.summaryStatus === 'failed' ? 'AI 总结失败' : '等待中' }}
        </span>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          v-if="showManualFirecrawlAction"
          class="inline-flex items-center gap-1.5 rounded-full border border-ink-200 bg-white px-3 py-1.5 text-xs font-semibold text-ink-700 transition hover:border-ink-300 hover:text-ink-900 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="actionBusy"
          @click="handleManualFirecrawl"
        >
          <Icon icon="mdi:web-sync" width="14" height="14" :class="{ 'animate-spin': manualFirecrawlLoading }" />
          {{ manualFirecrawlLabel }}
        </button>
        <button
          v-if="showManualSummaryAction"
          class="inline-flex items-center gap-1.5 rounded-full border border-ink-200 bg-white px-3 py-1.5 text-xs font-semibold text-ink-700 transition hover:border-ink-300 hover:text-ink-900 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="actionBusy"
          @click="handleManualSummary"
        >
          <Icon icon="mdi:brain" width="14" height="14" :class="{ 'animate-spin': manualSummaryLoading }" />
          {{ manualSummaryLabel }}
        </button>
      </div>
    </div>

    <div v-if="detailLines.length" class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-medium">
      <span v-for="line in detailLines" :key="line">{{ line }}</span>
    </div>

    <div
      v-if="manualActionError || mergedArticle?.firecrawlError || mergedArticle?.completionError"
      class="mt-3 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700"
    >
      {{ manualActionError || mergedArticle?.completionError || mergedArticle?.firecrawlError }}
    </div>
  </div>

  <!-- Content Source Toggle -->
  <div
    v-if="showContentSourceToggle"
    class="mb-6 flex items-center gap-2 rounded-2xl border border-ink-200 bg-white/80 p-2 shadow-subtle"
  >
    <button
      class="rounded-xl px-3 py-2 text-xs font-semibold transition"
      :class="activeContentSource === 'original' ? 'bg-ink-900 text-white' : 'text-ink-600 hover:bg-ink-100'"
      @click="emit('update:activeContentSource', 'original')"
    >
      原始内容
    </button>
    <button
      class="rounded-xl px-3 py-2 text-xs font-semibold transition"
      :class="activeContentSource === 'firecrawl' ? 'bg-ink-900 text-white' : 'text-ink-600 hover:bg-ink-100'"
      @click="emit('update:activeContentSource', 'firecrawl')"
    >
      Firecrawl 全文
    </button>
  </div>

  <!-- AI Summary -->
  <div v-if="mergedArticle?.aiContentSummary" class="mb-6 rounded-2xl border border-ink-200 bg-white/80 p-5 shadow-subtle">
    <div class="mb-3 flex items-center gap-2 text-ink-700">
      <Icon icon="mdi:brain" width="18" height="18" />
      <span class="text-sm font-semibold">AI 整理稿</span>
    </div>
    <ArticleTagList
      v-if="article?.tags?.length"
      class="mb-3"
      :tags="article.tags"
      :highlighted-slugs="highlightedTagSlugs"
      compact
      :show-article-count="false"
      show-watch
      @watch-toggle="handleTagWatchToggle"
    />
    <div class="summary-surface">
      <div class="markdown-body markdown-summary" v-html="renderedStoredSummary" />
    </div>
  </div>

  <!-- Meta -->
  <div class="article-meta">
    <span>{{ dayjs(articlePubDate).format('YYYY年MM月DD日 HH:mm') }}</span>
    <span v-if="articleAuthor">作者：{{ articleAuthor }}</span>
    <span v-if="articleRead" class="read-badge">
      <Icon icon="mdi:check-circle" width="14" height="14" />
      已读
    </span>
  </div>

  <!-- Title -->
  <h1 class="article-title-full">{{ articleTitleFull }}</h1>

  <!-- Tags & Manual Tagging -->
  <div class="mb-4 flex flex-wrap items-center gap-2">
    <ArticleTagList
      v-if="article?.tags?.length"
      :tags="article.tags"
      :highlighted-slugs="highlightedTagSlugs"
      compact
      show-watch
      @watch-toggle="handleTagWatchToggle"
    />
    <span v-if="manualTaggingLoading" class="inline-flex items-center gap-1 text-xs text-ink-medium">
      <Icon icon="mdi:loading" width="14" height="14" class="animate-spin" />
      正在生成标签...
    </span>
    <button
      v-else-if="aiEnabled"
      class="inline-flex items-center gap-1 rounded-full border border-dashed border-ink-200 bg-white/80 px-2.5 py-1 text-xs font-medium text-ink-medium transition hover:border-ink-300 hover:text-ink-900 disabled:cursor-not-allowed disabled:opacity-50"
      :disabled="actionBusy"
      @click="handleManualTagging"
    >
      <Icon icon="mdi:tag-plus-outline" width="14" height="14" />
      {{ manualTaggingLabel }}
    </button>
  </div>

  <div v-if="taggingError" class="mb-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs text-rose-700">
    {{ taggingError }}
  </div>

  <!-- Description -->
  <div v-if="showDescription" class="article-description">
    <div v-html="article?.description" />
  </div>

  <!-- Image -->
  <div v-if="articleImageUrl" class="article-image">
    <img :src="articleImageUrl" :alt="articleTitleFull" class="w-full">
  </div>

  <!-- Article Body -->
  <div class="article-body">
    <div v-if="displayContent" class="markdown-body markdown-article" v-html="displayContent" />
    <div v-else class="empty-content">
      <button class="btn btn-primary mt-4" @click="openOriginal">前往原文阅读</button>
    </div>
  </div>
</template>
