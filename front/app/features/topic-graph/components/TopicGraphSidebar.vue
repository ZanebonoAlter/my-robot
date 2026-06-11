<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { TopicGraphDetailPayload } from '~/api/topicGraph'
import type { TagHierarchyNode } from '~/types/topicTag'
import type { PendingArticle, TimelineDigestSelection, TimelineAggregationArticle } from '~/types/timeline'
import KeywordCloud from './KeywordCloud.vue'
import TopicGraphArticleCard from './TopicGraphArticleCard.vue'
import TopicGraphMergeDialog from './TopicGraphMergeDialog.vue'
import { useTopicGraphSidebar } from '~/features/topic-graph/composables/useTopicGraphSidebar'

interface Props {
  detail: TopicGraphDetailPayload | null
  selectedDigest?: TimelineDigestSelection | null
  loading?: boolean
  error?: string | null
  dataState?: string
  selectedKeyword?: string | null
  selectedTagSlug?: string | null
  pendingArticles?: PendingArticle[]
  selectedPendingNode?: boolean
  abstractNodeSlug?: string | null
  abstractNodeLabel?: string | null
  timelineGroupArticles?: TimelineAggregationArticle[]
  timelineGroupKey?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  selectedDigest: null,
  error: null,
  dataState: 'empty',
  selectedKeyword: null,
  selectedTagSlug: null,
  pendingArticles: () => [],
  selectedPendingNode: false,
  abstractNodeSlug: null,
  abstractNodeLabel: null,
  timelineGroupArticles: () => [],
  timelineGroupKey: null,
})

const emit = defineEmits<{
  openArticle: [articleId: number]
  highlightKeyword: [keywordSlug: string | null]
  tagMerged: []
  selectChildTag: [slug: string, label: string]
  selectAbstractTag: [slug: string]
}>()

const {
  activeKeywordSlug,
  showMergeDialog, mergeSearchQuery, mergeSearchResults,
  mergeSearching, mergeMerging, mergeError, mergeSuccess,
  abstractChildren, abstractLoading,
  topicCategoryLabels, deduplicatedArticles, keywords,
  shouldScrollFeaturedArticles, displayTopicCategory,
  handleKeywordSelect,
  openMergeDialog, closeMergeDialog, onMergeSearchInput, doMerge,
  handleChildTagClick, handleAbstractTagClick,
} = useTopicGraphSidebar(props, {
  openArticle: (id: number) => emit('openArticle', id),
  highlightKeyword: (slug: string | null) => emit('highlightKeyword', slug),
  tagMerged: () => emit('tagMerged'),
  selectChildTag: (slug: string, label: string) => emit('selectChildTag', slug, label),
  selectAbstractTag: (slug: string) => emit('selectAbstractTag', slug),
})
</script>

