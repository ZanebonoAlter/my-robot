<script setup lang="ts">
import { Icon } from '@iconify/vue'
import AddSemanticBoardDialog from './AddSemanticBoardDialog.vue'
import BoardCompositionPanel from './BoardCompositionPanel.vue'
import AuxiliaryLabelPool from './AuxiliaryLabelPool.vue'
import UpgradeSuggestionPanel from './UpgradeSuggestionPanel.vue'
import BackfillProgress from './BackfillProgress.vue'
import MatchingConfigDialog from './MatchingConfigDialog.vue'
import NarrativeGenerateDialog from './NarrativeGenerateDialog.vue'
import BoardDailyReportTimeline from './BoardDailyReportTimeline.vue'
import { TagMergePreview } from '~/features/topic-graph/public'
import BoardListSidebar from './BoardListSidebar.vue'
import BoardTimelinePanel from './BoardTimelinePanel.vue'
import BoardEditDialog from './BoardEditDialog.vue'
import ArticlePreviewModal from './ArticlePreviewModal.vue'
import { useTagsPage } from '~/features/tags/composables/useTagsPage'

const {
  // Board CRUD
  boards, selectedBoardId, boardsLoading, boardsError,
  compositionLabels, compositionLoading,
  editingBoard, editLabel, editDescription, editSaving, editError,
  showAddDialog,
  // Timeline
  timelineArticles, timelineLoading, timelineHasMore,
  activeFilterLabelId, filterFeedId, startDate, endDate,
  showDirectionMismatch, timelineSort, quickRange,
  timelineDisplayArticles, feedOptions, selectedTagForDetail,
  // Aux labels
  auxiliaryLabels, auxClusters, auxUnclusteredCount,
  auxLoading, auxSearchQuery, auxStatusFilter, auxPagination,
  // Other state
  contentTab, showUpgradeDialog, showMatchingConfigDialog,
  showGenerateDialog, showMergePreview, showArticlePreview,
  selectedPreviewArticle, previewArticles, loadingPreviewArticle,
  backfillTask,
  upgradeCandidates, upgradeClusters, upgradeSuggestions,
  upgradeLoading, upgradeSuggesting, upgradeBackfillNotice,
  matchingConfig, matchingConfigLoading,
  // Methods - board
  loadBoards: _lb, loadComposition, handleSelectBoard,
  openEditBoard, closeEditBoard, handleSaveBoardEdit,
  handleAddBoard, handleDeleteBoard, handleRemoveComposition,
  // Methods - timeline
  handleLoadMore, handleFilterLabel, handleFilterChange,
  handleSortChange, handleDateInputChange, applyQuickRange,
  toggleMatchDetail,
  // Methods - aux
  loadAuxiliaryLabels, loadClusters, handleUpdatePage,
  handleDisableAuxLabel, handleMergeAuxLabel,
  // Methods - other
  handleUpgradeSuggest, handleSuggestUpgrade, handleExecuteUpgrade,
  handleTriggerBackfill,
  handleOpenMatchingConfig, handleSaveMatchingConfig,
  handleMergeComplete,
  openArticlePreview, closeArticlePreview,
  handleArticleFavorite, handleArticleUpdate,
} = useTagsPage()
</script>

