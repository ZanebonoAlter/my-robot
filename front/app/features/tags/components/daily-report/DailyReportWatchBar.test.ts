import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import DailyReportWatchBar from './DailyReportWatchBar.vue'
import type { TopicWatch, TopicWatchHit } from '~/api/topicWatches'

// —— api mock：组件自包含拉取，这里固定返回数据 ----
const apiMocks = vi.hoisted(() => ({
  listWatches: vi.fn(),
  getWatchHits: vi.fn(),
  createWatch: vi.fn(),
  updateWatch: vi.fn(),
  deleteWatch: vi.fn(),
}))

vi.mock('~/api/topicWatches', () => ({
  useTopicWatchesApi: () => ({
    listWatches: apiMocks.listWatches,
    getWatchHits: apiMocks.getWatchHits,
    createWatch: apiMocks.createWatch,
    updateWatch: apiMocks.updateWatch,
    deleteWatch: apiMocks.deleteWatch,
  }),
}))

// Icon stub — 保持测试离线（不拉 iconify CDN）。
vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" aria-hidden="true" />' },
}))

// —— fixtures ——
function watch(over: Partial<TopicWatch> = {}): TopicWatch {
  return {
    id: '1',
    semanticBoardId: '1974',
    label: '美伊会不会真打起来',
    status: 'active',
    createdAt: '2026-06-29T00:00:00Z',
    updatedAt: '2026-06-29T00:00:00Z',
    ...over,
  }
}

function hit(over: Partial<TopicWatchHit> = {}): TopicWatchHit {
  return {
    id: '10',
    watchId: '1',
    sectionId: '123',
    reportId: '9',
    periodDate: '2026-06-29',
    reason: '事态升级的直接信号',
    ...over,
  }
}

const SECTIONS = [
  { id: 123, cluster_label: '霍尔木兹海峡油轮遇袭，油价单日跳涨 4%' },
  { id: 130, cluster_label: 'IAEA 报告：伊朗浓缩铀纯度突破 90%' },
  { id: 200, cluster_label: '欧盟 AI Act 执行细则公布' },
  { id: 201, cluster_label: '荷兰扩大 ASML 对华 DUV 设备出口限制' },
  { id: 202, cluster_label: '中国对镓、锗出口实施新一轮反制许可' },
]

function seed({ watches, hits }: { watches: TopicWatch[], hits: TopicWatchHit[] }) {
  apiMocks.listWatches.mockResolvedValue({ success: true, data: watches })
  apiMocks.getWatchHits.mockResolvedValue({ success: true, data: hits })
}

async function mountBar() {
  const wrapper = mount(DailyReportWatchBar, {
    props: { boardId: 1974, reportId: 9, sections: SECTIONS },
  })
  await flushPromises() // 等 onMounted refresh 完成
  return wrapper
}

let confirmSpy: ReturnType<typeof vi.fn>
let alertSpy: ReturnType<typeof vi.fn>
let promptSpy: ReturnType<typeof vi.fn>

beforeEach(() => {
  apiMocks.listWatches.mockReset()
  apiMocks.getWatchHits.mockReset()
  apiMocks.createWatch.mockReset()
  apiMocks.updateWatch.mockReset()
  apiMocks.deleteWatch.mockReset()
  confirmSpy = vi.fn()
  alertSpy = vi.fn()
  promptSpy = vi.fn()
  // 安装原生弹窗 spy（硬约束：全程零调用）。happy-dom 下 window === globalThis。
  const g = globalThis as unknown as Record<string, unknown>
  g.confirm = confirmSpy
  g.alert = alertSpy
  g.prompt = promptSpy
})

