import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

// useSchedulerStatus 的 aiHealthy 是 useState 共享态（ref）；
// 测试里用可控的 computed ref 替换（组件模板依赖顶层 ref 自动解包，mock 必须给真 ref）。
const headerState = vi.hoisted(() => ({
  analysisPaused: false,
  aiHealthy: true,
}))

vi.mock('~/composables/useSchedulerStatus', async () => {
  const { computed } = await import('vue')
  return {
    useSchedulerStatus: () => ({
      analysisPaused: computed(() => headerState.analysisPaused),
      aiHealthy: computed(() => headerState.aiHealthy),
      loadSchedulersStatus: vi.fn(),
      setAnalysisPaused: vi.fn(),
    }),
  }
})

vi.mock('~/composables/useTheme', () => ({
  useTheme: () => ({ toggleTheme: vi.fn(), isDark: { value: false } }),
}))

vi.mock('~/composables/useOnboarding', () => ({
  useOnboarding: () => ({ startTour: vi.fn() }),
}))

vi.mock('~/composables/useAnalysisPauseFavicon', () => ({
  useAnalysisPauseFavicon: vi.fn(),
}))

vi.mock('~/composables/useNotify', () => ({
  useNotify: () => ({ success: vi.fn(), error: vi.fn() }),
}))

import AppHeaderView from './AppHeaderView.vue'

const navigateToMock = vi.fn()

function mountHeader() {
  return mount(AppHeaderView, {
    global: {
      stubs: {
        // class 绑定会透传到 <i> 根节点，便于断言图标状态色 class
        Icon: { template: '<i />' },
      },
    },
  })
}

describe('AppHeaderView AI 健康状态指示', () => {
  beforeEach(() => {
    headerState.analysisPaused = false
    headerState.aiHealthy = true
    navigateToMock.mockClear()
    vi.stubGlobal('navigateTo', navigateToMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('aiHealthy=true 时渲染健康态（绿色图标 + 「AI 模型健康」title）', () => {
    const wrapper = mountHeader()
    const btn = wrapper.find('.ai-health-btn')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('title')).toBe('AI 模型健康')
    expect(btn.find('.ai-health-icon--ok').exists()).toBe(true)
    expect(btn.find('.ai-health-icon--down').exists()).toBe(false)
  })

  it('aiHealthy=false 时渲染未就绪态（琥珀图标 + 「未就绪」title）', () => {
    headerState.aiHealthy = false
    const wrapper = mountHeader()
    const btn = wrapper.find('.ai-health-btn')
    expect(btn.attributes('title')).toBe('AI 模型未就绪（LLM/Embedding 未连通）')
    expect(btn.find('.ai-health-icon--down').exists()).toBe(true)
    expect(btn.find('.ai-health-icon--ok').exists()).toBe(false)
  })

  it('点击指示跳转 /settings?section=ai-health', async () => {
    const wrapper = mountHeader()
    await wrapper.find('.ai-health-btn').trigger('click')
    expect(navigateToMock).toHaveBeenCalledWith('/settings?section=ai-health')
  })
})
