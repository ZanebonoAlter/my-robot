import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import WatchManagePanel from './WatchManagePanel.vue'
import type { TopicWatch } from '~/api/topicWatches'

// —— api mock：面板自包含拉取/写操作，全 mock 不依赖后端 ——
const apiMocks = vi.hoisted(() => ({
  listWatches: vi.fn(),
  createWatch: vi.fn(),
  updateWatch: vi.fn(),
  deleteWatch: vi.fn(),
  getWatchHits: vi.fn(),
}))

vi.mock('~/api/topicWatches', () => ({
  useTopicWatchesApi: () => ({
    listWatches: apiMocks.listWatches,
    createWatch: apiMocks.createWatch,
    updateWatch: apiMocks.updateWatch,
    deleteWatch: apiMocks.deleteWatch,
    getWatchHits: apiMocks.getWatchHits,
  }),
}))

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" aria-hidden="true" />' },
}))

// —— fixtures ——
function watch(over: Partial<TopicWatch> = {}): TopicWatch {
  return {
    id: '1',
    semanticBoardId: '1974',
    label: '美伊会不会真打起来',
    type: 'label',
    status: 'active',
    createdAt: '2026-06-29T00:00:00Z',
    updatedAt: '2026-06-29T00:00:00Z',
    ...over,
  }
}

function seed(watches: TopicWatch[]) {
  apiMocks.listWatches.mockResolvedValue({ success: true, data: watches })
}

// AppDialog 整体 Teleport 到 body：内容/交互全部走 document 查询（与
// 关注管理对话框沿用 AppDialog 的交互模式。
async function mountPanel(modelValue = true) {
  const wrapper = mount(WatchManagePanel, {
    props: { modelValue, boardId: 1974 },
  })
  await flushPromises() // onMounted refresh
  await nextTick()
  return wrapper
}

function q(selector: string): HTMLElement | null {
  return document.querySelector(selector)
}
function qa(selector: string): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(selector))
}
async function click(selector: string) {
  const el = q(selector)
  expect(el, `element not found: ${selector}`).toBeTruthy()
  el!.click()
  await nextTick()
}

let confirmSpy: ReturnType<typeof vi.fn>

beforeEach(() => {
  apiMocks.listWatches.mockReset()
  apiMocks.createWatch.mockReset()
  apiMocks.updateWatch.mockReset()
  apiMocks.deleteWatch.mockReset()
  apiMocks.getWatchHits.mockReset()
  document.body.innerHTML = ''
  confirmSpy = vi.fn()
  const g = globalThis as unknown as Record<string, unknown>
  g.confirm = confirmSpy
})

// ============================================================
// 列表渲染（spec：入口常驻 + 管理面板列表）
// ============================================================
describe('WatchManagePanel — 列表渲染', () => {
  it('列出该版块全部关注：label/keyword 类型徽标 + active/paused 状态', async () => {
    seed([
      watch({ id: '1', label: '美伊会不会真打起来', type: 'label', status: 'active' }),
      watch({ id: '2', label: 'ASML|镓锗 出口', type: 'keyword', status: 'active' }),
      watch({ id: '3', label: 'AI 监管立法进展', type: 'label', status: 'paused' }),
    ])

    const wrapper = await mountPanel()
    const rows = qa('[data-testid="watch-row"]')
    expect(rows).toHaveLength(3)
    expect(rows[0]!.textContent).toContain('话题')
    expect(rows[0]!.textContent).toContain('美伊会不会真打起来')
    expect(rows[1]!.textContent).toContain('关键字')
    expect(rows[1]!.textContent).toContain('ASML|镓锗 出口')
    // 类型徽标区分 class
    expect(rows[1]!.querySelector('.wmp-row__type')!.className).toContain('wmp-row__type--kw')
    expect(rows[0]!.querySelector('.wmp-row__type')!.className).toContain('wmp-row__type--label')
    // paused 行显「已暂停」+ is-paused class
    expect(rows[2]!.textContent).toContain('已暂停')
    expect(rows[2]!.className).toContain('is-paused')
    // 总数副标题（active + paused）
    expect(q('.app-dialog')!.textContent).toContain('3 个关注')

    wrapper.unmount()
  })

  it('空态：引导文案可见', async () => {
    seed([])
    const wrapper = await mountPanel()
    expect(qa('[data-testid="watch-row"]')).toHaveLength(0)
    const empty = q('[data-testid="watch-empty"]')
    expect(empty).not.toBeNull()
    expect(empty!.textContent).toContain('还没有关注')
    expect(empty!.textContent).toContain('新建关注')

    wrapper.unmount()
  })

  it('挂载即 emit changed（宿主刷新入口计数）', async () => {
    seed([watch({ id: '1' })])
    const wrapper = await mountPanel()
    expect(wrapper.emitted('changed')).toBeTruthy()

    wrapper.unmount()
  })
})

