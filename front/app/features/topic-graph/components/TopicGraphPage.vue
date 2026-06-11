<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useTopicGraph } from '~/features/topic-graph/composables/useTopicGraph'
import { normalizeTopicCategory } from '~/features/topic-graph/utils/normalizeTopicCategory'
import { ArticleContentView } from '~/features/articles/public'
import FeedCategoryFilter from '~/features/topic-graph/components/FeedCategoryFilter.vue'
import TopicGraphCanvas from '~/features/topic-graph/components/TopicGraphCanvas.client.vue'
import TopicGraphFooterPanels from '~/features/topic-graph/components/TopicGraphFooterPanels.vue'
import TopicGraphHeader from '~/features/topic-graph/components/TopicGraphHeader.vue'
import TopicGraphSidebar from '~/features/topic-graph/components/TopicGraphSidebar.vue'
import TopicTimeline from '~/features/topic-graph/components/TopicTimeline.vue'

const {
  // Core state
  selectedType, selectedDate, selectedFilterCategoryId, selectedFilterFeedId,
  selectedCategory, selectedKeywordSlug, selectedDigestId, previewDigestId,
  graphFocusRequestKey, detail, loadingGraph, loadingDetail, loadingPreviewArticle,
  notice, selectedPreviewArticle, previewArticles, selectedTopicSlug, pageState, graphPayload,

  // Hotspot
  hotspotCategories, hotspotSearchQueries, hotspotDropdownOpen, hotspotShowAll,
  hotspotSearchRefs, selectedHotspotTag,

  // Pending
  pendingArticles, selectedPendingNode,

  // Timeline
  aggregationMode, selectedGroupKey, selectedGroupArticles,
  timelineAggregationGroups, totalAggregatedCount, timelineOpen,
  selectedDigest, previewDigest,

  // Abstract tag
  abstractNodeSlug, abstractNodeLabel,

  // Computed
  viewModel, activeTopicNode, highlightedNodeIds, relatedEdgeIds, displayedGraph,
  selectedTopicInfo,

  // Timeline drag
  timelinePanelRef, isDragging, timelinePosition, resetTimelinePanelPosition,

  // Graph helpers
  isTopicShownInGraph, toggleTopicGraphVisibility,

  // Handlers
  handleNodeClick, handleKeywordHighlight, handleDigestSelect, handlePreviewDigest,
  closeDigestPreview, openArticlePreview, closeArticlePreview,
  handleArticleFavorite, handleArticleUpdate, handleTagSelect,
  handleChildTagSelect, handleAbstractTagSelect, handleTimelineGroupSelect,
  handleAggregationModeChange, handleSelectPending,
  toggleShowAll, closeHotspotDropdown, startTimelineDrag,

  // Actions
  loadGraph,
} = useTopicGraph()
</script>

