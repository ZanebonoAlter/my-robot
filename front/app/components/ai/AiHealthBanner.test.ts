import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

// useSchedulerStatus 的 analysisPaused/aiHealthy 是 useState 共享态；
// 测试里用可控的普通对象替换（挂载前赋值即可，组件 computed 渲染时读取当前值）。
const bannerState = vi.hoisted(() => ({
  analysisPaused: { value: false },
  aiHealthy: { value: true },
}))

vi.mock('~/composables/useSchedulerStatus', () => ({
  useSchedulerStatus: () => ({
    analysisPaused: bannerState.analysisPaused,
    aiHealthy: bannerState.aiHealthy,
  }),
}))

import AiHealthBanner from './AiHealthBanner.vue'

function mountBanner() {
  return mount(AiHealthBanner, {
    global: {
      stubs: {
        Icon: true,
        NuxtLink: { template: '<a class="banner-link"><slot /></a>' },
      },
    },
  })
}

describe('AiHealthBanner', () => {
  beforeEach(() => {
    bannerState.analysisPaused.value = false
    bannerState.aiHealthy.value = true
  })

  it('意图运行（analysisPaused=false）且不健康时显示提示与去配置入口', () => {
    bannerState.aiHealthy.value = false
    const wrapper = mountBanner()
    expect(wrapper.find('.ai-health-banner').exists()).toBe(true)
    expect(wrapper.text()).toContain('AI 模型未就绪')
    expect(wrapper.text()).toContain('分析暂停运行')
    expect(wrapper.find('.banner-link').exists()).toBe(true)
  })

  it('用户主动暂停（analysisPaused=true）时不显示', () => {
    bannerState.analysisPaused.value = true
    bannerState.aiHealthy.value = false
    const wrapper = mountBanner()
    expect(wrapper.find('.ai-health-banner').exists()).toBe(false)
  })

  it('健康（aiHealthy=true）时不显示', () => {
    bannerState.analysisPaused.value = false
    bannerState.aiHealthy.value = true
    const wrapper = mountBanner()
    expect(wrapper.find('.ai-health-banner').exists()).toBe(false)
  })
})