// ============================================================
// A-FE-T2 —— 顶部关注栏展示（spec：关注标记日报顶部独立栏位）
// ============================================================
describe('DailyReportWatchBar — display (spec: 顶部独立栏位)', () => {
  it('命中分组展示：按关注分组列出命中 section 标题 + AI 理由', async () => {
    seed({
      watches: [
        watch({ id: '1', label: '美伊会不会真打起来' }),
        watch({ id: '2', label: 'AI 监管立法进展' }),
      ],
      hits: [
        hit({ id: 'a', watchId: '1', sectionId: '123', reason: '海峡通航安全首次受实质威胁' }),
        hit({ id: 'b', watchId: '1', sectionId: '130', reason: '距武器级仅一步之遥' }),
        hit({ id: 'c', watchId: '2', sectionId: '200', reason: '从立法进入执行阶段' }),
      ],
    })

    const wrapper = await mountBar()
    const groups = wrapper.findAll('[data-testid="watch-group"]')
    expect(groups).toHaveLength(2)

    // 组顺序遵循 watches 顺序（后端创建序）
    expect(groups[0]!.text()).toContain('美伊会不会真打起来')
    expect(groups[0]!.text()).toContain('命中 2')
    expect(groups[1]!.text()).toContain('AI 监管立法进展')

    // 首组默认展开：标题 + 理由可见
    const firstHits = groups[0]!.findAll('[data-testid="watch-hit"]')
    expect(firstHits).toHaveLength(2)
    expect(firstHits[0]!.text()).toContain('霍尔木兹海峡油轮遇袭')
    expect(firstHits[0]!.text()).toContain('海峡通航安全首次受实质威胁')
    expect(firstHits[1]!.text()).toContain('IAEA 报告：伊朗浓缩铀纯度突破 90%')
  })

  it('无命中空态：显示提示且不渲染空分组', async () => {
    seed({
      watches: [watch({ id: '1', label: '美伊会不会真打起来' })],
      hits: [],
    })

    const wrapper = await mountBar()
    expect(wrapper.findAll('[data-testid="watch-group"]')).toHaveLength(0)
    const empty = wrapper.find('[data-testid="watch-empty"]')
    expect(empty.exists()).toBe(true)
    expect(empty.text()).toContain('今天没有命中你关注的动态')
    // 空态仍提示「N 个关注仍在监控中」
    expect(empty.text()).toContain('1 个关注仍在监控中')
  })

  it('折叠：单组命中超过阈值时折叠为「还有 N 条」，点击展开', async () => {
    seed({
      watches: [watch({ id: '1', label: '半导体出口管制博弈' })],
      hits: [
        hit({ id: 'h1', watchId: '1', sectionId: '201' }),
        hit({ id: 'h2', watchId: '1', sectionId: '202' }),
        hit({ id: 'h3', watchId: '1', sectionId: '123' }),
      ],
    })

    const wrapper = await mountBar()
    const group = wrapper.find('[data-testid="watch-group"]')
    // 首组默认展开
    expect(group.classes()).toContain('is-open')

    // 阈值=2：只渲染前 2 条 + 「还有 1 条命中」
    let hitRows = group.findAll('[data-testid="watch-hit"]')
    expect(hitRows).toHaveLength(2)
    const more = group.find('[data-testid="watch-hit-more"]')
    expect(more.exists()).toBe(true)
    expect(more.text()).toContain('还有 1 条命中')

    // 展开剩余
    await more.trigger('click')
    hitRows = group.findAll('[data-testid="watch-hit"]')
    expect(hitRows).toHaveLength(3)
    expect(group.find('[data-testid="watch-hit-more"]').exists()).toBe(false)
  })

  it('与正文分区语义区分：eyebrow「你在追踪 · Watchlist」+ accent 竖条标识存在', async () => {
    seed({
      watches: [watch({ id: '1', label: '美伊' })],
      hits: [hit({ watchId: '1', sectionId: '123' })],
    })

    const wrapper = await mountBar()
    // 独立 eyebrow 文案（区别于正文「话题·今日动态」）
    const eye = wrapper.find('.dwb-watch__eye')
    expect(eye.exists()).toBe(true)
    expect(eye.text()).toContain('你在追踪')
    expect(eye.text()).toContain('Watchlist')
    // accent 竖条 = 分组卡左 3px accent 边（DOM 上由 .dwb-group 承载）
    const group = wrapper.find('[data-testid="watch-group"]')
    expect(group.exists()).toBe(true)
    expect(group.classes()).toContain('dwb-group')
    // 栏位自身带 data-watchlist 语义标识
    expect(wrapper.find('[data-watchlist]').exists()).toBe(true)
  })

  it('暂停的关注不渲染命中分组（paused 不参与判定），但以灰显行列入管理面', async () => {
    seed({
      watches: [
        watch({ id: '1', label: '美伊会不会真打起来', status: 'active' }),
        watch({ id: '2', label: '已暂停的关注', status: 'paused' }),
      ],
      // 即便后端误返回 paused watch 的命中，分组也只取 active watches
      hits: [hit({ id: 'a', watchId: '1', sectionId: '123' })],
    })

    const wrapper = await mountBar()
    const groups = wrapper.findAll('[data-testid="watch-group"]')
    expect(groups).toHaveLength(1)
    expect(groups[0]!.text()).toContain('美伊会不会真打起来')

    // paused 关注以灰显行出现
    const pausedRow = wrapper.find('[data-testid="watch-paused"]')
    expect(pausedRow.exists()).toBe(true)
    expect(pausedRow.text()).toContain('已暂停的关注')
    expect(pausedRow.text()).toContain('已暂停')
    expect(pausedRow.classes()).toContain('dwb-paused-row')
  })

  it('无任何关注时不渲染栏位（避免空栏占据显著空白）', async () => {
    seed({ watches: [], hits: [] })
    const wrapper = await mountBar()
    expect(wrapper.find('[data-testid="watch-bar"]').exists()).toBe(false)
  })
})