<template>
  <div class="tags-page">
    <!-- Top bar -->
    <div class="tags-topbar">
      <div class="tags-topbar-inner">
        <div class="flex items-center gap-3">
          <NuxtLink to="/" class="tags-back-btn" title="返回首页">
            <Icon icon="mdi:arrow-left" width="16" />
          </NuxtLink>
          <Icon icon="mdi:view-grid" width="18" class="text-white/50" />
          <h1 class="tags-page-title">语义板块管理</h1>
        </div>
      </div>
    </div>

    <div class="tags-main">
      <BoardListSidebar
        :boards="boards"
        :selected-board-id="selectedBoardId"
        :boards-loading="boardsLoading"
        :boards-error="boardsError"
        @select="handleSelectBoard"
        @delete="handleDeleteBoard"
        @edit="openEditBoard"
        @add-board="showAddDialog = true"
        @upgrade-suggest="handleUpgradeSuggest"
        @trigger-backfill="handleTriggerBackfill"
        @open-merge-preview="showMergePreview = true"
        @open-matching-config="handleOpenMatchingConfig"
        @open-generate="showGenerateDialog = true"
      />

      <main class="tags-content">
        <div v-if="selectedBoardId !== null">
          <div class="tags-content-tabs">
            <button type="button" class="tags-content-tab" :class="{ 'tags-content-tab--active': contentTab === 'composition' }" @click="contentTab = 'composition'">
              <Icon icon="mdi:view-dashboard-outline" width="14" /> 板块内容
            </button>
            <button type="button" class="tags-content-tab" :class="{ 'tags-content-tab--active': contentTab === 'daily-reports' }" @click="contentTab = 'daily-reports'">
              <Icon icon="mdi:file-document-outline" width="14" /> 日报
            </button>
            <button type="button" class="tags-content-tab" :class="{ 'tags-content-tab--active': contentTab === 'articles' }" @click="contentTab = 'articles'">
              <Icon icon="mdi:newspaper-variant-outline" width="14" /> 文章
            </button>
          </div>

          <BoardCompositionPanel
            v-if="contentTab === 'composition'"
            :board-id="selectedBoardId"
            :labels="compositionLabels"
            :loading="compositionLoading"
            @remove="handleRemoveComposition"
            @refresh="() => loadComposition(selectedBoardId!)"
          />

          <BoardDailyReportTimeline v-if="contentTab === 'daily-reports'" :board-id="selectedBoardId" @open-article="openArticlePreview" />

          <BoardTimelinePanel
            v-if="contentTab === 'articles'"
            :timeline-articles="timelineArticles"
            :timeline-loading="timelineLoading"
            :timeline-has-more="timelineHasMore"
            :active-filter-label-id="activeFilterLabelId"
            :composition-labels="compositionLabels"
            :selected-board-id="selectedBoardId"
            :timeline-display-articles="timelineDisplayArticles"
            :feed-options="feedOptions"
            :selected-tag-for-detail="selectedTagForDetail"
            :filter-feed-id="filterFeedId"
            :start-date="startDate"
            :end-date="endDate"
            :show-direction-mismatch="showDirectionMismatch"
            :timeline-sort="timelineSort"
            :quick-range="quickRange"
            @load-more="handleLoadMore(selectedBoardId)"
            @filter-label="(id: number | null) => handleFilterLabel(id, selectedBoardId)"
            @filter-change="handleFilterChange(selectedBoardId)"
            @sort-change="(mode: 'quality' | 'time') => handleSortChange(mode, selectedBoardId)"
            @date-input-change="handleDateInputChange(selectedBoardId)"
            @apply-quick-range="(range: 'today' | '3d' | '7d' | '30d') => applyQuickRange(range, selectedBoardId)"
            @open-article-preview="(id: number) => openArticlePreview(id)"
            @toggle-match-detail="(tag) => toggleMatchDetail(tag)"
            @update:filter-feed-id="(id: number | null) => { filterFeedId = id; handleFilterChange(selectedBoardId) }"
            @update:start-date="(v: string) => startDate = v"
            @update:end-date="(v: string) => endDate = v"
            @update:show-direction-mismatch="(v: boolean) => showDirectionMismatch = v"
          />
        </div>

        <div v-else>
          <AuxiliaryLabelPool
            :labels="auxiliaryLabels"
            :clusters="auxClusters"
            :unclustered-count="auxUnclusteredCount"
            :loading="auxLoading"
            :search-query="auxSearchQuery"
            :status-filter="auxStatusFilter"
            :pagination="auxPagination"
            @update:search-query="(v: string) => auxSearchQuery = v"
            @update:status-filter="(v: string) => auxStatusFilter = v"
            @update:page="handleUpdatePage"
            @disable="handleDisableAuxLabel"
            @merge="handleMergeAuxLabel"
            @refresh="loadAuxiliaryLabels"
            @select-cluster="() => {}"
          />
        </div>
      </main>
    </div>

    <div class="tags-bottombar">
      <BackfillProgress :task="backfillTask" />
    </div>

    <AddSemanticBoardDialog :visible="showAddDialog" @confirm="handleAddBoard" @cancel="showAddDialog = false" />
    <BoardEditDialog
      :editing-board="!!editingBoard"
      :edit-label="editLabel"
      :edit-description="editDescription"
      :edit-saving="editSaving"
      :edit-error="editError"
      @update:edit-label="(v: string) => editLabel = v"
      @update:edit-description="(v: string) => editDescription = v"
      @save="handleSaveBoardEdit"
      @close="closeEditBoard"
    />
    <UpgradeSuggestionPanel
      :visible="showUpgradeDialog"
      :candidates="upgradeCandidates"
      :clusters="upgradeClusters"
      :suggestions="upgradeSuggestions"
      :loading="upgradeLoading"
      :suggesting="upgradeSuggesting"
      :backfill-notice="upgradeBackfillNotice"
      @suggest="handleSuggestUpgrade"
      @execute="handleExecuteUpgrade"
      @cancel="showUpgradeDialog = false"
    />
    <MatchingConfigDialog
      :visible="showMatchingConfigDialog"
      :config="matchingConfig"
      :loading="matchingConfigLoading"
      @save="handleSaveMatchingConfig"
      @cancel="showMatchingConfigDialog = false"
    />
    <NarrativeGenerateDialog :visible="showGenerateDialog" :boards="boards" @cancel="showGenerateDialog = false" />
    <TagMergePreview :visible="showMergePreview" @close="showMergePreview = false" @merged="handleMergeComplete" />
    <ArticlePreviewModal
      :visible="showArticlePreview"
      :selected-preview-article="selectedPreviewArticle"
      :preview-articles="previewArticles"
      :loading-preview-article="loadingPreviewArticle"
      @close="closeArticlePreview"
      @favorite="handleArticleFavorite"
      @article-update="handleArticleUpdate"
      @navigate="(a) => selectedPreviewArticle = a"
    />
  </div>
