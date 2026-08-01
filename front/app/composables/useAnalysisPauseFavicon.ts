/**
 * 分析暂停态 favicon 标识
 * watch 全局暂停态（useSchedulerStatus 的 analysisPaused），经 useHead 切换
 * link[rel=icon]：暂停时换成带橙色 ⏸ 角标的内联 SVG，恢复时回到 /favicon.png。
 *
 * 与 nuxt.config 里的静态 favicon 共用 key 'app-favicon'，unhead 按 key 去重覆盖。
 * useHead 属性值支持响应式 computed，SSR 渲染初始值 /favicon.png，客户端自动更新。
 */

import { computed } from 'vue'
import { useHead } from '#imports'
import { useSchedulerStatus } from '~/composables/useSchedulerStatus'

// 暂停态 favicon：深灰圆角底 + 白色列表条，右下角橙色圆形 ⏸ 角标
const PAUSED_FAVICON =
  'data:image/svg+xml,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">' +
      '<rect width="32" height="32" rx="7" fill="#4b5563"/>' +
      '<rect x="8" y="9" width="16" height="2.6" rx="1.3" fill="#f9fafb"/>' +
      '<rect x="8" y="14.6" width="11" height="2.6" rx="1.3" fill="#f9fafb"/>' +
      '<rect x="8" y="20.2" width="13" height="2.6" rx="1.3" fill="#f9fafb"/>' +
      '<circle cx="23.5" cy="23.5" r="8" fill="#f59e0b" stroke="#ffffff" stroke-width="1.6"/>' +
      '<rect x="20.4" y="19.5" width="2.5" height="8" rx="1.1" fill="#ffffff"/>' +
      '<rect x="24.8" y="19.5" width="2.5" height="8" rx="1.1" fill="#ffffff"/>' +
      '</svg>',
  )

export function useAnalysisPauseFavicon() {
  const { analysisPaused } = useSchedulerStatus()

  const faviconHref = computed(() => (analysisPaused.value ? PAUSED_FAVICON : '/favicon.png'))
  const faviconType = computed(() => (analysisPaused.value ? 'image/svg+xml' : 'image/png'))

  useHead({
    link: [{ key: 'app-favicon', rel: 'icon', type: faviconType, href: faviconHref }],
  })
}
