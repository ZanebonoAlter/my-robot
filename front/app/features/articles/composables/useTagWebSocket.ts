/**
 * 标签 WebSocket 连接管理器 — 基于 useEventStream
 *
 * 现有消费者（ArticleContentView）的 API 完全兼容：
 *   onResult(handler), onError(handler), watchArticle(id), clearWatch()
 */

import { useEventStream } from '~/composables/useEventStream'
import { EVENT_TYPES } from '~/utils/eventTypes'
import type { ArticleTag } from '~/types/article'

interface TagCompletedItem {
  slug: string
  label: string
  category: string
  score: number
  icon: string
}

interface TagCompletedMessage {
  type: 'tag_completed'
  article_id: number
  job_id: number
  tags: TagCompletedItem[]
}

interface TagFailedMessage {
  type: 'tag_failed'
  article_id: number
  job_id: number
  error: string
}

type TagResultHandler = (articleId: number, tags: ArticleTag[], jobId: number) => void
type TagErrorHandler = (articleId: number, error: string, jobId: number) => void

export function useTagWebSocket() {
  const stream = useEventStream()
  const connected = ref(false)
  const pendingArticleId = ref<number | null>(null)

  const resultHandlers = ref<TagResultHandler[]>([])
  const errorHandlers = ref<TagErrorHandler[]>([])

  // 订阅 tag_completed
  const unsubResult = stream.on<TagCompletedMessage>(EVENT_TYPES.TAG_COMPLETED, (msg) => {
    const tags: ArticleTag[] = msg.tags.map(t => ({
      slug: t.slug,
      label: t.label,
      category: t.category,
      score: t.score,
      icon: t.icon,
    }))
    for (const handler of resultHandlers.value) {
      handler(msg.article_id, tags, msg.job_id)
    }
    if (pendingArticleId.value === msg.article_id) {
      pendingArticleId.value = null
    }
  })

  // 订阅 tag_failed
  const unsubError = stream.on<TagFailedMessage>(EVENT_TYPES.TAG_FAILED, (msg) => {
    for (const handler of errorHandlers.value) {
      handler(msg.article_id, msg.error, msg.job_id)
    }
    if (pendingArticleId.value === msg.article_id) {
      pendingArticleId.value = null
    }
  })

  // 连接状态
  watchEffect(() => {
    connected.value = stream.connected
  })

  function onResult(handler: TagResultHandler) {
    resultHandlers.value.push(handler)
  }

  function onError(handler: TagErrorHandler) {
    errorHandlers.value.push(handler)
  }

  function watchArticle(articleId: number) {
    pendingArticleId.value = articleId
  }

  function clearWatch() {
    pendingArticleId.value = null
  }

  // 清理
  onUnmounted(() => {
    unsubResult()
    unsubError()
  })

  return {
    connected,
    pendingArticleId,
    onResult,
    onError,
    watchArticle,
    clearWatch,
  }
}