</template>

<style scoped>
.tags-page { display: flex; flex-direction: column; height: 100vh; background: #080c12; color: rgba(255, 255, 255, 0.85); }
.tags-topbar { position: sticky; top: 0; z-index: 30; border-bottom: 1px solid rgba(255, 255, 255, 0.06); background: rgba(8, 12, 18, 0.92); backdrop-filter: blur(16px); }
.tags-topbar-inner { display: flex; align-items: center; justify-content: space-between; max-width: min(1800px, 95vw); margin: 0 auto; padding: 0.75rem 1.5rem; }
.tags-back-btn { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; color: rgba(255, 255, 255, 0.35); text-decoration: none; transition: all 0.12s ease; }
.tags-back-btn:hover { border-color: rgba(255, 255, 255, 0.2); color: rgba(255, 255, 255, 0.6); background: rgba(255, 255, 255, 0.04); }
.tags-page-title { font-family: serif; font-size: 1.1rem; font-weight: 600; color: rgba(255, 255, 255, 0.9); letter-spacing: 0.02em; }
.tags-main { display: flex; flex: 1; min-height: 0; max-width: min(1800px, 95vw); width: 100%; margin: 0 auto; }
.tags-content { flex: 1; min-width: 0; padding: 1.25rem 1.5rem 3.5rem; overflow-y: auto; }
.tags-content-tabs { display: flex; gap: 0.25rem; padding: 0 0 0.75rem; margin-bottom: 1rem; border-bottom: 1px solid rgba(255, 255, 255, 0.06); }
.tags-content-tab { display: flex; align-items: center; gap: 0.35rem; padding: 0.4rem 0.75rem; border: none; border-radius: 8px 8px 0 0; background: none; color: rgba(255, 255, 255, 0.4); font-size: 0.75rem; cursor: pointer; transition: all 0.12s ease; position: relative; }
.tags-content-tab:hover { color: rgba(255, 255, 255, 0.65); background: rgba(255, 255, 255, 0.03); }
.tags-content-tab--active { color: rgba(255, 220, 200, 0.85); background: rgba(240, 138, 75, 0.08); }
.tags-content-tab--active::after { content: ''; position: absolute; bottom: -1px; left: 0; right: 0; height: 2px; background: rgba(240, 138, 75, 0.6); border-radius: 1px; }
.tags-bottombar { position: fixed; bottom: 0; left: 0; right: 0; z-index: 40; display: flex; align-items: center; justify-content: flex-end; gap: 0.75rem; padding: 0.45rem 1.25rem; border-top: 1px solid rgba(255, 255, 255, 0.04); background: rgba(8, 12, 18, 0.88); backdrop-filter: blur(12px); }
</style>
