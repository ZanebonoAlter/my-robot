/**
 * AI 健康「重新检测」交互：触发后端异步重探（POST /api/ai/health/reprobe），
 * 然后轮询 GET /api/ai/health 直至快照 checked_at 前移（探测完成）或超时。
 * 探测在后台跑（可能数十秒），轮询让按钮能反馈最新结果，而不是盲等。
 */
import { ref } from 'vue'
import { useAIAdminApi } from '~/api'
import type { AIHealthSnapshot } from '~/types'

/** 轮询等待探测完成的最长时间 */
const REPROBE_POLL_TIMEOUT_MS = 30_000
/** 轮询间隔 */
const REPROBE_POLL_INTERVAL_MS = 2_000

export function useHealthReprobe() {
  const reprobing = ref(false)

  /**
   * 触发一次重探并等待快照刷新。返回最新快照（超时未刷新时返回 null）。
   * 已有探测在跑时后端返回 skipped=true——同样会等到其完成后的快照。
   */
  async function reprobeHealth(): Promise<AIHealthSnapshot | null> {
    if (reprobing.value) return null
    reprobing.value = true
    try {
      const { reprobeHealth: reprobe, getHealth } = useAIAdminApi()
      const res = await reprobe()
      if (!res.success) throw new Error(res.error || '触发重新检测失败')

      const before = (await getHealth())?.data?.checked_at ?? null
      const deadline = Date.now() + REPROBE_POLL_TIMEOUT_MS
      let snapshot: AIHealthSnapshot | null = null
      while (Date.now() < deadline) {
        await new Promise(resolve => setTimeout(resolve, REPROBE_POLL_INTERVAL_MS))
        const health = await getHealth()
        if (!health.success || !health.data) continue
        snapshot = health.data
        // checked_at 前移（或从 null 变为非 null）= 本次/进行中的探测已落盘
        if (snapshot.checked_at && snapshot.checked_at !== before) break
      }
      return snapshot
    } finally {
      reprobing.value = false
    }
  }

  return { reprobing, reprobeHealth }
}