<template>
  <div
    class="topic-stage min-h-screen px-4 py-5 md:px-6 md:py-7"
    data-testid="topic-graph-page"
    :data-state="pageState"
  >
    <div class="topic-shell mx-auto w-full">
      <section class="topic-layout grid gap-5 2xl:grid-cols-[minmax(0,2.15fr)_minmax(430px,0.95fr)]">
        <div class="space-y-5">
          <article class="topic-canvas-shell rounded-[34px] p-4 md:p-5">
            <div class="topic-studio grid gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
              <aside class="topic-studio__rail rounded-[30px] p-4 md:p-5">
                <TopicGraphHeader
                  :selected-type="selectedType"
                  :selected-date="selectedDate"
                  :loading="loadingGraph"
                  :hero-label="viewModel.stats.heroLabel"
                  :hero-subline="viewModel.stats.heroSubline"
                  @update:type="selectedType = $event"
                  @update:date="selectedDate = $event"
                  @refresh="loadGraph"
                />

                <div class="mt-6">
                  <p class="text-xs uppercase tracking-[0.3em] text-white/42">Graph Field</p>
                  <h2 class="mt-2 font-serif text-2xl text-white md:text-[2.25rem]">{{ graphPayload?.period_label || '话题网络' }}</h2>
                  <p class="mt-3 text-sm leading-6 text-[rgba(255,255,255,0.68)]">
                    事件、人物、关键词的热点题材默认全部进入拓扑图；底部可单独控制各节点的显示与隐藏。
                  </p>
                </div>

                <FeedCategoryFilter
                  :selected-category-id="selectedFilterCategoryId"
                  :selected-feed-id="selectedFilterFeedId"
                  @update:selected-category-id="selectedFilterCategoryId = $event"
                  @update:selected-feed-id="selectedFilterFeedId = $event"
                />
              </aside>

              <div class="space-y-4">
                <TopicGraphCanvas
                  :nodes="displayedGraph.nodes"
                  :edges="displayedGraph.edges"
                  :featured-node-ids="displayedGraph.featuredNodeIds"
                  :active-node-id="activeTopicNode?.id || null"
                  :focus-request-key="graphFocusRequestKey"
                  :selected-category="selectedCategory"
                  :highlighted-node-ids="highlightedNodeIds"
                  :related-edge-ids="relatedEdgeIds"
                  @node-click="handleNodeClick"
                />

                <article class="topic-note rounded-[30px] px-5 py-4 text-sm leading-6 text-[rgba(255,255,255,0.78)]">
                  <div class="flex items-start gap-3">
                    <Icon icon="mdi:orbit-variant" width="20" height="20" class="mt-1 text-[rgba(240,138,75,0.92)]" />
                    <p>
                      先看结构，再读内容：亮色主节点是当前焦点，周边只保留一跳关系的高亮，更多细节放到右侧阅读栏。
                    </p>
                  </div>
                </article>

                <section class="topic-hotspot-strip rounded-[30px] p-4 md:p-5">
                  <div class="topic-hotspot-strip__header">
                    <div>
                      <p class="text-xs uppercase tracking-[0.24em] text-white/42">热点题材</p>
                      <h3 class="mt-2 font-serif text-xl text-white">把最热的话题平铺到底部，避免和左侧控制重复。</h3>
                    </div>
                  </div>

                  <div class="mt-4 grid gap-3 xl:grid-cols-3">
                    <section
                      v-for="category in hotspotCategories"
                      :key="category.key"
                      class="topic-category-column rounded-[22px] p-3"
                      :data-testid="`hotspot-category-${category.key}`"
                    >
                      <div class="topic-category-header" :class="category.headerClass">
                        <Icon :icon="category.icon" width="14" />
                        <span>{{ category.label }}</span>
                        <span class="topic-count">({{ category.topics.length }})</span>
                      </div>

                      <div :ref="el => { if (el) hotspotSearchRefs[category.key] = el as HTMLDivElement }" class="topic-search-wrapper mt-3">
                        <div class="topic-search-input-wrapper" @click="hotspotDropdownOpen[category.key] = true">
                          <Icon icon="mdi:magnify" width="14" class="topic-search-icon" />
                          <input
                            v-model="hotspotSearchQueries[category.key]"
                            type="text"
                            class="topic-search-input"
                            placeholder="搜索..."
                            @focus="hotspotDropdownOpen[category.key] = true"
                          />
                          <button
                            v-if="hotspotSearchQueries[category.key]"
                            class="topic-search-clear"
                            @click.stop="hotspotSearchQueries[category.key] = ''"
                          >
                            <Icon icon="mdi:close" width="12" />
                          </button>
                        </div>

                        <div
                          v-if="hotspotDropdownOpen[category.key] && category.filteredTopics.length > 0"
                          class="topic-search-dropdown"
                          @mousedown.prevent
                        >
                          <div class="topic-dropdown-scroll">
                            <div
                              v-for="topic in category.displayTopics"
                              :key="topic.slug"
                              class="topic-dropdown-row"
                            >
                              <button
                                type="button"
                                class="topic-dropdown-item"
                                :class="{
                                  'topic-dropdown-item--active': selectedTopicSlug === topic.slug,
                                  'topic-dropdown-item--abstract': topic.is_abstract,
                                }"
                                @click="handleTagSelect(topic.slug, normalizeTopicCategory(topic.category, topic.kind)); hotspotDropdownOpen[category.key] = false"
                              >
                                <Icon v-if="topic.is_abstract" icon="mdi:tag-group" width="14" class="text-amber-400/80" />
                                <Icon v-else-if="topic.icon" :icon="topic.icon" width="14" />
                                <span>{{ topic.label }}</span>
                                <span v-if="topic.is_abstract" class="ml-1 text-[10px] text-amber-400/60">({{ topic.child_slugs?.length ?? 0 }})</span>
                                <span v-else-if="topic.is_low_quality" class="ml-1 text-[10px] text-white/35">低质量</span>
                              </button>
                              <button
                                type="button"
                                class="topic-graph-toggle"
                                :class="{ 'topic-graph-toggle--active': isTopicShownInGraph(topic.slug) }"
                                :aria-label="isTopicShownInGraph(topic.slug) ? `从拓扑图隐藏 ${topic.label}` : `在拓扑图展示 ${topic.label}`"
                                :title="isTopicShownInGraph(topic.slug) ? '从拓扑图隐藏' : '在拓扑图展示'"
                                @click.stop="toggleTopicGraphVisibility(topic.slug)"
                              >
                                <Icon :icon="isTopicShownInGraph(topic.slug) ? 'mdi:eye-outline' : 'mdi:eye-off-outline'" width="14" />
                              </button>
                            </div>
                          </div>
                          <button
                            v-if="category.hasMore"
                            class="topic-dropdown-toggle"
                            @click="toggleShowAll(category.key)"
                          >
                            <Icon :icon="category.showAll ? 'mdi:chevron-up' : 'mdi:chevron-down'" width="16" />
                            {{ category.showAll ? '恢复默认筛选' : `显示全部标签 (${category.filteredTopics.length})` }}
                          </button>
                        </div>

                        <div
                          v-if="hotspotDropdownOpen[category.key] && hotspotSearchQueries[category.key] && category.filteredTopics.length === 0"
                          class="topic-search-no-results"
                        >
                          未找到匹配的结果
                          <button
                            class="topic-dropdown-close"
                            @click.stop="closeHotspotDropdown(category.key)"
                          >
                            关闭
                          </button>
                        </div>
                      </div>

                      <div v-if="!hotspotSearchQueries[category.key]" class="topic-quick-tags mt-3">
                        <div
                          v-for="topic in category.topics.slice(0, 5)"
                          :key="topic.slug"
                          class="topic-badge-row"
                        >
                          <button
                            type="button"
                            class="topic-badge text-left"
                            :class="{
                              'topic-badge--event': normalizeTopicCategory(topic.category, topic.kind) === 'event',
                              'topic-badge--person': normalizeTopicCategory(topic.category, topic.kind) === 'person',
                              'topic-badge--keyword': normalizeTopicCategory(topic.category, topic.kind) === 'keyword',
                              'topic-badge--active': selectedTopicSlug === topic.slug,
                              'topic-badge--abstract': topic.is_abstract,
                            }"
                            @click="handleTagSelect(topic.slug, normalizeTopicCategory(topic.category, topic.kind))"
                          >
                            <Icon v-if="topic.is_abstract" icon="mdi:tag-group" width="14" class="text-amber-400/80" />
                            <Icon v-else-if="topic.icon" :icon="topic.icon" width="14" />
                            {{ topic.label }}
                            <span v-if="topic.is_abstract" class="ml-1 text-[10px] text-amber-400/60">({{ topic.child_slugs?.length ?? 0 }})</span>
                            <span v-else-if="topic.is_low_quality" class="ml-1 text-[10px] text-white/35">低质量</span>
                          </button>
                          <button
                            type="button"
                            class="topic-badge-toggle"
                            :class="{ 'topic-badge-toggle--active': isTopicShownInGraph(topic.slug) }"
                            :aria-label="isTopicShownInGraph(topic.slug) ? `从拓扑图隐藏 ${topic.label}` : `在拓扑图展示 ${topic.label}`"
                            :title="isTopicShownInGraph(topic.slug) ? '从拓扑图隐藏' : '在拓扑图展示'"
                            @click.stop="toggleTopicGraphVisibility(topic.slug)"
                          >
                            <Icon :icon="isTopicShownInGraph(topic.slug) ? 'mdi:eye-outline' : 'mdi:eye-off-outline'" width="14" />
                          </button>
                        </div>
                        <button
                          v-if="category.topics.length > 5"
                          class="topic-more-hint"
                          @click="hotspotSearchQueries[category.key] = ''; hotspotDropdownOpen[category.key] = true"
                        >
                          +{{ category.topics.length - 5 }} 更多
                        </button>
                      </div>
                    </section>
                  </div>
                </section>

                <TopicGraphFooterPanels :detail="detail" />
              </div>
            </div>
          </article>

          <button
            type="button"
            class="timeline-fab"
            :class="{ 'timeline-fab--active': timelineOpen }"
            @click="timelineOpen = !timelineOpen"
          >
            <Icon icon="mdi:timeline-clock-outline" width="18" />
            <span class="timeline-fab__label">时间线</span>
            <span v-if="totalAggregatedCount" class="timeline-fab__badge">{{ totalAggregatedCount }}</span>
          </button>
        </div>

        <div class="topic-reading-rail" data-testid="topic-graph-sidebar-region">
          <TopicGraphSidebar
            :detail="detail"
            :selected-digest="selectedDigest"
            :loading="loadingDetail"
            :error="notice"
            :data-state="detail ? 'detail' : (loadingDetail ? 'loading' : 'empty')"
            :selected-keyword="selectedKeywordSlug"
            :selected-tag-slug="selectedHotspotTag?.slug"
            :pending-articles="selectedPendingNode ? pendingArticles : []"
            :selected-pending-node="selectedPendingNode"
            :abstract-node-slug="abstractNodeSlug"
            :abstract-node-label="abstractNodeLabel"
            :timeline-group-articles="selectedGroupArticles"
            :timeline-group-key="selectedGroupKey"
            @open-article="openArticlePreview"
            @highlight-keyword="handleKeywordHighlight"
            @select-child-tag="handleChildTagSelect"
            @select-abstract-tag="handleAbstractTagSelect"
          />
        </div>
      </section>
    </div>

    <Teleport to="body">
      <Transition name="timeline-slide">
        <div
          v-if="timelineOpen"
          ref="timelinePanelRef"
          class="timeline-float-panel"
          :class="{ 'timeline-float-panel--dragging': isDragging }"
          :style="{
            transform: timelinePosition.x || timelinePosition.y
              ? `translate(${timelinePosition.x}px, ${timelinePosition.y}px)`
              : 'none',
          }"
        >
          <header
            class="timeline-float-panel__header"
            @mousedown="startTimelineDrag"
          >
            <div class="flex items-center gap-2">
              <Icon icon="mdi:drag-horizontal-variant" width="16" class="text-white/40 cursor-grab" />
              <Icon icon="mdi:timeline-clock-outline" width="16" class="text-[rgba(240,138,75,0.8)]" />
              <span class="font-serif text-sm text-white">时间线</span>
              <span v-if="totalAggregatedCount" class="text-xs text-white/40">{{ totalAggregatedCount }} 篇</span>
            </div>
            <button
              type="button"
              class="btn-ghost min-h-8 min-w-8 px-0"
              @mousedown.stop
              @click="timelineOpen = false"
            >
              <Icon icon="mdi:close" width="16" />
            </button>
          </header>
          <div class="timeline-float-panel__body">
            <TopicTimeline
              :selected-topic="selectedTopicInfo"
              :groups="timelineAggregationGroups"
              :active-group-key="selectedGroupKey"
              :aggregation-mode="aggregationMode"
              :total-count="totalAggregatedCount"
              @select-group="handleTimelineGroupSelect"
              @open-article="openArticlePreview"
              @update:aggregation-mode="handleAggregationModeChange"
            />
          </div>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="selectedPreviewArticle"
        class="topic-article-modal"
        data-testid="topic-graph-article-preview"
        @click.self="closeArticlePreview"
      >
        <div class="topic-article-modal__panel">
          <header class="topic-article-modal__header">
            <p class="truncate text-sm text-ink-medium">
              {{ loadingPreviewArticle ? '正在准备文章预览...' : '文章预览里保留项目已有的阅读、收藏和抓取动作。' }}
            </p>
            <button
              class="btn-ghost min-h-11 min-w-11 px-0"
              type="button"
              aria-label="关闭文章弹窗"
              data-testid="topic-graph-article-preview-close"
              @click="closeArticlePreview"
            >
              <Icon icon="mdi:close" width="18" />
            </button>
          </header>
          <div class="topic-article-modal__body">
            <ArticleContentView
              :article="selectedPreviewArticle"
              :articles="previewArticles"
              :highlighted-tag-slugs="selectedTopicSlug ? [selectedTopicSlug] : []"
              @navigate="selectedPreviewArticle = $event"
              @favorite="handleArticleFavorite"
              @article-update="handleArticleUpdate"
            />
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.topic-stage {
  background:
    radial-gradient(circle at top left, rgba(240, 138, 75, 0.18), transparent 24%),
    radial-gradient(circle at 85% 12%, rgba(63, 124, 255, 0.18), transparent 24%),
    linear-gradient(180deg, #0e161d 0%, #172733 54%, #10212e 100%);
}

.topic-shell {
  width: min(100%, calc(100vw - 1.5rem));
}

.topic-canvas-shell {
  position: relative;
  z-index: 2;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(11, 18, 24, 0.4);
  box-shadow: 0 40px 120px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(20px);
}

.topic-note {
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(11, 18, 24, 0.7);
  box-shadow: 0 24px 80px rgba(6, 10, 16, 0.24);
  backdrop-filter: blur(12px);
}

.topic-hotspot-strip {
  position: relative;
  z-index: 4;
  overflow: visible;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background:
    radial-gradient(circle at 12% 18%, rgba(240, 138, 75, 0.12), transparent 24%),
    linear-gradient(180deg, rgba(12, 19, 27, 0.86), rgba(8, 14, 22, 0.96));
  box-shadow: 0 24px 80px rgba(6, 10, 16, 0.22);
}

.topic-hotspot-strip__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.topic-timeline-shell {
  position: relative;
  z-index: 1;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(11, 18, 24, 0.4);
  box-shadow: 0 40px 120px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(20px);
}

.topic-layout {
  align-items: start;
}

.topic-studio__rail {
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(255, 255, 255, 0.04);
  background: linear-gradient(180deg, rgba(15, 23, 31, 0.85), rgba(8, 14, 20, 0.95));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.topic-stat-card {
  border: 1px solid rgba(255, 255, 255, 0.04);
  background: rgba(0, 0, 0, 0.2);
}

.topic-stat-card__label {
  font-size: 0.7rem;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: rgba(255,255,255,0.46);
}

.topic-stat-card__value {
  margin-top: 0.55rem;
  font-size: 1.8rem;
  font-weight: 700;
  color: white;
}

.topic-reading-rail {
  position: sticky;
  top: 1rem;
}

.topic-digest-modal {
  position: fixed;
  inset: 0;
  z-index: 78;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: rgba(8, 12, 18, 0.7);
  backdrop-filter: blur(10px);
}

.topic-digest-modal__panel {
  width: min(1100px, 100%);
  max-height: calc(100vh - 2rem);
  overflow: auto;
  border-radius: 1.75rem;
  background: linear-gradient(180deg, rgba(17, 27, 38, 0.98), rgba(9, 15, 23, 1));
  box-shadow: 0 30px 100px rgba(0, 0, 0, 0.32);
}

.topic-digest-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding: 1.1rem 1.25rem 1rem;
}

.topic-digest-modal__body {
  display: grid;
  gap: 1rem;
  padding: 1.2rem 1.25rem 1.35rem;
}

.topic-digest-modal__summary {
  line-height: 1.8;
  color: rgba(236, 242, 248, 0.9);
  white-space: pre-wrap;
}

.topic-digest-modal__tags,
.topic-digest-modal__sources {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.topic-digest-modal__tag,
.topic-digest-modal__source {
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
}

.topic-digest-modal__tag {
  padding: 0.32rem 0.7rem;
  font-size: 0.78rem;
  color: rgba(245, 227, 212, 0.88);
}

.topic-digest-modal__source {
  padding: 0.4rem 0.78rem;
  color: rgba(241, 246, 251, 0.84);
}

.topic-article-modal {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  align-items: stretch;
  justify-content: center;
  background: rgba(8, 12, 18, 0.7);
  padding: 1rem;
  backdrop-filter: blur(10px);
}

.topic-article-modal__panel {
  display: flex;
  height: calc(100vh - 2rem);
  width: min(1500px, 100%);
  flex-direction: column;
  overflow: hidden;
  border-radius: 1.75rem;
  background: rgba(255, 252, 248, 0.98);
  box-shadow: 0 30px 100px rgba(0, 0, 0, 0.28);
}

.topic-article-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(20, 33, 44, 0.08);
  padding: 1rem 1.25rem;
}

.topic-article-modal__body {
  min-height: 0;
  flex: 1;
}

@media (min-width: 1280px) {
  .topic-shell {
    width: min(100%, calc(100vw - 2rem));
  }
}

@media (min-width: 1600px) {
  .topic-shell {
    width: min(100%, calc(100vw - 2.75rem));
  }
}

.topic-badge {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.35rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 0.55rem 0.9rem;
  font-size: 0.78rem;
  color: rgba(255, 255, 255, 0.78);
  background: rgba(255,255,255,0.04);
  transition:
    transform 0.15s ease,
    border-color 0.15s ease,
    background 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.topic-badge:hover,
.topic-badge:focus-visible {
  transform: translateY(-1px);
  color: white;
}

.topic-badge--event {
  border-color: rgba(245, 158, 11, 0.72);
  background: rgba(245, 158, 11, 0.18);
}

.topic-badge--person {
  border-color: rgba(16, 185, 129, 0.72);
  background: rgba(16, 185, 129, 0.18);
}

.topic-badge--keyword {
  border-color: rgba(99, 102, 241, 0.72);
  background: rgba(99, 102, 241, 0.18);
}

.topic-badge--active {
  color: white;
  box-shadow: 0 12px 28px rgba(4, 8, 14, 0.24);
}

.topic-badge--abstract {
  border-color: rgba(251, 191, 36, 0.55);
  background: rgba(251, 191, 36, 0.12);
  font-weight: 500;
}

.topic-category-column {
  position: relative;
  z-index: 5;
  overflow: visible;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(20, 29, 40, 0.74), rgba(10, 15, 23, 0.92));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.topic-category-header {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.72rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.topic-category-header--button {
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 0.28rem 0.62rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.topic-category-header--button:hover,
.topic-category-header--button:focus-visible {
  border-color: rgba(255, 255, 255, 0.28);
}

.topic-category-header--active {
  border-color: rgba(255, 255, 255, 0.44);
  background: rgba(255, 255, 255, 0.12);
}

.topic-category-header--event {
  color: rgba(252, 211, 77, 0.9);
}

.topic-category-header--person {
  color: rgba(110, 231, 183, 0.9);
}

.topic-category-header--keyword {
  color: rgba(165, 180, 252, 0.92);
}

.topic-category-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

/* Hotspot Search Styles */
.topic-search-wrapper {
  position: relative;
  z-index: 6;
  width: 100%;
}

.topic-search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  transition: all 0.2s ease;
}

.topic-search-input-wrapper:focus-within {
  border-color: rgba(240, 138, 75, 0.5);
  background: rgba(0, 0, 0, 0.3);
}

.topic-search-icon {
  color: rgba(255, 255, 255, 0.4);
  flex-shrink: 0;
}

.topic-search-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: rgba(255, 255, 255, 0.9);
  font-size: 0.8rem;
  padding: 0 0.5rem;
  min-width: 0;
}

.topic-search-input::placeholder {
  color: rgba(255, 255, 255, 0.35);
}

.topic-search-clear {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.6);
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.topic-search-clear:hover {
  background: rgba(255, 255, 255, 0.2);
  color: rgba(255, 255, 255, 0.9);
}

.topic-search-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  background: rgba(22, 28, 38, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 0.5rem;
  max-height: 400px;
  display: flex;
  flex-direction: column;
  z-index: 50;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(20px);
}

.topic-dropdown-scroll {
  overflow-y: auto;
  max-height: 320px;
  padding-right: 0.25rem;
}

.topic-dropdown-scroll::-webkit-scrollbar {
  width: 4px;
}

.topic-dropdown-scroll::-webkit-scrollbar-track {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 2px;
}

.topic-dropdown-scroll::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
  border-radius: 2px;
}

.topic-dropdown-scroll::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.25);
}

