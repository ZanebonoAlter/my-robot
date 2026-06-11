import { ref, onMounted, onUnmounted } from 'vue'
import { useSemanticBoardsApi, type UpgradeCandidate, type UpgradeCluster, type UpgradeSuggestion, type BackfillTask, type MatchingConfig } from '~/api/semanticBoards'
import { useArticlesApi } from '~/api/articles'
import { normalizeArticle, type ArticlePayload } from '~/api/normalizers/article'
import type { Article } from '~/types'
import { useBoardCRUD } from './useBoardCRUD'
import { useBoardTimeline } from './useBoardTimeline'
import { useAuxiliaryLabels } from './useAuxiliaryLabels'

export function useTagsPage() {
  const boardCRUD = useBoardCRUD()
  const timeline = useBoardTimeline()
  const auxLabels = useAuxiliaryLabels()

  const sbApi = useSemanticBoardsApi()
  const articlesApi = useArticlesApi()

  // Tab / dialog state
  const contentTab = ref<'composition' | 'daily-reports' | 'articles'>('composition')
  const showUpgradeDialog = ref(false)
  const showMatchingConfigDialog = ref(false)
  const showGenerateDialog = ref(false)
  const showMergePreview = ref(false)
  const showArticlePreview = ref(false)

  // Article preview
  const selectedPreviewArticle = ref<Article | null>(null)
  const previewArticles = ref<Article[]>([])
  const loadingPreviewArticle = ref(false)

  // Upgrade
  const upgradeCandidates = ref<UpgradeCandidate[]>([])
  const upgradeClusters = ref<UpgradeCluster[]>([])
  const upgradeSuggestions = ref<UpgradeSuggestion[]>([])
  const upgradeLoading = ref(false)
  const upgradeSuggesting = ref(false)
  const upgradeBackfillNotice = ref(false)

  // Backfill
  const backfillTask = ref<BackfillTask | null>(null)
  let backfillPollTimer: ReturnType<typeof setInterval> | null = null

  // Matching config
  const matchingConfig = ref<MatchingConfig | null>(null)
  const matchingConfigLoading = ref(false)

  // ---- Integration: boardCRUD + timeline ----
  function handleSelectBoard(id: number | null) {
    boardCRUD.handleSelectBoard(id)
    if (id !== null) {
      void timeline.loadTimelineArticles(id)
    } else {
      timeline.timelineArticles.value = []
    }
  }

  // ---- Article preview ----
  async function openArticlePreview(articleId: number) {
    loadingPreviewArticle.value = true
    showArticlePreview.value = true
    try {
      const response = await articlesApi.getArticle(articleId)
      if (!response.success || !response.data) return
      selectedPreviewArticle.value = normalizeArticle(response.data as unknown as ArticlePayload)
      previewArticles.value = []
    } catch (error) {
      console.error('Failed to open article preview:', error)
    } finally {
      loadingPreviewArticle.value = false
    }
  }

  function closeArticlePreview() {
    showArticlePreview.value = false
    selectedPreviewArticle.value = null
  }

  async function handleArticleFavorite(articleId: string) {
    const response = await articlesApi.updateArticle(Number(articleId), { favorite: !selectedPreviewArticle.value?.favorite })
    if (!response.success) return
    if (selectedPreviewArticle.value?.id === articleId) {
      selectedPreviewArticle.value = {
        ...selectedPreviewArticle.value,
        favorite: !selectedPreviewArticle.value.favorite,
      }
    }
  }

  function handleArticleUpdate(articleId: string, updates: Partial<Article>) {
    if (selectedPreviewArticle.value?.id === articleId) {
      Object.assign(selectedPreviewArticle.value, updates)
    }
  }

  // ---- Upgrade ----
  async function handleUpgradeSuggest() {
    upgradeLoading.value = true
    showUpgradeDialog.value = true
    upgradeBackfillNotice.value = false
    const res = await sbApi.getUpgradeCandidates()
    if (res.success && res.data) {
      upgradeCandidates.value = res.data.candidates
      upgradeClusters.value = res.data.clusters
    }
    upgradeLoading.value = false
  }

  async function handleSuggestUpgrade() {
    upgradeSuggesting.value = true
    upgradeBackfillNotice.value = false
    const res = await sbApi.suggestUpgrade()
    if (res.success && res.data) {
      upgradeSuggestions.value = res.data.suggestions
    }
    upgradeSuggesting.value = false
  }

  async function handleExecuteUpgrade(suggestion: UpgradeSuggestion, index: number) {
    if (suggestion.decision === 'skip') return
    const res = await sbApi.executeUpgrade({
      decision: suggestion.decision,
      board_label: suggestion.board_label,
      description: suggestion.description,
      target_board_id: suggestion.target_board_id,
      auxiliary_label_ids: suggestion.auxiliary_label_ids,
    })
    if (res.success) {
      upgradeSuggestions.value.splice(index, 1)
      upgradeBackfillNotice.value = true
      void boardCRUD.loadBoards()
    }
  }

  // ---- Backfill ----
  async function handleTriggerBackfill() {
    const res = await sbApi.triggerBackfill({ mode: 'all' })
    if (res.success && res.data) {
      backfillTask.value = res.data
      startBackfillPolling(res.data.id)
    }
  }

  function startBackfillPolling(id: string) {
    if (backfillPollTimer) clearInterval(backfillPollTimer)
    backfillPollTimer = setInterval(async () => {
      const res = await sbApi.getBackfillStatus(id)
      if (res.success && res.data) {
        backfillTask.value = res.data
        if (res.data.status === 'completed' || res.data.status === 'failed') {
          if (backfillPollTimer) clearInterval(backfillPollTimer)
          backfillPollTimer = null
        }
      }
    }, 2000)
  }

  // ---- Matching config ----
  async function handleOpenMatchingConfig() {
    matchingConfigLoading.value = true
    showMatchingConfigDialog.value = true
    const res = await sbApi.getMatchingConfig()
    if (res.success && res.data) {
      matchingConfig.value = res.data
    }
    matchingConfigLoading.value = false
  }

  async function handleSaveMatchingConfig(data: Partial<MatchingConfig>) {
    const res = await sbApi.updateMatchingConfig(data)
    if (res.success) showMatchingConfigDialog.value = false
  }

  // ---- Aux label actions ----
  function handleMergeComplete() {
    void auxLabels.loadAuxiliaryLabels()
    void boardCRUD.loadBoards()
  }

  // ---- Lifecycle ----
  onMounted(() => {
    void boardCRUD.loadBoards()
    void auxLabels.loadAuxiliaryLabels()
    void auxLabels.loadClusters()
  })

  onUnmounted(() => {
    if (backfillPollTimer) clearInterval(backfillPollTimer)
  })

  return {
    // Board CRUD
    boards: boardCRUD.boards,
    selectedBoardId: boardCRUD.selectedBoardId,
    boardsLoading: boardCRUD.boardsLoading,
    boardsError: boardCRUD.boardsError,
    compositionLabels: boardCRUD.compositionLabels,
    compositionLoading: boardCRUD.compositionLoading,
    editingBoard: boardCRUD.editingBoard,
    editLabel: boardCRUD.editLabel,
    editDescription: boardCRUD.editDescription,
    editSaving: boardCRUD.editSaving,
    editError: boardCRUD.editError,
    showAddDialog: boardCRUD.showAddDialog,
    loadBoards: boardCRUD.loadBoards,
    loadComposition: boardCRUD.loadComposition,
    handleSelectBoard,
    openEditBoard: boardCRUD.openEditBoard,
    closeEditBoard: boardCRUD.closeEditBoard,
    handleSaveBoardEdit: boardCRUD.handleSaveBoardEdit,
    handleAddBoard: boardCRUD.handleAddBoard,
    handleDeleteBoard: boardCRUD.handleDeleteBoard,
    handleRemoveComposition: boardCRUD.handleRemoveComposition,
    sourceIcon: boardCRUD.sourceIcon,
    sourceTitle: boardCRUD.sourceTitle,

    // Timeline
    timelineArticles: timeline.timelineArticles,
    timelineLoading: timeline.timelineLoading,
    timelineHasMore: timeline.timelineHasMore,
    activeFilterLabelId: timeline.activeFilterLabelId,
    filterFeedId: timeline.filterFeedId,
    startDate: timeline.startDate,
    endDate: timeline.endDate,
    showDirectionMismatch: timeline.showDirectionMismatch,
    timelineSort: timeline.timelineSort,
    quickRange: timeline.quickRange,
    timelineDisplayArticles: timeline.timelineDisplayArticles,
    feedOptions: timeline.feedOptions,
    selectedTagForDetail: timeline.selectedTagForDetail,
    handleLoadMore: timeline.handleLoadMore,
    handleFilterLabel: timeline.handleFilterLabel,
    handleFilterChange: timeline.handleFilterChange,
    handleSortChange: timeline.handleSortChange,
    handleDateInputChange: timeline.handleDateInputChange,
    applyQuickRange: timeline.applyQuickRange,
    toggleMatchDetail: timeline.toggleMatchDetail,

    // Auxiliary labels
    auxiliaryLabels: auxLabels.auxiliaryLabels,
    auxClusters: auxLabels.auxClusters,
    auxUnclusteredCount: auxLabels.auxUnclusteredCount,
    auxLoading: auxLabels.auxLoading,
    auxSearchQuery: auxLabels.auxSearchQuery,
    auxStatusFilter: auxLabels.auxStatusFilter,
    auxPagination: auxLabels.auxPagination,
    loadAuxiliaryLabels: auxLabels.loadAuxiliaryLabels,
    loadClusters: auxLabels.loadClusters,
    handleUpdatePage: auxLabels.handleUpdatePage,
    handleDisableAuxLabel: auxLabels.handleDisableAuxLabel,
    handleMergeAuxLabel: auxLabels.handleMergeAuxLabel,

    // Additional state
    contentTab, showUpgradeDialog, showMatchingConfigDialog,
    showGenerateDialog, showMergePreview, showArticlePreview,

    // Article preview
    selectedPreviewArticle, previewArticles, loadingPreviewArticle,
    openArticlePreview, closeArticlePreview,
    handleArticleFavorite, handleArticleUpdate,

    // Upgrade
    upgradeCandidates, upgradeClusters, upgradeSuggestions,
    upgradeLoading, upgradeSuggesting, upgradeBackfillNotice,
    handleUpgradeSuggest, handleSuggestUpgrade, handleExecuteUpgrade,

    // Backfill
    backfillTask, handleTriggerBackfill,

    // Matching config
    matchingConfig, matchingConfigLoading,
    handleOpenMatchingConfig, handleSaveMatchingConfig,

    // Composite methods
    handleMergeComplete,
  }
}
