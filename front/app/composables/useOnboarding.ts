import { driver, type Config, type DriveStep, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { computed, nextTick, ref } from 'vue'

/**
 * useOnboarding — 首次使用引导（driver.js 分步教程）。
 *
 * 纯前端、基于 localStorage 标记。当前应用 `ssr: false`（SPA），
 * 所有 `import.meta.client` 守卫均为防御性预留，便于将来切 SSR 与 Vitest 行为可预测。
 *
 * 详见 openspec/changes/user-onboarding/design.md（D2/D3/D4）。
 */

const STORAGE_KEY = 'syntopica_onboarding_complete'
const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)'

/** 模块级单例：driver.js 实例 + 响应式状态，跨调用方共享。 */
let driverInstance: Driver | null = null
const tourActive = ref(false)

/**
 * Client-environment guard. Defaults to the Nuxt `import.meta.client` macro so
 * production stays SSR-safe; exposed as a mutable token purely so unit tests can
 * toggle client/SSR behavior — the macro itself is not runtime-overridable under
 * Vitest (it resolves to `undefined` there). See useOnboarding.test.ts.
 */
export const __onboardingClient = { value: import.meta.client }

/** 防御性客户端守卫：非客户端环境一律短路。 */
function isClient(): boolean {
  return __onboardingClient.value
}

/** prefers-reduced-motion 偏好（仅客户端调用）。 */
function prefersReducedMotion(): boolean {
  if (!isClient() || typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia(REDUCED_MOTION_QUERY).matches
}

/**
 * 构建引导步骤并预检过滤缺失元素。
 *
 * driver.js v1 不会自动跳过 `element` 选择器查不到的步骤（见 design.md D2），
 * 因此这里在 `await nextTick()` 之后、传给 driver() 之前，
 * 用 `document.querySelector` 把查不到的选择器对应的步骤剔除。
 * 无 `element`（欢迎弹窗）的步骤始终保留。
 */
function buildTourSteps(): DriveStep[] {
  const steps: DriveStep[] = [
    {
      popover: {
        title: '欢迎使用 Syntopica',
        description: '一个信息聚合 + AI 标签系统。接下来用几步带你了解核心入口。',
        side: 'over',
      },
    },
    {
      element: '[data-onboarding="sidebar-feeds"]',
      popover: {
        title: '订阅源与分类',
        description: '在这里添加和管理你的 RSS 源与分类，文章会自动抓取。',
      },
    },
    {
      element: '[data-onboarding="nav-topic-graph"]',
      popover: {
        title: '主题图谱',
        description: '可视化标签之间的关系，发现内容之间的联系。',
      },
    },
    {
      element: '[data-onboarding="nav-tags"]',
      popover: {
        title: '标签管理',
        description: '查看和管理 AI 为文章自动生成的标签与语义版块。',
      },
    },
    {
      element: '[data-onboarding="watched-tags"]',
      popover: {
        title: '关注标签',
        description: '关注感兴趣的标签，获取个性化文章推送。',
      },
    },
  ]

  return steps.filter((step) => {
    // 无元素（欢迎弹窗）或非字符串选择器（Element/函数）一律保留
    if (!step.element || typeof step.element !== 'string') return true
    if (!isClient()) return true
    return document.querySelector(step.element) !== null
  })
}

/** 标记引导完成（完成/跳过/点外均写入），幂等。 */
function markComplete(): void {
  if (!isClient()) return
  localStorage.setItem(STORAGE_KEY, 'true')
}

/** 启动引导教程。 */
async function startTour(): Promise<void> {
  // 防御性客户端守卫
  if (!isClient()) return
  // 已有实例在跑，忽略重复启动
  if (driverInstance) return

  // 等待 DOM 稳定后再预检选择器（锚点可能依赖异步渲染）
  await nextTick()

  const steps = buildTourSteps()
  // welcome 步骤无元素，理论上始终 ≥1；防御性兜底
  if (steps.length === 0) {
    markComplete()
    return
  }

  const config: Config = {
    steps,
    animate: !prefersReducedMotion(),
    showProgress: true,
    allowClose: true,
    overlayClickBehavior: 'close',
    progressText: '{{index}} / {{total}}',
    nextBtnText: '下一步',
    prevBtnText: '上一步',
    doneBtnText: '完成',
    onDestroyed: () => {
      // 覆盖所有结束路径：完成最后一步 / 点 Skip / 点遮罩外
      markComplete()
      driverInstance = null
      tourActive.value = false
    },
  }

  tourActive.value = true
  driverInstance = driver(config)
  driverInstance.drive()
}

/** 主动结束并标记完成（可用于程序化跳过）。 */
function dismissTour(): void {
  if (!isClient()) return
  markComplete()
  if (driverInstance) {
    driverInstance.destroy()
    driverInstance = null
  }
  tourActive.value = false
}

/** 清除完成标记并刷新页面，使首次访问逻辑重新触发引导。 */
function resetOnboarding(): void {
  if (!isClient()) return
  localStorage.removeItem(STORAGE_KEY)
  location.reload()
}

export function useOnboarding() {
  return {
    // 惰性读 localStorage：每次访问反映当前状态，便于 UI 与测试
    isFirstRun: computed(() => {
      if (!isClient()) return false
      return localStorage.getItem(STORAGE_KEY) !== 'true'
    }),
    isTourActive: computed(() => tourActive.value),
    startTour,
    dismissTour,
    resetOnboarding,
  }
}