.topic-dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  padding: 0.6rem 0.75rem;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.75);
  font-size: 0.82rem;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.topic-dropdown-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.topic-dropdown-row + .topic-dropdown-row {
  margin-top: 0.15rem;
}

.topic-dropdown-item:hover {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.95);
}

.topic-dropdown-item--active {
  background: rgba(240, 138, 75, 0.2) !important;
  color: rgba(255, 235, 220, 0.95) !important;
}

.topic-dropdown-item--abstract {
  background: rgba(251, 191, 36, 0.08);
  font-weight: 500;
}

.topic-graph-toggle,
.topic-badge-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(255, 255, 255, 0.55);
  transition:
    border-color 0.15s ease,
    background 0.15s ease,
    color 0.15s ease,
    transform 0.15s ease;
}

.topic-graph-toggle:hover,
.topic-graph-toggle:focus-visible,
.topic-badge-toggle:hover,
.topic-badge-toggle:focus-visible {
  transform: translateY(-1px);
  border-color: rgba(240, 138, 75, 0.38);
  color: rgba(255, 238, 227, 0.92);
}

.topic-graph-toggle {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
}

.topic-badge-toggle {
  flex-shrink: 0;
  width: 2rem;
  min-height: 2rem;
  border-radius: 999px;
}

.topic-graph-toggle--active,
.topic-badge-toggle--active {
  border-color: rgba(240, 138, 75, 0.44);
  background: rgba(240, 138, 75, 0.16);
  color: rgba(255, 234, 220, 0.96);
}