<template>
  <aside
    class="topic-sidebar rounded-[34px] px-5 py-5 md:px-6 md:py-6"
    data-testid="topic-graph-sidebar"
    :data-state="props.dataState"
  >
    <div v-if="props.loading" class="topic-sidebar__empty">正在展开话题脉络...</div>
    <div v-else-if="props.error" class="topic-sidebar__empty">{{ props.error }}</div>
    <div v-else-if="!props.detail" class="topic-sidebar__empty">点一个节点，右侧就会展开这类题材的近期总结、历史轨迹和外部入口。</div>
    <div v-else class="topic-sidebar__content">
      <!-- Current topic header -->
      <section class="space-y-3">
        <p class="topic-sidebar__eyebrow">当前焦点</p>
        <div class="flex flex-wrap items-center gap-3">
          <h2 class="font-serif text-3xl text-[var(--topic-ink-strong)]">{{ props.detail.topic.label }}</h2>
          <span class="topic-pill" :class="`topic-pill--${displayTopicCategory}`">
            {{ topicCategoryLabels[displayTopicCategory] }}
          </span>
          <button
            class="topic-merge-btn"
            type="button"
            title="合并到其他标签"
            @click="openMergeDialog"
          >
            <Icon icon="mdi:merge" class="text-base" />
          </button>
        </div>
        <p class="text-sm text-[var(--topic-ink-medium)]">
          {{ props.selectedPendingNode ? '待整理文章列表' : (props.selectedDigest ? '当前日报来源文章' : '先从下方选择一条日报') }}
        </p>
      </section>

      <!-- Pending Articles Section -->
      <section v-if="props.selectedPendingNode" class="topic-panel topic-panel--featured rounded-[28px] p-4 md:p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="topic-sidebar__eyebrow">待整理文章</p>
            <p class="topic-related-card__context mt-2">已打标签但尚未生成日报</p>
          </div>
          <span class="topic-summary__count">{{ props.pendingArticles.length }} 条</span>
        </div>
        <div
          v-if="props.pendingArticles.length"
          class="topic-sidebar__news-scroll mt-4"
          data-testid="topic-graph-pending-articles"
        >
          <div class="grid gap-3">
            <TopicGraphArticleCard
              v-for="article in props.pendingArticles"
              :key="article.id"
              :article="article"
              :data-article-id="String(article.id)"
              context="待整理文章"
              @click="emit('openArticle', article.id)"
            />
          </div>
        </div>
        <div v-else class="topic-sidebar__empty topic-sidebar__empty--soft">当前没有待整理的文章。</div>
      </section>

      <!-- Related Articles (deduplicated) -->
      <section v-if="!props.selectedPendingNode" class="topic-panel topic-panel--featured rounded-[28px] p-4 md:p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="topic-sidebar__eyebrow">{{ props.timelineGroupKey ? '时间线文章' : '日报文章' }}</p>
            <p v-if="props.timelineGroupKey && props.timelineGroupArticles.length" class="topic-related-card__context mt-2">
              该时间段共 {{ props.timelineGroupArticles.length }} 篇关联文章
            </p>
            <p v-else-if="props.selectedDigest" class="topic-related-card__context mt-2">{{ props.selectedDigest.title }}</p>
          </div>
          <span class="topic-summary__count">{{ props.timelineGroupKey ? props.timelineGroupArticles.length : deduplicatedArticles.length }} 条</span>
        </div>

        <!-- Timeline group articles -->
        <template v-if="props.timelineGroupKey && props.timelineGroupArticles.length">
          <div
            class="topic-sidebar__news-scroll mt-4"
            :class="{ 'topic-sidebar__news-scroll--bounded': props.timelineGroupArticles.length > 8 }"
          >
            <div class="grid gap-3">
              <TopicGraphArticleCard
                v-for="article in props.timelineGroupArticles"
                :key="article.id"
                :article="article"
                :context="article.pubDate ? new Date(article.pubDate).toLocaleString('zh-CN') : '日期待补'"
                @click="emit('openArticle', Number(article.id))"
              />
            </div>
          </div>
          <div v-if="!props.timelineGroupArticles.length" class="topic-sidebar__empty topic-sidebar__empty--soft">
            该时间段暂无关联文章
          </div>
        </template>
        <!-- Deduplicated digest articles -->
        <template v-else-if="!props.timelineGroupKey">
        <div
          v-if="deduplicatedArticles.length"
          class="topic-sidebar__news-scroll mt-4"
          :class="{ 'topic-sidebar__news-scroll--bounded': shouldScrollFeaturedArticles }"
          data-testid="topic-graph-related-articles"
        >
          <div class="grid gap-3">
            <TopicGraphArticleCard
              v-for="article in deduplicatedArticles"
              :key="article.id"
              :article="article"
              :context="`来自：${props.selectedDigest?.title || '当前日报'}`"
              :note="article.matchedBySummaryOnly ? '命中日报关键词，article 本身暂未打上当前 topic 标签' : '命中当前 topic/article 标签'"
              :note-soft="article.matchedBySummaryOnly"
              @click="emit('openArticle', article.id)"
            />
          </div>
        </div>
        <div v-else class="topic-sidebar__empty topic-sidebar__empty--soft">点击下方日报后，这里只展示该日报里命中当前主题的文章。</div>
        </template>
      </section>

      <!-- Keyword Cloud (Related Topics) -->
      <section v-if="keywords.length > 0" class="topic-panel rounded-[26px] p-4">
        <p class="topic-sidebar__eyebrow">相关主题</p>
        <div class="mt-4">
          <KeywordCloud
            :keywords="keywords"
            :selected-keyword="activeKeywordSlug"
            @select="handleKeywordSelect"
          />
          <p class="keywords-hint">
            点击标签，只高亮当前标签节点和它的一跳邻居
          </p>
        </div>
      </section>

      <!-- Abstract Tag Detail Panel -->
      <section v-if="props.abstractNodeSlug && abstractChildren.length > 0" class="topic-panel rounded-[26px] p-4">
        <p class="topic-sidebar__eyebrow">抽象标签详情</p>

        <!-- Child Tags -->
        <div class="mt-3">
          <h4 class="text-xs font-medium text-[var(--topic-ink-soft)]">子标签</h4>
          <div class="mt-2 grid gap-1.5">
            <!-- Abstract tag itself (returns to parent) -->
            <button
              type="button"
              class="flex items-center gap-2 rounded-xl px-2.5 py-1.5 text-left transition-all hover:border-[rgba(240,138,75,0.2)]"
              :class="'bg-[rgba(10,16,23,0.56)] border border-[rgba(240,138,75,0.3)]'"
              @click="handleAbstractTagClick"
            >
              <span class="w-2 h-2 rounded-full shrink-0 bg-[rgba(240,138,75,0.8)]"></span>
              <span class="text-sm text-[var(--topic-ink-strong)] truncate">{{ props.abstractNodeLabel || '全部' }}</span>
              <span class="ml-auto text-xs text-[rgba(240,138,75,0.7)] shrink-0">全部</span>
            </button>
            <!-- Child tags -->
            <button
                v-for="child in abstractChildren"
                :key="child.slug"
                type="button"
                class="flex items-center gap-2 rounded-xl px-2.5 py-1.5 text-left transition-all hover:border-[rgba(240,138,75,0.2)]"
                :class="'bg-[rgba(10,16,23,0.56)] border border-[var(--topic-border)]'"
                @click="handleChildTagClick(child)"
              >
              <span class="w-2 h-2 rounded-full shrink-0" :class="`bg-[#6366f1]`"></span>
              <span class="text-sm text-[var(--topic-ink-strong)] truncate">{{ child.label }}</span>
              <span class="ml-auto text-xs text-[var(--topic-ink-soft)] shrink-0">{{ child.articleCount }} 篇</span>
            </button>
          </div>
        </div>
      </section>
    </div>

    <!-- Merge Dialog -->
    <TopicGraphMergeDialog
      :show="showMergeDialog"
      :topic-label="props.detail?.topic?.label"
      :search-query="mergeSearchQuery"
      :search-results="mergeSearchResults"
      :searching="mergeSearching"
      :merging="mergeMerging"
      :error="mergeError"
      :success="mergeSuccess"
      @close="closeMergeDialog"
      @search-input="onMergeSearchInput"
      @do-merge="doMerge"
    />
  </aside>
