/**
 * ComposeSidebar — 就地编排态右侧候选侧边栏测试（inline-compose-lane 切片②）.
 *
 * 纯展示组件，只测 props→DOM/emit 接线（composable 逻辑由 D1 覆盖）：
 *  - 无候选时候选区隐藏
 *  - 正在连续命中组：activatable 置顶高亮、确认启用 disabled 当 !activatable
 *  - 已中断组：标「近期未命中」、不渲染确认启用按钮、视觉弱化
 *  - 点确认启用 emit("activate", topicId)；点采纳 emit("adopt", item)
 *  - 搜索框输入 emit("update:queryText")；searchError 显提示条
 */
import { describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import ComposeSidebar from './ComposeSidebar.vue'
import AppButton from '~/components/ui/AppButton.vue'
import type { BoardTopicListItem } from '~/api/dailyReports'
import type { SidebarCandidateItem } from '~/features/tags/composables/useInlineCompose'

function makeTopic(over: Partial<BoardTopicListItem> = {}): BoardTopicListItem {
  return {
    id: 1,
    semantic_board_id: 1,
    label: '美伊博弈',
    description: '',
    status: 'candidate',
    first_seen_date: '2026-01-01',
    last_seen_date: '2026-06-01',
    hit_count: 3,
    consecutive_hits: 3,
    section_count: 4,
    color: '#c2410c',
    can_activate: true,
    ...over,
  }
}

function makeItem(
  topicOver: Partial<BoardTopicListItem> = {},
  itemOver: Partial<Omit<SidebarCandidateItem, 'topic'>> = {},
): SidebarCandidateItem {
  const topic = makeTopic(topicOver)
  return {
    topic,
    activatable: topic.can_activate,
    brokenStreak: topic.consecutive_hits === 0,
    ...itemOver,
  }
}

function mountSidebar(over: Record<string, unknown> = {}) {
  return mount(ComposeSidebar, {
    props: {
      items: [],
      queryText: '',
      searchError: null,
      searching: false,
      ...over,
    },
  })
}

describe('ComposeSidebar — 无候选', () => {
  it('items 为空时候选区整体隐藏', () => {
    const wrapper = mountSidebar({ items: [] })
    expect(wrapper.find('.cs-candidates').exists()).toBe(false)
  })
})

describe('ComposeSidebar — 正在连续命中组', () => {
  it('activatable 候选置顶高亮渲染', () => {
    const activatable = makeItem({ id: 10, label: '美伊博弈', consecutive_hits: 3, can_activate: true })
    const wrapper = mountSidebar({ items: [activatable] })
    const card = wrapper.find('.cs-card.is-activatable')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('美伊博弈')
    expect(card.text()).toContain('连续 3 天')
    expect(card.text()).toContain('含 4 条 section')
  })

  it('!activatable 候选「确认启用」禁用且提示条件', () => {
    const notYet = makeItem({ id: 11, label: '油价波动', consecutive_hits: 1, can_activate: false })
    const wrapper = mountSidebar({ items: [notYet] })
    const card = wrapper.find('.cs-card')
    const confirmBtn = card.findAllComponents(AppButton).find((b: VueWrapper) => b.text().includes('确认启用'))
    expect(confirmBtn).toBeTruthy()
    expect(confirmBtn!.props('disabled')).toBe(true)
    expect(card.text()).toContain('需先满足连续多天出现条件')
  })

  it('点确认启用 emit("activate", topicId)', async () => {
    const item = makeItem({ id: 20, can_activate: true })
    const wrapper = mountSidebar({ items: [item] })
    const card = wrapper.find('.cs-card')
    const confirmBtn = card.findAllComponents(AppButton).find((b: VueWrapper) => b.text().includes('确认启用'))
    await confirmBtn!.trigger('click')
    expect(wrapper.emitted('activate')).toBeTruthy()
    expect(wrapper.emitted('activate')![0]).toEqual([20])
  })
})

describe('ComposeSidebar — 已中断·近期未命中组', () => {
  it('brokenStreak 候选标「近期未命中」且不渲染确认启用按钮', () => {
    const broken = makeItem({ id: 30, label: '断连续', consecutive_hits: 0, can_activate: false })
    const wrapper = mountSidebar({ items: [broken] })
    const brokenGroup = wrapper.find('.cs-group--broken')
    expect(brokenGroup.exists()).toBe(true)
    expect(brokenGroup.text()).toContain('近期未命中')
    // 不应出现「连续 0 天」，也不应出现「确认启用」按钮
    expect(brokenGroup.text()).not.toContain('连续 0 天')
    const confirmBtn = brokenGroup.findAllComponents(AppButton)
      .find((b: VueWrapper) => b.text().includes('确认启用'))
    expect(confirmBtn).toBeUndefined()
  })
})

describe('ComposeSidebar — 采纳', () => {
  it('点采纳 emit("adopt", item)', async () => {
    const item = makeItem({ id: 40, label: '半导体管制', can_activate: true })
    const wrapper = mountSidebar({ items: [item] })
    const card = wrapper.find('.cs-card')
    const adoptBtn = card.findAllComponents(AppButton).find((b: VueWrapper) => b.text().includes('采纳'))
    await adoptBtn!.trigger('click')
    expect(wrapper.emitted('adopt')).toBeTruthy()
    expect(wrapper.emitted('adopt')![0]).toEqual([item])
  })
})

describe('ComposeSidebar — 搜索框', () => {
  it('输入搜索文本 emit("update:queryText")', async () => {
    const wrapper = mountSidebar({ queryText: '' })
    await wrapper.find('input').setValue('半导体出口管制')
    const evt = wrapper.emitted('update:queryText')
    expect(evt).toBeTruthy()
    expect(evt![0]).toEqual(['半导体出口管制'])
  })

  it('searchError 非空时显示降级提示条', () => {
    const wrapper = mountSidebar({ searchError: '搜索向量生成失败' })
    const hint = wrapper.find('.cs-search__error')
    expect(hint.exists()).toBe(true)
    expect(hint.text()).toContain('搜索向量生成失败')
  })

  it('searching 时显示搜索中态', () => {
    const wrapper = mountSidebar({ searching: true })
    expect(wrapper.find('.cs-search__loading').exists()).toBe(true)
  })
})