.topic-dropdown-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  width: 100%;
  padding: 0.6rem 0.75rem;
  margin-top: 0.35rem;
  border-radius: 8px;
  border: none;
  background: rgba(240, 138, 75, 0.15);
  color: rgba(255, 220, 200, 0.85);
  font-size: 0.78rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.topic-dropdown-toggle:hover {
  background: rgba(240, 138, 75, 0.25);
  color: rgba(255, 235, 220, 0.95);
}

.topic-search-no-results {
  padding: 1rem;
  text-align: center;
  color: rgba(255, 255, 255, 0.45);
  font-size: 0.8rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.topic-dropdown-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.4rem 0.9rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.7);
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.15s ease;
  align-self: center;
}

.topic-dropdown-close:hover {
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.9);
}

.topic-quick-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.topic-badge-row {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.topic-more-hint {
  display: inline-flex;
  align-items: center;
  padding: 0.4rem 0.75rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.4);
  font-size: 0.75rem;
  border: none;
  cursor: pointer;
  transition: all 0.15s ease;
}

.topic-more-hint:hover {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.6);
}

.th-tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  background: none;
  color: rgba(255, 255, 255, 0.45);
  font-size: 0.7rem;
  padding: 0.28rem 0.7rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.th-tab-btn:hover {
  border-color: rgba(255, 255, 255, 0.2);
  color: rgba(255, 255, 255, 0.7);
}

