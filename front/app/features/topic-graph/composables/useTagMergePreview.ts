import { ref, computed, watch } from 'vue'
import { useTagMergePreviewApi } from '~/api/tagMergePreview'
import type { MergeGroup, MergeSuggestion, EvaluateProgress, ScanProgress, LLMVerdict } from '~/types/tagMerge'

export function useTagMergePreview(visibleSource: () => boolean, emit: { merged: () => void; close: () => void }) {
  const api = useTagMergePreviewApi()

  // --- State ---
  const loading = ref(false)
  const groups = ref<MergeGroup[]>([])
  const error = ref<string | null>(null)

  const evaluating = ref(false)
  const evalProgress = ref<EvaluateProgress | null>(null)
  let evalEs: EventSource | null = null

  const scanning = ref(false)
  const scanProgress = ref<ScanProgress | null>(null)
  let scanEs: EventSource | null = null

  const selectedKeys = ref<Set<string>>(new Set())
  const merging = ref(false)
  const mergedCount = ref(0)
  const mergeProgress = ref<{ done: number; total: number } | null>(null)

  const searchingGroupId = ref<number | null>(null)
  const searchQuery = ref('')
  const searchResults = ref<Array<{ id: number; label: string; slug: string; category: string; feed_count: number }>>([])
  const searchLoading = ref(false)
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  // --- Computed ---
  const selectedCount = computed(() => selectedKeys.value.size)
  const hasMergeableSuggestions = computed(() => {
    return groups.value?.some(g =>
      g.suggestions?.some(s => {
        const verdict = parseVerdict(s.llm_verdict)
        return !verdict || verdict.should_merge
      })
    ) ?? false
  })

  // --- Selection helpers ---
  function sugKey(targetTagId: number, newTagId: number): string {
    return `${targetTagId}:${newTagId}`
  }

  function toggleSelect(targetTagId: number, newTagId: number) {
    const key = sugKey(targetTagId, newTagId)
    const next = new Set(selectedKeys.value)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    selectedKeys.value = next
  }

  function isSugSelected(targetTagId: number, newTagId: number): boolean {
    return selectedKeys.value.has(sugKey(targetTagId, newTagId))
  }

  function selectAllInGroup(group: MergeGroup) {
    const next = new Set(selectedKeys.value)
    for (const sug of group.suggestions) next.add(sugKey(group.target_tag_id, sug.new_tag_id))
    selectedKeys.value = next
  }

  function deselectAllInGroup(group: MergeGroup) {
    const next = new Set(selectedKeys.value)
    for (const sug of group.suggestions) next.delete(sugKey(group.target_tag_id, sug.new_tag_id))
    selectedKeys.value = next
  }

  function isGroupAllSelected(group: MergeGroup): boolean {
    return group.suggestions.length > 0 && group.suggestions.every(s => isSugSelected(group.target_tag_id, s.new_tag_id))
  }

  function selectAllMergeable() {
    const next = new Set(selectedKeys.value)
    for (const group of groups.value) {
      for (const sug of group.suggestions) {
        const verdict = parseVerdict(sug.llm_verdict)
        if (!verdict || verdict.should_merge) next.add(sugKey(group.target_tag_id, sug.new_tag_id))
      }
    }
    selectedKeys.value = next
  }

  function clearSelection() { selectedKeys.value = new Set() }

  // --- Load groups ---
  async function loadGroups() {
    loading.value = true; error.value = null
    try {
      const response = await api.loadMergeGroups({ limit: 200 })
      if (response.success && response.data) groups.value = response.data.groups || []
      else error.value = response.error || '加载失败'
    } catch (err) { error.value = err instanceof Error ? err.message : '加载失败' }
    finally { loading.value = false }
  }

  // --- SSE helpers ---
  function createEvalSSE() {
    evaluating.value = true
    evalEs = api.createEvaluateEventSource((progress: EvaluateProgress) => {
      evalProgress.value = progress
      if (progress.status === 'done' || progress.status === 'error') {
        evalEs?.close(); evalEs = null; evaluating.value = false
        if (progress.status === 'done') void loadGroups()
      }
    })
  }

  function createScanSSE() {
    scanning.value = true
    scanEs = api.createScanEventSource((progress: ScanProgress) => {
      scanProgress.value = progress
      if (progress.status === 'done' || progress.status === 'error') {
        scanEs?.close(); scanEs = null; scanning.value = false
        if (progress.status === 'done') void loadGroups()
      }
    })
  }

  // --- Evaluate ---
  async function triggerEvaluate() {
    evaluating.value = true; evalProgress.value = null; error.value = null
    createEvalSSE()
    const response = await api.triggerEvaluate()
    if (!response.success) {
      if (response.error?.includes('already in progress')) return
      evalEs?.close(); evalEs = null; evaluating.value = false
      error.value = response.error || '评估失败'
    }
  }

  function cancelEvaluate() {
    evalEs?.close(); evalEs = null; evaluating.value = false; evalProgress.value = null
  }

  // --- Full scan ---
  async function triggerFullScan() {
    scanning.value = true; scanProgress.value = null; error.value = null
    createScanSSE()
    const response = await api.triggerFullScan()
    if (!response.success) {
      if (response.error?.includes('already in progress')) return
      scanEs?.close(); scanEs = null; scanning.value = false
      error.value = response.error || '扫描失败'
    }
  }

  function cancelScan() {
    scanEs?.close(); scanEs = null; scanning.value = false; scanProgress.value = null
  }

  // --- Batch merge ---
  async function mergeSelected() {
    if (selectedKeys.value.size === 0) return
    merging.value = true
    mergeProgress.value = { done: 0, total: selectedKeys.value.size }
    let count = 0
    try {
      for (const key of selectedKeys.value) {
        const [targetStr, newStr] = key.split(':')
        const targetTagId = Number(targetStr); const newTagId = Number(newStr)
        const group = groups.value.find(g => g.target_tag_id === targetTagId)
        const sug = group?.suggestions.find(s => s.new_tag_id === newTagId)
        if (!group || !sug) { mergeProgress.value!.done++; continue }
        const verdict = parseVerdict(sug.llm_verdict)
        const newName = verdict?.suggested_name || group.target_label
        try {
          await api.mergeTagsWithCustomName({ sourceTagId: newTagId, targetTagId, newName })
          count++
        } catch (err) { console.error(`Failed to merge ${sug.new_label} → ${group.target_label}:`, err) }
        mergeProgress.value!.done++
      }
    } finally { mergeProgress.value = null }
    mergedCount.value += count
    selectedKeys.value = new Set()
    void loadGroups()
    merging.value = false
  }

  // --- Dismiss ---
  async function removeSuggestion(sug: MergeSuggestion, group: MergeGroup) {
    try {
      await api.dismissSuggestion(sug.new_tag_id, group.target_tag_id)
      group.suggestions = group.suggestions.filter(s => s.id !== sug.id)
      if (group.suggestions.length === 0) groups.value = groups.value.filter(g => g.target_tag_id !== group.target_tag_id)
    } catch (err) { console.error('Failed to remove suggestion:', err) }
  }

  // --- Search ---
  function openSearch(target_tag_id: number) { searchingGroupId.value = target_tag_id; searchQuery.value = ''; searchResults.value = [] }
  function closeSearch() { searchingGroupId.value = null; searchQuery.value = ''; searchResults.value = [] }

  async function onSearchInput() {
    if (searchTimer) clearTimeout(searchTimer)
    if (searchQuery.value.trim().length < 1) { searchResults.value = []; return }
    searchTimer = setTimeout(async () => {
      searchLoading.value = true
      try {
        const response = await api.searchTags(searchQuery.value)
        if (response.success && response.data) {
          const group = groups.value.find(g => g.target_tag_id === searchingGroupId.value)
          const existingIds = new Set([searchingGroupId.value, ...(group?.suggestions.map(s => s.new_tag_id) || [])])
          searchResults.value = (response.data as Array<{ id: number; label: string; slug: string; category: string; feed_count: number }>)
            .filter(t => !existingIds.has(t.id))
        }
      } catch (err) { console.error('Search failed:', err) }
      finally { searchLoading.value = false }
    }, 300)
  }

  async function addTagToGroup(tagId: number) {
    if (!searchingGroupId.value) return
    try {
      await api.addToGroup(searchingGroupId.value, tagId)
      closeSearch()
      void loadGroups()
    } catch (err) { console.error('Add to group failed:', err) }
  }

  // --- Helpers ---
  function parseVerdict(raw: string | null): LLMVerdict | null {
    if (!raw) return null
    try { return JSON.parse(raw) } catch { return null }
  }

  function formatSimilarity(similarity: number) {
    return `${Math.round(similarity * 100)}%`
  }

  // --- Recover ---
  async function recoverRunningState() {
    try {
      const response = await api.getMergePreviewStatus()
      if (response.success && response.data) {
        if (response.data.eval_running) createEvalSSE()
        if (response.data.scan_running) createScanSSE()
      }
    } catch (err) { console.error('查询合并预览状态失败:', err) }
  }

  // --- Lifecycle ---
  watch(visibleSource, (isVisible) => {
    if (isVisible) {
      void recoverRunningState()
      void loadGroups()
    }
  }, { immediate: true })

  function handleClose() {
    if (mergedCount.value > 0) emit.merged()
    emit.close()
    groups.value = []
    selectedKeys.value = new Set()
    mergedCount.value = 0
    error.value = null
  }

  return {
    loading, groups, error, evaluating, evalProgress, scanning, scanProgress,
    selectedKeys, merging, mergedCount, mergeProgress,
    searchingGroupId, searchQuery, searchResults, searchLoading,
    selectedCount, hasMergeableSuggestions,
    toggleSelect, isSugSelected, selectAllInGroup, deselectAllInGroup,
    isGroupAllSelected, selectAllMergeable, clearSelection,
    loadGroups, triggerEvaluate, cancelEvaluate,
    triggerFullScan, cancelScan, mergeSelected,
    removeSuggestion, openSearch, closeSearch, onSearchInput, addTagToGroup,
    parseVerdict, formatSimilarity,
    handleClose,
  }
}
