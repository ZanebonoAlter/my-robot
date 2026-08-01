import { computed, onMounted, ref } from 'vue'
import { usePreferenceProfileApi } from '~/api/preferenceProfile'
import { useNotify } from '~/composables/useNotify'
import type { PreferenceProfileItem } from '~/types/discovery'

export interface ProfileGroup {
  boardLabel: string
  items: PreferenceProfileItem[]
}

export interface TagWeight {
  tag: string
  weight: number
}

/**
 * 兴趣画像（设置 preferences section）：按版块分组的偏好向量画像 + 手动重算。
 * 失败通知只在此一层；无数据时返回空组列表（视图渲染空态引导，不报错）。
 */
export function usePreferenceProfile() {
  const api = usePreferenceProfileApi()
  const notify = useNotify()

  const items = ref<PreferenceProfileItem[]>([])
  const loading = ref(false)
  const recomputing = ref(false)

  /** 按版块分组（同一版块可能有 behavior + seed 两行） */
  const groups = computed<ProfileGroup[]>(() => {
    const map = new Map<string, PreferenceProfileItem[]>()
    for (const item of items.value) {
      const list = map.get(item.boardLabel) ?? []
      list.push(item)
      map.set(item.boardLabel, list)
    }
    return [...map.entries()].map(([boardLabel, groupItems]) => ({
      boardLabel,
      items: groupItems,
    }))
  })

  /** top 标签（按权重降序，最多 8 个） */
  function topTags(item: PreferenceProfileItem, limit = 8): TagWeight[] {
    return Object.entries(item.tagWeights)
      .map(([tag, weight]) => ({ tag, weight }))
      .sort((a, b) => b.weight - a.weight)
      .slice(0, limit)
  }

  /** 组内最大权重（条形图比例基准）；空画像返回 0，视图据此不渲染条形 */
  function maxWeight(tags: TagWeight[]): number {
    return tags.reduce((max, t) => Math.max(max, t.weight), 0)
  }

  async function load() {
    loading.value = true
    const res = await api.getProfile()
    loading.value = false
    if (res.success && res.data) {
      items.value = res.data
    } else {
      notify.error(res.error || '加载兴趣画像失败')
    }
  }

  async function recompute() {
    recomputing.value = true
    const res = await api.recompute()
    recomputing.value = false
    if (res.success && res.data) {
      const s = res.data
      if (s.BoardsComputed > 0) {
        notify.success(`重算完成：${s.BoardsComputed} 个版块、${s.TagsUsed} 个标签`)
      } else {
        notify.warn('还没有足够的阅读行为，多读几篇文章再试')
      }
      await load()
    } else {
      notify.error(res.error || '重算失败')
    }
  }

  onMounted(() => {
    void load()
  })

  return {
    items,
    groups,
    loading,
    recomputing,
    topTags,
    maxWeight,
    load,
    recompute,
  }
}
