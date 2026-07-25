/**
 * 订阅源发现 store — 推荐卡片 + 目录状态的服务端状态与写操作。
 *
 * 写操作在此通知（失败只此一层 toast）；卡片列表仅服务发现页，
 * 与 useApiStore 的 feeds 数据隔离（接受成功后由页面层触发 feeds 重拉）。
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useDiscoveryApi } from '~/api/discovery'
import { useNotify } from '~/composables/useNotify'
import type {
  CatalogStatus,
  DiscoveryRecommendation,
} from '~/types/discovery'

export const useDiscoveryStore = defineStore('discovery', () => {
  const api = useDiscoveryApi()
  const notify = useNotify()

  const cards = ref<DiscoveryRecommendation[]>([])
  const catalogStatus = ref<CatalogStatus | null>(null)
  const loading = ref(false)
  const refreshing = ref(false)
  const asking = ref(false)
  const syncingCatalog = ref(false)
  /** 接受/拒绝进行中的卡片 id（防重复点击） */
  const actingIds = ref<string[]>([])

  async function loadRecommendations() {
    loading.value = true
    const res = await api.getRecommendations('pending')
    loading.value = false
    if (res.success && res.data) {
      cards.value = res.data
    } else {
      notify.error(res.error || '加载推荐失败')
    }
  }

  async function loadCatalogStatus() {
    const res = await api.getCatalogStatus()
    if (res.success && res.data) {
      catalogStatus.value = res.data
    }
  }

  async function refresh() {
    refreshing.value = true
    const res = await api.refreshRecommendations()
    refreshing.value = false
    if (res.success && res.data) {
      const s = res.data
      if (s.inserted > 0) {
        notify.success(`换了一批新推荐（新增 ${s.inserted} 条）`)
      } else if (s.candidates === 0) {
        notify.warn('暂无可推荐的内容：先同步目录，或用问答告诉我你想看什么')
      } else {
        notify.warn('没有新的推荐了，过几天再试试')
      }
      await loadRecommendations()
    } else {
      notify.error(res.error || '刷新推荐失败')
    }
  }

  /** 问答：即时推荐落库后重拉列表；同时写入种子偏好。返回是否成功。 */
  async function ask(question: string): Promise<boolean> {
    const q = question.trim()
    if (!q) return false
    asking.value = true
    const res = await api.ask(q)
    asking.value = false
    if (res.success && res.data) {
      if (res.data.length === 0) {
        notify.warn('没匹配到合适的订阅源，换个说法试试')
      }
      await loadRecommendations()
      return true
    }
    notify.error(res.error || '问答失败，请稍后再试')
    return false
  }

  async function accept(id: string, opts: { categoryId?: string, parameters?: Record<string, string> } = {}): Promise<boolean> {
    if (actingIds.value.includes(id)) return false
    actingIds.value = [...actingIds.value, id]
    const res = await api.acceptRecommendation(id, opts)
    actingIds.value = actingIds.value.filter(x => x !== id)
    if (res.success && res.data) {
      cards.value = cards.value.filter(c => c.id !== id)
      notify.success(`已订阅「${res.data.title || res.data.url}」`)
      return true
    }
    notify.error(res.error || '订阅失败')
    return false
  }

  async function dismiss(id: string): Promise<boolean> {
    if (actingIds.value.includes(id)) return false
    actingIds.value = [...actingIds.value, id]
    const res = await api.dismissRecommendation(id)
    actingIds.value = actingIds.value.filter(x => x !== id)
    if (res.success) {
      cards.value = cards.value.filter(c => c.id !== id)
      return true
    }
    notify.error(res.error || '操作失败')
    return false
  }

  async function syncCatalog(): Promise<boolean> {
    syncingCatalog.value = true
    const res = await api.syncCatalog()
    syncingCatalog.value = false
    if (res.success && res.data) {
      const s = res.data
      if (s.Total > 0) {
        notify.success(`目录同步完成，共 ${s.Total} 条路由（新增 ${s.Inserted} / 更新 ${s.Updated}）`)
      } else {
        notify.warn('目录源暂时不可达，稍后再试')
      }
      await loadCatalogStatus()
      return s.Total > 0
    }
    notify.error(res.error || '目录同步失败')
    return false
  }

  return {
    cards,
    catalogStatus,
    loading,
    refreshing,
    asking,
    syncingCatalog,
    actingIds,
    loadRecommendations,
    loadCatalogStatus,
    refresh,
    ask,
    accept,
    dismiss,
    syncCatalog,
  }
})