// ============================================================
// A-FE-T3 —— 关注管理（新建/暂停/恢复/删除），零 window.*
// ============================================================
describe('DailyReportWatchBar — management (spec: 管理 API + 零 window.*)', () => {
  it('新建关注：打开 AppDialog → 输入 label → 创建调用 createWatch(boardId, label)', async () => {
    seed({
      watches: [watch({ id: '1', label: '已有' })],
      hits: [hit({ watchId: '1', sectionId: '123' })],
    })
    apiMocks.createWatch.mockResolvedValue({
      success: true,
      data: watch({ id: '9', label: '半导体出口管制博弈', status: 'active' }),
    })

    const wrapper = await mountBar()

    // 打开新建对话框
    await wrapper.find('[data-testid="watch-create-open"]').trigger('click')
    await nextTick()
    // AppDialog teleport 到 body；不使用原生 prompt 弹窗
    expect(alertSpy).not.toHaveBeenCalled()
    expect(promptSpy).not.toHaveBeenCalled()
    const dialog = document.querySelector('.app-dialog')
    expect(dialog).not.toBeNull()
    expect(dialog!.textContent).toContain('新建关注')

    // 填入 label
    const input = document.querySelector('.app-dialog input') as HTMLInputElement
    expect(input).toBeTruthy()
    input.value = '半导体出口管制博弈'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()

    // 提交
    const submit = document.querySelector('[data-testid="watch-create-submit"]') as HTMLButtonElement
    expect(submit.disabled).toBe(false)
    submit.click()
    await flushPromises()

    expect(apiMocks.createWatch).toHaveBeenCalledWith(1974, '半导体出口管制博弈')
    // 创建成功后刷新
    expect(apiMocks.listWatches).toHaveBeenCalledTimes(2)
  })

  it('暂停：点击 active 关注的暂停按钮 → updateWatch(id, {status:"paused"})', async () => {
    seed({
      watches: [watch({ id: '5', label: '美伊', status: 'active' })],
      hits: [hit({ watchId: '5', sectionId: '123' })],
    })
    apiMocks.updateWatch.mockResolvedValue({
      success: true,
      data: watch({ id: '5', label: '美伊', status: 'paused' }),
    })

    const wrapper = await mountBar()
    await wrapper.find('[data-testid="watch-toggle-status-5"]').trigger('click')
    await flushPromises()

    expect(apiMocks.updateWatch).toHaveBeenCalledWith('5', { status: 'paused' })
  })

  it('恢复：paused 行的恢复按钮 → updateWatch(id, {status:"active"})', async () => {
    seed({
      watches: [
        watch({ id: '5', label: '美伊', status: 'active' }),
        watch({ id: '7', label: '已暂停项', status: 'paused' }),
      ],
      hits: [hit({ watchId: '5', sectionId: '123' })],
    })
    apiMocks.updateWatch.mockResolvedValue({
      success: true,
      data: watch({ id: '7', label: '已暂停项', status: 'active' }),
    })

    const wrapper = await mountBar()
    // paused 行内的恢复按钮（data-testid 同名）
    await wrapper.find('[data-testid="watch-toggle-status-7"]').trigger('click')
    await flushPromises()

    expect(apiMocks.updateWatch).toHaveBeenCalledWith('7', { status: 'active' })
  })

  it('删除：走 AppDialog 二次确认，绝不调用原生 confirm；确认后 deleteWatch', async () => {
    seed({
      watches: [watch({ id: '5', label: '美伊会不会真打起来', status: 'active' })],
      hits: [hit({ watchId: '5', sectionId: '123' })],
    })
    apiMocks.deleteWatch.mockResolvedValue({ success: true, message: 'deleted' })

    const wrapper = await mountBar()

    // 点删除（不直接删，先弹确认框）
    await wrapper.find('[data-testid="watch-delete-5"]').trigger('click')
    await nextTick()

    // 硬约束：全程不调用原生 confirm/alert/prompt
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(promptSpy).not.toHaveBeenCalled()

    // 弹出 AppDialog 确认框，显示 label 与不可撤销提示
    const confirmText = document.querySelector('.dwb-dialog__confirm')
    expect(confirmText).not.toBeNull()
    expect(confirmText!.textContent).toContain('美伊会不会真打起来')
    expect(confirmText!.textContent).toContain('不可撤销')

    // 确认删除
    const confirmBtn = document.querySelector('[data-testid="watch-delete-confirm"]') as HTMLButtonElement
    confirmBtn.click()
    await flushPromises()

    expect(apiMocks.deleteWatch).toHaveBeenCalledWith('5')
    // 级联清理由后端负责；前端删除成功后刷新列表
    expect(apiMocks.listWatches).toHaveBeenCalledTimes(2)
    // 仍未调用 window.*
    expect(confirmSpy).not.toHaveBeenCalled()
  })

  it('删除可在确认前取消（不发起 DELETE）', async () => {
    seed({
      watches: [watch({ id: '5', label: '美伊', status: 'active' })],
      hits: [hit({ watchId: '5', sectionId: '123' })],
    })
    const wrapper = await mountBar()
    await wrapper.find('[data-testid="watch-delete-5"]').trigger('click')
    await nextTick()

    const cancelBtn = document.querySelector('[data-testid="watch-delete-cancel"]') as HTMLButtonElement
    cancelBtn.click()
    await nextTick()

    expect(apiMocks.deleteWatch).not.toHaveBeenCalled()
    // 对话框关闭
    expect(document.querySelector('.dwb-dialog__confirm')).toBeNull()
  })
})