</template>

<style scoped>
.topic-sidebar {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  position: relative;
  overflow: hidden;
  --topic-ink-strong: rgba(248, 251, 255, 0.96);
  --topic-ink-medium: rgba(210, 221, 232, 0.82);
  --topic-ink-soft: rgba(148, 168, 188, 0.7);
  --topic-border: rgba(123, 154, 192, 0.18);
  --topic-border-strong: rgba(123, 154, 192, 0.28);
  --topic-card: linear-gradient(180deg, rgba(20, 30, 42, 0.86), rgba(11, 17, 25, 0.92));
  --topic-card-raised: linear-gradient(180deg, rgba(25, 37, 50, 0.94), rgba(13, 21, 30, 0.96));
  --topic-chip: rgba(12, 19, 28, 0.82);
  background:
    radial-gradient(circle at 16% 14%, rgba(240, 138, 75, 0.18), transparent 26%),
    radial-gradient(circle at 82% 10%, rgba(74, 129, 219, 0.16), transparent 24%),
    linear-gradient(180deg, rgba(17, 28, 39, 0.96), rgba(8, 14, 22, 0.98));
  border: 1px solid rgba(153, 187, 227, 0.18);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 28px 90px rgba(2, 6, 12, 0.4);
}
.topic-sidebar::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent 18%),
    linear-gradient(90deg, rgba(255, 255, 255, 0.03), transparent 28%);
  pointer-events: none;
}
.topic-sidebar::after {
  content: '';
  position: absolute;
  inset: 1rem auto 1rem 1rem;
  width: 1px;
  background: linear-gradient(180deg, rgba(240, 138, 75, 0.4), rgba(115, 150, 198, 0.08) 42%, rgba(115, 150, 198, 0));
  pointer-events: none;
}
.topic-sidebar__content {
  position: relative;
  z-index: 1;
  display: grid;
  min-height: 0;
  flex: 1;
  gap: 1.5rem;
  grid-template-rows: auto minmax(0, 1fr) auto;
}
.topic-sidebar__content > section:first-child {
  padding-left: 0.75rem;
}
.topic-sidebar__empty--soft {
  border-style: solid;
  background: rgba(10, 16, 23, 0.56);
}
.topic-sidebar__eyebrow {
  font-size: 0.72rem;
  letter-spacing: 0.24em;
  text-transform: uppercase;
  color: var(--topic-ink-soft);
}
.topic-sidebar__empty {
  position: relative;
  z-index: 1;
  border-radius: 1.6rem;
  border: 1px dashed rgba(153, 187, 227, 0.2);
  background: rgba(9, 15, 23, 0.5);
  padding: 1.2rem;
  color: var(--topic-ink-medium);
}
.topic-panel {
  position: relative;
  overflow: hidden;
  border: 1px solid var(--topic-border);
  background: var(--topic-card);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 22px 60px rgba(2, 6, 12, 0.24);
  backdrop-filter: blur(16px);
}
.topic-panel::before {
  content: '';
  position: absolute;
  inset: 0 auto auto 0;
  width: 100%;
  height: 1px;
  background: linear-gradient(90deg, rgba(240, 138, 75, 0.44), rgba(120, 167, 230, 0.14), rgba(255, 255, 255, 0));
  pointer-events: none;
}
.topic-panel--featured {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background: var(--topic-card-raised);
}
.topic-sidebar__news-scroll {
  min-height: 0;
  flex: 1;
  padding-right: 0.25rem;
}
.topic-sidebar__news-scroll--bounded {
  overflow-y: auto;
  max-height: 75vh;
}
.topic-sidebar__news-scroll--bounded::-webkit-scrollbar {
  width: 0.45rem;
}
.topic-sidebar__news-scroll--bounded::-webkit-scrollbar-track {
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
}
.topic-sidebar__news-scroll--bounded::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(240, 138, 75, 0.28);
}
.topic-pill {
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(22, 29, 39, 0.88), rgba(11, 17, 24, 0.96));
  padding: 0.45rem 0.85rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: rgba(248, 251, 255, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}