.th-tab-btn--active {
  border-color: rgba(240, 138, 75, 0.45);
  background: rgba(240, 138, 75, 0.1);
  color: rgba(255, 220, 200, 0.88);
}

.topic-count {
  margin-left: 0.5rem;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.5);
  font-size: 0.7rem;
}

/* Timeline floating action button */
.timeline-fab {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  z-index: 60;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid rgba(240, 138, 75, 0.35);
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(20, 29, 40, 0.92), rgba(12, 18, 26, 0.96));
  color: rgba(255, 220, 200, 0.9);
  font-size: 0.78rem;
  padding: 0.55rem 1rem;
  cursor: pointer;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.35), 0 0 0 1px rgba(240, 138, 75, 0.08);
  backdrop-filter: blur(16px);
  transition: all 0.2s ease;
}

.timeline-fab:hover {
  transform: translateY(-2px);
  border-color: rgba(240, 138, 75, 0.55);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(240, 138, 75, 0.15);
}

.timeline-fab--active {
  border-color: rgba(240, 138, 75, 0.6);
  background: linear-gradient(135deg, rgba(240, 138, 75, 0.18), rgba(20, 29, 40, 0.94));
}

.timeline-fab__label {
  white-space: nowrap;
}

.timeline-fab__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.2rem;
  height: 1.2rem;
  border-radius: 999px;
  background: rgba(240, 138, 75, 0.25);
  color: rgba(255, 220, 200, 0.95);
  font-size: 0.65rem;
  padding: 0 0.3rem;
}

