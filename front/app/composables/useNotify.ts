/**
 * 全局通知管道
 * 提供 success/error/warn 方法向全局 toast 队列推送消息
 *
 * 使用 useState 替代模块级 ref 以确保 SSR 兼容性。
 * Toast 状态全局共享，所有 useNotify() 调用返回同一实例。
 */

import { readonly } from 'vue'

export type ToastType = 'success' | 'error' | 'warn'

export interface Toast {
  id: string
  type: ToastType
  message: string
  createdAt: number
}

/**
 * Generate unique ID for each toast
 */
let idCounter = 0
function nextId(): string {
  return `toast-${++idCounter}-${Date.now()}`
}

/**
 * Default TTL per toast type (ms)
 */
const TTL: Record<ToastType, number> = {
  success: 3000,
  error: 5000,
  warn: 4000,
}

export function useNotify() {
  const toasts = useState<Toast[]>('notify:toasts', () => [])

  function push(type: ToastType, message: string, ttl?: number) {
    const toast: Toast = {
      id: nextId(),
      type,
      message,
      createdAt: Date.now(),
    }
    toasts.value = [...toasts.value, toast]

    const duration = ttl ?? TTL[type]
    setTimeout(() => {
      dismiss(toast.id)
    }, duration)
  }

  function dismiss(id: string) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return {
    success(msg: string) { push('success', msg) },
    error(msg: string) { push('error', msg) },
    warn(msg: string) { push('warn', msg) },
    dismiss,
    toasts: readonly(toasts),
  }
}
