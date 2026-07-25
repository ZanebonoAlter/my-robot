import { computed, onMounted } from 'vue'
import { useDiscoveryStore } from '~/stores/discovery'
import { useApiStore } from '~/stores/api'
import type { DiscoveryRecommendation } from '~/types/discovery'

export interface RecommendationGroup {
  label: string
  cards: DiscoveryRecommendation[]
}

/**
 * 发现页数据编排：加载推荐与目录状态、按版块分组、空态判定。
 * 卡片级交互（接受/拒绝/填参）由 DiscoveryCard 直接走 store action。
 */
export function useDiscovery() {
  const store = useDiscoveryStore()
  const apiStore = useApiStore()

  onMounted(() => {
    void store.loadRecommendations()
    void store.loadCatalogStatus()
    // 卡片订阅的分类下拉（直订 + 填参）需要分类数据
    if (apiStore.categories.length === 0) {
      void apiStore.fetchCategories()
    }
  })

  /** 按版块分组（无版块归「全局推荐」），组内按分数降序。 */
  const groups = computed<RecommendationGroup[]>(() => {
    const map = new Map<string, DiscoveryRecommendation[]>()
    for (const card of store.cards) {
      const label = card.boardLabel || '全局推荐'
      const list = map.get(label) ?? []
      list.push(card)
      map.set(label, list)
    }
    return [...map.entries()].map(([label, cards]) => ({
      label,
      cards: [...cards].sort((a, b) => b.score - a.score),
    }))
  })

  /** 目录未同步（一条路由都没有）→ 空态引导同步 */
  const catalogEmpty = computed(
    () => store.catalogStatus !== null && store.catalogStatus.total === 0,
  )

  return {
    store,
    groups,
    catalogEmpty,
  }
}
