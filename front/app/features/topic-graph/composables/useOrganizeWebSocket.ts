/**
 * 标签整理 WebSocket 连接管理器 — 基于 useEventStream
 *
 * 现有消费者（TagHierarchy.vue）的 API 完全兼容：
 *   status, totalUnclassified, processed, currentGroup, category
 *   connect(), disconnect(), reset()
 */

import { useEventStream } from '~/composables/useEventStream'
import { EVENT_TYPES } from '~/utils/eventTypes'

interface OrganizeGroupInfo {
  new_label: string
  candidate_count: number
  action: string
  similarity?: number
}

interface OrganizeProgressMessage {
  type: 'organize_progress'
  status: 'processing' | 'completed'
  total_unclassified: number
  processed: number
  current_group?: OrganizeGroupInfo
  groups?: OrganizeGroupInfo[]
  category?: string
}

export function useOrganizeWebSocket() {
  const stream = useEventStream()
  const status = ref<'idle' | 'processing' | 'completed'>('idle')
  const totalUnclassified = ref(0)
  const processed = ref(0)
  const currentGroup = ref<OrganizeGroupInfo | null>(null)
  const category = ref<string>('')

  // 订阅 organize_progress
  const unsub = stream.on<OrganizeProgressMessage>(EVENT_TYPES.ORGANIZE_PROGRESS, (msg) => {
    status.value = msg.status
    totalUnclassified.value = msg.total_unclassified
    processed.value = msg.processed
    currentGroup.value = msg.current_group ?? null
    category.value = msg.category ?? ''
  })

  function connect() {
    // useEventStream 自动管理连接，这里仅做兼容
  }

  function disconnect() {
    // useEventStream 自动管理清理，这里仅做兼容
  }

  function reset() {
    status.value = 'idle'
    totalUnclassified.value = 0
    processed.value = 0
    currentGroup.value = null
    category.value = ''
  }

  onUnmounted(() => {
    unsub()
  })

  return {
    status,
    totalUnclassified,
    processed,
    currentGroup,
    category,
    connect,
    disconnect,
    reset,
  }
}
