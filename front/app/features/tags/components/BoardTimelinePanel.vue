<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { BoardArticle, BoardArticleTag, AuxiliaryLabelItem } from '~/api/semanticBoards'
import type { RssFeed } from '~/types'
import MatchDetailPanel from './MatchDetailPanel.vue'

const props = defineProps<{
  timelineArticles: BoardArticle[]
  timelineLoading: boolean
  timelineHasMore: boolean
  activeFilterLabelId: number | null
  compositionLabels: AuxiliaryLabelItem[]
  selectedBoardId: number | null
  timelineDisplayArticles: BoardArticle[]
  feedOptions: RssFeed[]
  selectedTagForDetail: BoardArticleTag | null
  filterFeedId: number | null
  startDate: string
  endDate: string
  showDirectionMismatch: boolean
  timelineSort: 'quality' | 'time'
  quickRange: string | null
}>()

const emit = defineEmits<{
  'load-more': []
  'filter-label': [id: number | null]
  'filter-change': []
  'sort-change': [mode: 'quality' | 'time']
  'date-input-change': []
  'apply-quick-range': [range: 'today' | '3d' | '7d' | '30d']
  'open-article-preview': [id: number]
  'toggle-match-detail': [tag: BoardArticleTag]
  'update:filter-feed-id': [id: number | null]
  'update:start-date': [val: string]
  'update:end-date': [val: string]
  'update:show-direction-mismatch': [val: boolean]
  'update:timeline-sort': [mode: 'quality' | 'time']
}>()

function matchReasonColor(reason: string, downgraded?: boolean): string {
  const colors: Record<string, string> = {
    direct_hit: '#22c55e',
    hit_rate: '#3b82f6',
    max_sim: '#f59e0b',
    weighted: '#94a3b8',
  }
  const color = colors[reason] || '#94a3b8'
  return downgraded ? color + '80' : color
}

function matchInfoLabel(tag: BoardArticleTag): string {
  const labels: Record<string, string> = {
    direct_hit: '直接命中',
    hit_rate: '命中率',
    max_sim: '相似度',
    weighted: '综合',
  }
  return `${labels[tag.match_reason] || tag.match_reason} ${tag.score.toFixed(2)}${tag.downgraded ? '↓' : ''}`
}

function strongestMatch(tags: BoardArticleTag[]): BoardArticleTag | null {
  if (!tags?.length) return null
  const [first, ...rest] = tags
  if (!first) return null
  return rest.reduce((best, t) => t.score > best.score ? t : best, first)
}

function isSelectedDetailTag(tag: BoardArticleTag): boolean {
  return props.selectedTagForDetail?.id === tag.id
}
</script>

