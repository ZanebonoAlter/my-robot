import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

// useGlobalSettings / useApiStore 在组件里是显式 import，mock 其模块即可。
// 返回值给真 ref（模板依赖顶层自动解包），其余方法给 vi.fn()。
vi.mock('~/composables/useGlobalSettings', async () => {
  const { ref } = await import('vue')
  return {
    useGlobalSettings: () => ({
      collapsedCategories: ref({}),
      loading: ref(false),
      error: ref<string | null>(null),
      success: ref<string | null>(null),
      feedsByCategory: ref<Record<string, unknown[]>>({}),
      categories: ref([]),
      refreshOptions: [],
      maxArticlesOptions: [],
      updateFeedSetting: vi.fn(),
      refreshFeed: vi.fn(),
      createCategoryAndAssign: vi.fn(),
      deleteFeed: vi.fn(),
    }),
  }
})

const fetchFeedsMock = vi.fn().mockResolvedValue(undefined)
vi.mock('~/stores/api', () => ({
  useApiStore: () => ({
    fetchFeeds: fetchFeedsMock,
    exportOpml: vi.fn(),
  }),
}))

import SettingsSectionFeeds from './SettingsSectionFeeds.vue'

function mountSection() {
  return mount(SettingsSectionFeeds, {
    global: {
      stubs: {
        Icon: { template: '<i />' },
        FeedMasterList: { template: '<div class="stub-feed-master-list" />' },
        FeedDetailEditor: { template: '<div class="stub-feed-detail-editor" />' },
        AddFeedDialog: { template: '<div class="stub-add-feed-dialog" />' },
        AddCategoryDialog: { template: '<div class="stub-add-category-dialog" />' },
        ImportOpmlDialog: { template: '<div class="stub-import-opml-dialog" />' },
      },
    },
  })
}

describe('SettingsSectionFeeds 订阅源管理工具条', () => {
  beforeEach(() => {
    fetchFeedsMock.mockClear()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('渲染 4 个管理入口：添加订阅源 / 添加分类 / 导入 / 导出', () => {
    const wrapper = mountSection()
    const buttons = wrapper.findAll('.feeds-toolbar__btn')
    expect(buttons).toHaveLength(4)
    const labels = buttons.map(b => b.text())
    expect(labels).toContain('添加订阅源')
    expect(labels).toContain('添加分类')
    expect(labels).toContain('导入')
    expect(labels).toContain('导出')
  })

  it('点击「添加订阅源」打开 AddFeedDialog', async () => {
    const wrapper = mountSection()
    expect(wrapper.find('.stub-add-feed-dialog').exists()).toBe(false)

    const btn = wrapper
      .findAll('.feeds-toolbar__btn')
      .find(b => b.text().includes('添加订阅源'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')

    expect(wrapper.find('.stub-add-feed-dialog').exists()).toBe(true)
  })

  it('点击「添加分类」打开 AddCategoryDialog', async () => {
    const wrapper = mountSection()
    const btn = wrapper
      .findAll('.feeds-toolbar__btn')
      .find(b => b.text().includes('添加分类'))
    await btn!.trigger('click')
    expect(wrapper.find('.stub-add-category-dialog').exists()).toBe(true)
  })

  it('点击「导入」打开 ImportOpmlDialog', async () => {
    const wrapper = mountSection()
    const btn = wrapper
      .findAll('.feeds-toolbar__btn')
      .find(b => b.text().includes('导入'))
    await btn!.trigger('click')
    expect(wrapper.find('.stub-import-opml-dialog').exists()).toBe(true)
  })
})
