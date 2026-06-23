/**
 * 统一事件流客户端
 * 单例 WebSocket 连接 + 类型化事件订阅 + 自动重连（指数退避）
 *
 * 后端仍使用 WebSocket（/ws），未切换到 SSE。
 * 后续后端提供 SSE 端点后，可将底层切换为 EventSource，外部 API 不变。
 *
 * 生命周期：
 * - stream.on() 注册处理器，递增引用计数；首次调用时建立连接
 * - 返回的取消订阅函数递减引用计数；归零时自动断开并清理
 * - 后续 stream.on() 自动重建连接
 * - 调用 useEventStream() 的组件卸载时，其 composable 应自行调用取消订阅
 */

import { getApiOrigin } from '~/utils/api'

const INITIAL_RECONNECT_DELAY = 1000
const MAX_RECONNECT_DELAY = 30000
const BACKOFF_MULTIPLIER = 1.5

type Handler = (data: unknown) => void

class EventStreamConnection {
  private ws: WebSocket | null = null
  private handlers = new Map<string, Set<Handler>>()
  private reconnectDelay = INITIAL_RECONNECT_DELAY
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private _refCount = 0

  constructor(private readonly onIdle: () => void) {}

  /** 当前是否已连接 */
  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  /** 当前注册的处理器数量 */
  get refCount(): number {
    return this._refCount
  }

  /**
   * 订阅事件，返回取消订阅函数
   */
  on<T = unknown>(type: string, handler: (data: T) => void): () => void {
    this._refCount++
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set())
    }
    this.handlers.get(type)!.add(handler as Handler)

    if (!this.ws) {
      this.connect()
    }

    let active = true
    return () => {
      if (!active) return
      active = false
      this.handlers.get(type)?.delete(handler as Handler)
      this._refCount--
      if (this._refCount <= 0) {
        this.cleanup()
        this.onIdle()
      }
    }
  }

  off(type: string, handler: Handler): void {
    this.handlers.get(type)?.delete(handler)
  }

  private connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) return

    const wsBase = getApiOrigin().replace(/^http/, 'ws')
    const url = `${wsBase}/ws`

    try {
      this.ws = new WebSocket(url)
    } catch {
      this.scheduleReconnect()
      return
    }

    this.ws.onopen = () => {
      this.reconnectDelay = INITIAL_RECONNECT_DELAY
    }

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data)
        const type = msg.type as string
        if (type && this.handlers.has(type)) {
          const handlers = this.handlers.get(type)
          if (handlers) {
            for (const handler of handlers) {
              try {
                handler(msg)
              } catch {
                // 单个 handler 异常不影响其他
              }
            }
          }
        }
      } catch {
        // 忽略非 JSON 消息
      }
    }

    this.ws.onclose = () => {
      this.ws = null
      if (this._refCount > 0) {
        this.scheduleReconnect()
      }
    }

    this.ws.onerror = () => {
      // onclose 会跟随触发，不需要重复处理
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectDelay = Math.min(this.reconnectDelay * BACKOFF_MULTIPLIER, MAX_RECONNECT_DELAY)
      this.connect()
    }, this.reconnectDelay)
  }

  /**
   * 清理连接和处理器，允许后续重新连接
   */
  private cleanup(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.close(1000, 'Cleanup')
      this.ws = null
    }
    this.handlers.clear()
    this._refCount = 0
    this.reconnectDelay = INITIAL_RECONNECT_DELAY
  }
}

/** 模块级单例 */
let globalStream: EventStreamConnection | null = null

/**
 * 使用全局事件流
 *
 * 所有调用共享同一个后端连接。首个订阅者触发连接建立，
 * 最后一个取消订阅时自动断开。
 */
export function useEventStream(): {
  on: <T>(type: string, handler: (data: T) => void) => () => void
  off: (type: string, handler: Handler) => void
  connected: boolean
} {
  if (!globalStream) {
    globalStream = new EventStreamConnection(() => {
      globalStream = null
    })
  }

  return {
    on<T>(type: string, handler: (data: T) => void): () => void {
      return globalStream!.on(type, handler)
    },
    off(type: string, handler: Handler): void {
      globalStream!.off(type, handler)
    },
    get connected(): boolean {
      return globalStream?.connected ?? false
    },
  }
}