<template>
  <div class="tags-articles-layout">
    <div class="tags-timeline">
      <div class="tags-timeline-header">
        <Icon icon="mdi:timeline-clock-outline" width="15" class="text-[rgba(240,138,75,0.8)]" />
        <span class="tags-timeline-title">相关文章</span>
        <span v-if="timelineArticles.length" class="tags-timeline-count">{{ timelineArticles.length }} 篇</span>
        <div class="tags-sort-toggle">
          <button
            type="button"
            class="tags-sort-btn"
            :class="{ 'tags-sort-btn--active': timelineSort === 'quality' }"
            @click="emit('sort-change', 'quality')"
          >
            <Icon icon="mdi:star-outline" width="13" /> 质量
          </button>
          <button
            type="button"
            class="tags-sort-btn"
            :class="{ 'tags-sort-btn--active': timelineSort === 'time' }"
            @click="emit('sort-change', 'time')"
          >
            <Icon icon="mdi:clock-outline" width="13" /> 时间
          </button>
        </div>
        <label class="tags-direction-toggle">
          <input :checked="showDirectionMismatch" type="checkbox" @change="emit('update:show-direction-mismatch', ($event.target as HTMLInputElement).checked); emit('filter-change')" />
          显示方向不符
        </label>
      </div>

      <!-- Filter chips -->
      <div v-if="compositionLabels.length > 0 || selectedBoardId !== null" class="tags-filter-chips">
        <button
          v-if="compositionLabels.length > 0"
          type="button"
          class="tags-filter-chip"
          :class="{ 'tags-filter-chip--active': activeFilterLabelId === null }"
          @click="emit('filter-label', null)"
        >
          全部
        </button>
        <button
          v-for="label in compositionLabels"
          :key="label.id"
          type="button"
          class="tags-filter-chip"
          :class="{ 'tags-filter-chip--active': activeFilterLabelId === label.id }"
          @click="emit('filter-label', label.id)"
        >
          {{ label.label }}
        </button>
        <div class="tags-quick-range">
          <button
            v-for="opt in [
              { key: 'today', label: '今天' },
              { key: '3d', label: '3天' },
              { key: '7d', label: '7天' },
              { key: '30d', label: '30天' },
            ]"
            :key="opt.key"
            type="button"
            class="tags-filter-chip"
            :class="{ 'tags-filter-chip--active': quickRange === opt.key }"
            @click="emit('apply-quick-range', opt.key as 'today' | '3d' | '7d' | '30d')"
          >
            {{ opt.label }}
          </button>
        </div>
        <select :value="filterFeedId" class="tags-filter-select" @change="emit('update:filter-feed-id', Number(($event.target as HTMLSelectElement).value) || null); emit('filter-change')">
          <option :value="null">全部来源</option>
          <option v-for="feed in feedOptions" :key="feed.id" :value="Number(feed.id)">{{ feed.title }}</option>
        </select>
        <input type="date" :value="startDate" class="tags-filter-date" @change="emit('update:start-date', ($event.target as HTMLInputElement).value); emit('date-input-change')" />
        <input type="date" :value="endDate" class="tags-filter-date" @change="emit('update:end-date', ($event.target as HTMLInputElement).value); emit('date-input-change')" />
      </div>

      <div v-if="timelineLoading && timelineArticles.length === 0" class="tags-timeline-loading">
        <div v-for="i in 3" :key="i" class="th-skeleton" />
      </div>

      <div v-else-if="timelineArticles.length === 0" class="tags-timeline-empty">
        <Icon icon="mdi:newspaper-variant-outline" width="28" class="text-white/15" />
        <p>暂无相关文章</p>
      </div>

      <div v-else class="tags-timeline-list">
        <div
          v-for="article in timelineDisplayArticles"
          :key="article.id"
          class="tags-timeline-item"
          @click="emit('open-article-preview', article.id)"
        >
          <div class="tags-timeline-item-meta">
            <span class="tags-timeline-item-date">
              {{ new Date(article.pub_date).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' }) }}
            </span>
            <span v-if="article.feed_name" class="tags-timeline-item-feed-name">{{ article.feed_name }}</span>
          </div>
          <div class="tags-timeline-item-body">
            <div class="tags-timeline-item-content">
              <span class="tags-timeline-item-title">{{ article.title }}</span>
              <div v-if="article.filtered_tags?.length" class="tags-timeline-item-tags">
                <button
                  v-for="tag in article.filtered_tags"
                  :key="tag.id"
                  type="button"
                  class="tags-timeline-tag-chip"
                  :class="{
                    'tags-timeline-tag-chip--selected': isSelectedDetailTag(tag),
                    'tags-timeline-tag-chip--direction-mismatch': tag.direction_mismatch,
                  }"
                  :style="{ borderColor: matchReasonColor(tag.match_reason, tag.downgraded) }"
                  :title="matchInfoLabel(tag)"
                  @click.stop="emit('toggle-match-detail', tag)"
                >
                  {{ tag.label }} {{ tag.score.toFixed(2) }}{{ tag.downgraded ? '↓' : '' }}{{ tag.direction_mismatch ? '⊘' : '' }}
                </button>
              </div>
            </div>
            <span
              v-if="strongestMatch(article.filtered_tags)"
              class="tags-timeline-item-match-info"
              :style="{ color: matchReasonColor(strongestMatch(article.filtered_tags)!.match_reason) }"
            >
              {{ matchInfoLabel(strongestMatch(article.filtered_tags)!) }}
            </span>
          </div>
        </div>
      </div>
      <button
        v-if="timelineHasMore"
        type="button"
        class="tags-timeline-more"
        :disabled="timelineLoading"
        @click="emit('load-more')"
      >
        <template v-if="timelineLoading">加载中...</template>
        <template v-else>加载更多</template>
      </button>
    </div>

    <Transition name="match-detail-panel">
      <MatchDetailPanel
        v-if="selectedTagForDetail && selectedBoardId !== null"
        :board-id="selectedBoardId"
        :tag="selectedTagForDetail"
        class="tags-match-detail-panel"
        @close="selectedTagForDetail = null"
      />
    </Transition>
  </div>
</template>

<style scoped>
.tags-articles-layout { display: flex; align-items: flex-start; gap: 1rem; min-width: 0; }
.tags-articles-layout .tags-timeline { flex: 1; min-width: 0; }

.tags-timeline { margin-top: 0; }
.tags-timeline-header { display: flex; align-items: center; gap: 0.4rem; margin-bottom: 0.75rem; }
.tags-timeline-title { font-family: serif; font-size: 0.9rem; color: rgba(255, 255, 255, 0.8); }
.tags-timeline-count { font-size: 0.65rem; color: rgba(255, 255, 255, 0.3); padding: 0.1rem 0.45rem; border-radius: 999px; background: rgba(255, 255, 255, 0.06); }
.tags-direction-toggle { display: inline-flex; align-items: center; gap: 0.35rem; margin-left: auto; font-size: 0.7rem; color: rgba(255, 255, 255, 0.38); cursor: pointer; }
.tags-direction-toggle input { cursor: pointer; }
.tags-sort-toggle { display: inline-flex; gap: 0; border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 6px; overflow: hidden; }
.tags-sort-btn { display: inline-flex; align-items: center; gap: 0.25rem; padding: 0.2rem 0.5rem; font-size: 0.68rem; color: rgba(255, 255, 255, 0.4); background: transparent; border: none; cursor: pointer; transition: all 0.15s; }
.tags-sort-btn--active { color: rgba(240, 138, 75, 0.9); background: rgba(240, 138, 75, 0.1); }
.tags-sort-btn:hover:not(.tags-sort-btn--active) { color: rgba(255, 255, 255, 0.6); }
.tags-articles-layout .tags-timeline { margin-top: 0; }

.tags-timeline-loading { display: flex; flex-direction: column; gap: 0.5rem; }
.th-skeleton { height: 36px; border-radius: 10px; background: rgba(255, 255, 255, 0.03); animation: thPulse 1.5s ease-in-out infinite; }
@keyframes thPulse { 0%, 100% { opacity: 0.4; } 50% { opacity: 0.8; } }
.tags-timeline-empty { display: flex; flex-direction: column; align-items: center; gap: 0.4rem; padding: 2.5rem 0; color: rgba(255, 255, 255, 0.3); font-size: 0.8rem; }
.tags-timeline-list { display: flex; flex-direction: column; gap: 0.25rem; }

.tags-timeline-item { display: flex; flex-direction: column; gap: 0.25rem; padding: 0.5rem 0.65rem; border-radius: 8px; cursor: pointer; transition: background 0.12s ease; }
.tags-timeline-item:hover { background: rgba(255, 255, 255, 0.03); }
.tags-timeline-item-meta { display: flex; align-items: center; gap: 0.5rem; }
.tags-timeline-item-date { font-size: 0.65rem; color: rgba(255, 255, 255, 0.3); white-space: nowrap; }
.tags-timeline-item-feed-name { font-size: 0.62rem; color: rgba(255, 255, 255, 0.25); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tags-timeline-item-content { display: flex; flex-direction: column; gap: 0.2rem; min-width: 0; }
.tags-timeline-item-body { display: flex; align-items: flex-start; gap: 0.5rem; }
.tags-timeline-item-title { font-size: 0.8rem; color: rgba(255, 255, 255, 0.7); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; transition: color 0.12s ease; }
.tags-timeline-item-title:hover { color: rgba(255, 220, 200, 0.9); }
.tags-timeline-item-tags { display: flex; flex-wrap: wrap; gap: 0.25rem; margin-top: 0.15rem; }

.tags-timeline-tag-chip { display: inline-flex; align-items: center; padding: 0.1rem 0.4rem; font-size: 0.62rem; font-family: inherit; border-radius: 4px; border: 1px solid rgba(255, 255, 255, 0.08); background: rgba(255, 255, 255, 0.06); color: rgba(255, 255, 255, 0.5); cursor: pointer; transition: background 0.12s ease, box-shadow 0.12s ease; }
.tags-timeline-tag-chip:hover { background: rgba(255, 255, 255, 0.1); }
.tags-timeline-tag-chip--selected { box-shadow: 0 0 0 2px rgba(240, 138, 75, 0.35); background: rgba(240, 138, 75, 0.12); }
.tags-timeline-tag-chip--direction-mismatch { border-style: dashed; opacity: 0.65; }
.tags-timeline-item-match-info { flex-shrink: 0; font-size: 0.62rem; font-weight: 500; white-space: nowrap; margin-left: auto; padding-left: 0.5rem; }
.tags-timeline-more { display: flex; align-items: center; justify-content: center; width: 100%; margin-top: 0.75rem; padding: 0.5rem; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 10px; background: none; color: rgba(255, 255, 255, 0.4); font-size: 0.75rem; cursor: pointer; transition: all 0.12s ease; }
.tags-timeline-more:hover { border-color: rgba(255, 255, 255, 0.15); color: rgba(255, 255, 255, 0.6); background: rgba(255, 255, 255, 0.03); }
.tags-timeline-more:disabled { opacity: 0.5; cursor: not-allowed; }

.tags-filter-select { padding: 0.2rem 0.4rem; border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.08); background: rgba(255, 255, 255, 0.04); color: rgba(255, 255, 255, 0.45); font-size: 0.7rem; cursor: pointer; transition: all 0.12s ease; }
.tags-filter-select:hover { border-color: rgba(255, 255, 255, 0.18); color: rgba(255, 255, 255, 0.65); }
.tags-filter-select option { background: #1a1f2a; color: rgba(255, 255, 255, 0.85); }
.tags-filter-date { padding: 0.2rem 0.4rem; border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.08); background: rgba(255, 255, 255, 0.04); color: rgba(255, 255, 255, 0.45); font-size: 0.7rem; cursor: pointer; transition: all 0.12s ease; }
.tags-filter-date:hover { border-color: rgba(255, 255, 255, 0.18); color: rgba(255, 255, 255, 0.65); }
.tags-filter-date::-webkit-calendar-picker-indicator { filter: invert(0.5); }

.tags-filter-chips { display: flex; flex-wrap: wrap; gap: 0.35rem; margin-bottom: 0.75rem; padding-bottom: 0.75rem; border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.tags-filter-chip { padding: 0.2rem 0.55rem; border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.08); background: none; color: rgba(255, 255, 255, 0.45); font-size: 0.7rem; cursor: pointer; transition: all 0.12s ease; }
.tags-filter-chip:hover { border-color: rgba(255, 255, 255, 0.18); color: rgba(255, 255, 255, 0.65); background: rgba(255, 255, 255, 0.03); }
.tags-filter-chip--active { border-color: rgba(240, 138, 75, 0.45); color: rgba(255, 220, 200, 0.85); background: rgba(240, 138, 75, 0.1); }
.tags-quick-range { display: flex; gap: 0.25rem; margin-right: 0.5rem; padding-right: 0.5rem; border-right: 1px solid rgba(255, 255, 255, 0.08); }

.tags-match-detail-panel { width: 320px; flex-shrink: 0; position: sticky; top: 1rem; align-self: flex-start; max-height: calc(100vh - 6rem); overflow-y: auto; }
.match-detail-panel-enter-active, .match-detail-panel-leave-active { transition: opacity 0.16s ease, transform 0.16s ease; }
.match-detail-panel-enter-from, .match-detail-panel-leave-to { opacity: 0; transform: translateX(12px); }
</style>