// ============================================================
// 暂停 / 恢复 / 删除（spec：管理面板内暂停或删除关注）
// ============================================================
describe('WatchManagePanel — 管理', () => {
  it('暂停：active 行点暂停 → updateWatch(id, {status:"paused"}) + 刷新', async () => {
    seed([watch({ id: '5', status: 'active' })])
    apiMocks.updateWatch.mockResolvedValue({
      success: true,
      data: watch({ id: '5', status: 'paused' }),
    })

    const wrapper = await mountPanel()
    await click('[data-testid="watch-toggle-status-5"]')
    await flushPromises()

    expect(apiMocks.updateWatch).toHaveBeenCalledWith('5', { status: 'paused' })
    expect(apiMocks.listWatches).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })

  it('恢复：paused 行点恢复 → updateWatch(id, {status:"active"})', async () => {
    seed([watch({ id: '7', status: 'paused' })])
    apiMocks.updateWatch.mockResolvedValue({
      success: true,
      data: watch({ id: '7', status: 'active' }),
    })

    const wrapper = await mountPanel()
    await click('[data-testid="watch-toggle-status-7"]')
    await flushPromises()

    expect(apiMocks.updateWatch).toHaveBeenCalledWith('7', { status: 'active' })

    wrapper.unmount()
  })

  it('删除：AppDialog 二次确认（零原生确认框），确认后 deleteWatch + 刷新', async () => {
    seed([watch({ id: '5', label: '美伊会不会真打起来' })])
    apiMocks.deleteWatch.mockResolvedValue({ success: true, message: 'deleted' })

    const wrapper = await mountPanel()
    await click('[data-testid="watch-delete-5"]')

    // 硬约束：不走原生 confirm
    expect(confirmSpy).not.toHaveBeenCalled()
    // 确认对话框展示 label 与不可撤销提示
    const confirmText = q('.wmp-confirm')
    expect(confirmText).not.toBeNull()
    expect(confirmText!.textContent).toContain('美伊会不会真打起来')
    expect(confirmText!.textContent).toContain('不可撤销')

    await click('[data-testid="watch-delete-confirm"]')
    await flushPromises()

    expect(apiMocks.deleteWatch).toHaveBeenCalledWith('5', false)
    expect(apiMocks.listWatches).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })

  it('删除可在确认前取消（不发起 DELETE）', async () => {
    seed([watch({ id: '5' })])
    const wrapper = await mountPanel()
    await click('[data-testid="watch-delete-5"]')
    await click('[data-testid="watch-delete-cancel"]')

    expect(apiMocks.deleteWatch).not.toHaveBeenCalled()
    expect(q('.wmp-confirm')).toBeNull()

    wrapper.unmount()
  })
})

// ============================================================
// 新建（旧提示轨创建入口退役：仅物化轨双选）
// ============================================================
describe('WatchManagePanel — 新建', () => {
  it('「新建关注」打开物化轨双选对话框（label/keyword 旧类型卡不渲染）', async () => {
    seed([watch({ id: '1' })])
    const wrapper = await mountPanel()

    await click('[data-testid="watch-manage-create"]')

    // 新建对话框 teleport 到 body（与面板同级）：仅两张物化轨卡
    expect(q('[data-testid="watch-type-keyword_topic"]')).not.toBeNull()
    expect(q('[data-testid="watch-type-sentence_topic"]')).not.toBeNull()
    expect(q('[data-testid="watch-type-label"]')).toBeNull()
    expect(q('[data-testid="watch-type-keyword"]')).toBeNull()

    wrapper.unmount()
  })

  it('keyword_topic 建成功：列表刷新，无回扫 banner（提示轨反馈已随入口退役）', async () => {
    seed([])
    apiMocks.createWatch.mockResolvedValue({
      success: true,
      data: {
        id: '9',
        semanticBoardId: '1974',
        label: 'ASML|镓锗 出口',
        type: 'keyword_topic',
        status: 'active',
        createdAt: 'a',
        updatedAt: 'b',
      },
    })

    const wrapper = await mountPanel()
    await click('[data-testid="watch-manage-create"]')

    // 默认 keyword_topic 态输入有效表达式并提交
    const input = q('[data-testid="watch-keyword-input"] input') as HTMLInputElement
    input.value = 'ASML|镓锗 出口'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    await click('[data-testid="watch-create-submit"]')
    await flushPromises()

    expect(apiMocks.createWatch).toHaveBeenCalledWith(1974, 'ASML|镓锗 出口', 'keyword_topic')
    expect(q('[data-testid="watch-scan-banner"]')).toBeNull()
    // 列表已刷新
    expect(apiMocks.listWatches).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })
})