.topic-pill--event { border: 1px solid rgba(245, 158, 11, 0.32); }
.topic-pill--person { border: 1px solid rgba(16, 185, 129, 0.32); }
.topic-pill--keyword { border: 1px solid rgba(99, 102, 241, 0.32); }
.topic-related-card__context {
  font-size: 0.74rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--topic-ink-soft, rgba(148, 168, 188, 0.7));
}
.topic-related-card__context {
  margin-top: 0.65rem;
}
@media (max-width: 1279px) {
  .topic-sidebar__content {
    display: flex;
    flex-direction: column;
  }
  .topic-panel--featured,
  .topic-sidebar__news-scroll {
    min-height: auto;
    flex: none;
    overflow: visible;
  }
}
.topic-summary__count {
  border-radius: 999px;
  border: 1px solid rgba(240, 138, 75, 0.2);
  background: var(--topic-chip);
  padding: 0.32rem 0.7rem;
  font-size: 0.75rem;
  color: rgba(255, 228, 209, 0.88);
}
.keywords-hint {
  font-size: 0.72rem;
  color: rgba(255, 255, 255, 0.4);
  text-align: center;
  margin-top: 0.75rem;
}
.topic-merge-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  border: 1px solid rgba(141, 173, 214, 0.2);
  background: rgba(14, 21, 30, 0.7);
  color: rgba(241, 246, 250, 0.7);
  cursor: pointer;
  transition: all 0.2s ease;
}
.topic-merge-btn:hover {
  border-color: rgba(240, 138, 75, 0.4);
  color: rgba(240, 138, 75, 0.9);
  background: rgba(240, 138, 75, 0.1);
}
</style>
