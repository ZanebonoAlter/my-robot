import { ref, watch } from 'vue'
import { useAuxiliaryLabelsApi, type AuxiliaryLabel, type AuxiliaryLabelCluster } from '~/api/auxiliaryLabels'

export function useAuxiliaryLabels() {
  const auxApi = useAuxiliaryLabelsApi()

  const auxiliaryLabels = ref<AuxiliaryLabel[]>([])
  const auxClusters = ref<AuxiliaryLabelCluster[]>([])
  const auxUnclusteredCount = ref(0)
  const auxLoading = ref(false)
  const auxSearchQuery = ref('')
  const auxStatusFilter = ref('')
  const auxPage = ref(1)
  const auxPagination = ref<{ page: number; pages: number; total: number } | null>(null)
  const auxPerPage = 50

  async function loadAuxiliaryLabels() {
    auxLoading.value = true
    const res = await auxApi.getLabels({
      search: auxSearchQuery.value || undefined,
      status: auxStatusFilter.value || undefined,
      page: auxPage.value,
      per_page: auxPerPage,
    })
    if (res.success && res.data) {
      auxiliaryLabels.value = res.data.items
      if (res.pagination) {
        auxPagination.value = { page: res.pagination.page, pages: res.pagination.pages, total: res.pagination.total }
      } else {
        auxPagination.value = null
      }
    } else {
      auxiliaryLabels.value = []
      auxPagination.value = null
    }
    auxLoading.value = false
  }

  async function loadClusters() {
    const res = await auxApi.getClusters()
    if (res.success && res.data) {
      auxClusters.value = res.data.clusters
      auxUnclusteredCount.value = res.data.unclustered_count
    }
  }

  function handleUpdatePage(page: number) {
    auxPage.value = page
    void loadAuxiliaryLabels()
  }

  function handleDisableAuxLabel(id: number) {
    auxApi.disableLabel(id).then(() => void loadAuxiliaryLabels())
  }

  function handleMergeAuxLabel(sourceId: number, targetId: number) {
    auxApi.mergeAlias(sourceId, targetId).then(() => void loadAuxiliaryLabels())
  }

  watch(auxSearchQuery, () => { auxPage.value = 1; void loadAuxiliaryLabels() })
  watch(auxStatusFilter, () => { auxPage.value = 1; void loadAuxiliaryLabels() })

  return {
    auxiliaryLabels, auxClusters, auxUnclusteredCount,
    auxLoading, auxSearchQuery, auxStatusFilter, auxPage, auxPagination, auxPerPage,
    loadAuxiliaryLabels, loadClusters, handleUpdatePage,
    handleDisableAuxLabel, handleMergeAuxLabel,
  }
}