/* Timeline floating panel */
.timeline-float-panel {
  position: fixed;
  bottom: 4.5rem;
  right: 1.5rem;
  z-index: 75;
  display: flex;
  flex-direction: column;
  width: min(520px, calc(100vw - 3rem));
  max-height: 65vh;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 1.25rem;
  background: linear-gradient(180deg, rgba(17, 27, 38, 0.97), rgba(9, 15, 23, 0.99));
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(20px);
  overflow: hidden;
  transition: box-shadow 0.2s ease;
}

.timeline-float-panel--dragging {
  box-shadow: 0 32px 100px rgba(0, 0, 0, 0.6), 0 0 0 1px rgba(240, 138, 75, 0.2);
}

.timeline-float-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}

.timeline-float-panel__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0.75rem 1rem 1rem;
}

.timeline-float-panel__body::-webkit-scrollbar {
  width: 4px;
}
.timeline-float-panel__body::-webkit-scrollbar-track {
  background: transparent;
}
.timeline-float-panel__body::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.12);
  border-radius: 2px;
}

/* Slide transition */
.timeline-slide-enter-active,
.timeline-slide-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.timeline-slide-enter-from,
.timeline-slide-leave-to {
  opacity: 0;
  transform: translateY(20px) scale(0.96);
}
</style>
