/**
 * ComposePanel — 编排态组件测试（切片③ tasks 3.2-3.5, 3.8）.
 *
 * Covers section-lifecycle spec scenarios:
 *  - 工具条渲染（返回 + 徽标 + 名称输入 + 保存/取消）
 *  - 预览泳道实时反映勾选（节点数 = 选中数）
 *  - 候选池多选 + 离群标黄（建议剔除但不自动删）
 *  - 体检三卡（聚类质量 / 撞车检查 / 未来预期淡显）
 *  - 保存流程：调 createManualLane → emit saved（无撞车直存；撞车走 AppDialog）
 */
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ComposePanel from './ComposePanel.vue'
import type { ComposeCandidate } from '~/api/persistentTopics'

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" aria-hidden="true" />' },
}))

const getComposeCandidates = vi.fn()
const createManualLane = vi.fn()
const embedQuery = vi.fn()
vi.mock('~/api/persistentTopics', async () => {
  const actual = await vi.importActual<typeof import('~/api/persistentTopics')>('~/api/persistentTopics')
  return {
    ...actual,
    usePersistentTopicsApi: () => ({ getComposeCandidates, createManualLane, embedQuery }),
  }
})

const getDailyReportDetail = vi.fn()
vi.mock('~/api/dailyReports', async () => {
  const actual = await vi.importActual<typeof import('~/api/dailyReports')>('~/api/dailyReports')
  return {
    ...actual,
    useDailyReportsApi: () => ({ getDailyReportDetail }),
  }
})

const stubs = {
  AppDialog: {
    name: 'AppDialog',
    props: ['modelValue', 'title', 'width'],
    template: `<div v-if="modelValue" class="app-dialog-stub" :data-title="title">
      <div class="app-dialog-body"><slot /></div>
      <div class="app-dialog-footer"><slot name="footer" /></div>
    </div>`,
  },
  AppButton: {
    name: 'AppButton',
    props: ['variant', 'size', 'disabled', 'loading'],
    template: '<button class="app-button-stub" :disabled="disabled || loading" :data-variant="variant"><slot /></button>',
  },
  AppInput: {
    name: 'AppInput',
    props: ['modelValue', 'type', 'placeholder'],
    emits: ['update:modelValue'],
    template: '<input class="app-input-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}

function makeCandidate(id: string, opts: Partial<ComposeCandidate> = {}): ComposeCandidate {
  return {
    id,
    reportId: opts.reportId ?? '10',
    periodDate: opts.periodDate ?? '2026-06-18T00:00:00Z',
    clusterLabel: opts.clusterLabel ?? `节点${id}`,
    embedding: opts.embedding ?? [1, 0, 0],
    persistentTopicId: opts.persistentTopicId,
    persistentTopic: opts.persistentTopic,
  }
}

async function mountPanel(props: Record<string, unknown> = {}) {
  const wrapper = mount(ComposePanel, {
    props: { boardId: 1, days: 14, ...props },
    global: { stubs },
  })
  await nextTick() // flush onMounted → load
  return wrapper
}

describe('ComposePanel — 编排态（切片③）', () => {
  beforeEach(() => {
    getComposeCandidates.mockReset()
    createManualLane.mockReset()
    embedQuery.mockReset()
    getDailyReportDetail.mockReset()
  })

  // ---- 11.x: 已勾选 section 查看线索 ----
  it('shows 查看线索 toggle only on checked candidates and loads threads on click', async () => {
    const cands = [
      makeCandidate('1', { reportId: '10', embedding: [1, 0] }),
      makeCandidate('2', { reportId: '10', embedding: [1, 0] }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    // 未勾选：无查看线索入口
    expect(wrapper.findAll('.cp-cand-threads__toggle').length).toBe(0)

    // 勾选 1 → 出现查看线索入口（仅已勾选）
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    expect(wrapper.findAll('.cp-cand-threads__toggle').length).toBe(1)

    // 点查看线索 → 调 getDailyReportDetail(reportId=10) → 就地显示线索列表
    getDailyReportDetail.mockResolvedValue({ success: true, data: { report: { sections: [{ id: 1, threads: [{ id: 100, title: '线索A', related_article_ids: [1, 2] }] }] } } })
    await wrapper.find('[data-cand-threads="1"]').trigger('click')
    await nextTick()
    expect(getDailyReportDetail).toHaveBeenCalledWith(10)
    expect(wrapper.findAll('.cp-thread-row').length).toBe(1)
    expect(wrapper.find('.cp-thread-row__title').text()).toContain('线索A')
    // 不展开文章正文（仅标题+篇数）
    expect(wrapper.find('.cp-thread-row__count').text()).toContain('2篇')
  })

  it('collapses the threads panel when the candidate is unchecked', async () => {
    const cands = [makeCandidate('1', { reportId: '10', embedding: [1, 0] })]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    getDailyReportDetail.mockResolvedValue({ success: true, data: { report: { sections: [{ id: 1, threads: [{ id: 100, title: 'x', related_article_ids: [] }] }] } } })
    await wrapper.find('[data-cand-threads="1"]').trigger('click')
    await nextTick()
    expect(wrapper.find('.cp-cand-threads__body').exists()).toBe(true)
    // 取消勾选 → 线索区随 v-if 收起
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    expect(wrapper.findAll('.cp-cand-threads').length).toBe(0)
  })

  it('degrades gracefully when getDailyReportDetail fails', async () => {
    const cands = [makeCandidate('1', { reportId: '10', embedding: [1, 0] })]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    getDailyReportDetail.mockResolvedValue({ success: false, error: 'boom' })
    await wrapper.find('[data-cand-threads="1"]').trigger('click')
    await nextTick()
    // 轻量错误提示，不崩溃、不阻断
    expect(wrapper.find('.cp-cand-threads__hint--err').text()).toContain('boom')
  })

  // ---- 3.2: 工具条 ----
  it('renders the compose toolbar (back + badge + name input + save/cancel)', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    const text = wrapper.text()
    expect(text).toContain('返回总览')
    expect(text).toContain('新建 · 待保存')
    expect(text).toContain('保存为新泳道')
    expect(text).toContain('取消')
    expect(wrapper.find('.app-input-stub').exists()).toBe(true)
  })

  // ---- 3.3: 预览泳道实时反映勾选 ----
  it('renders one preview node per selected candidate and re-groups on toggle', async () => {
    const cands = [
      makeCandidate('1', { periodDate: '2026-06-18T00:00:00Z', embedding: [1, 0] }),
      makeCandidate('2', { periodDate: '2026-06-18T00:00:00Z', embedding: [1, 0] }),
      makeCandidate('3', { periodDate: '2026-06-19T00:00:00Z', embedding: [0.9, 0.1] }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    // initially nothing selected → placeholder
    expect(wrapper.findAll('.cp-node').length).toBe(0)

    // select 2 → 2 preview nodes (06-18 stacked, 06-19 not yet)
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    await wrapper.find('[data-cand-id="2"]').trigger('click')
    expect(wrapper.findAll('.cp-node').length).toBe(2)
    // 同天纵向堆叠：06-18 列下有 2 个节点
    const col18 = wrapper.findAll('.cp-col').find(c => c.find('.cp-col__date b').text() === '06-18')
    expect(col18!.findAll('.cp-node').length).toBe(2)

    // deselect 1 → preview re-renders to 1 node
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    expect(wrapper.findAll('.cp-node').length).toBe(1)
  })

  // ---- 3.4: 候选池多选 + 离群标黄（建议剔除但不自动删）----
  it('marks outlier candidates yellow + 建议剔除 but keeps them checked', async () => {
    // anchor from these 3 near-identical; an outlier far away
    const cands = [
      makeCandidate('1', { embedding: [1, 0] }),
      makeCandidate('2', { embedding: [0.99, 0.01] }),
      makeCandidate('3', { embedding: [0.98, 0.02] }),
      makeCandidate('4', { embedding: [-1, 0], clusterLabel: '离群项' }), // ~opposite → far/outlier
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    // select all four (including the outlier)
    for (const id of ['1', '2', '3', '4']) {
      await wrapper.find(`[data-cand-id="${id}"]`).trigger('click')
    }
    const outlier = wrapper.find('[data-cand-id="4"]')
    expect(outlier.classes()).toContain('cp-cand--outlier')
    expect(outlier.text()).toContain('建议剔除')
    // 建议剔除但不自动删：仍保持勾选
    expect(outlier.classes()).toContain('cp-cand--checked')
  })

  // ---- 3.5: 体检三卡 ----
  it('renders three health cards (聚类质量 / 撞车检查 / 未来预期)', async () => {
    const cands = [
      makeCandidate('1', { embedding: [1, 0], persistentTopicId: '7', persistentTopic: { id: '7', label: '中东局势' } }),
      makeCandidate('2', { embedding: [0.99, 0.01], persistentTopicId: '7', persistentTopic: { id: '7', label: '中东局势' } }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    await wrapper.find('[data-cand-id="2"]').trigger('click')
    const text = wrapper.text()
    expect(text).toContain('聚类质量')
    expect(text).toContain('撞车检查')
    expect(text).toContain('未来预期')
    // 撞车提示移出（Scenario「撞车明确提示移出」）
    expect(text).toContain('移出')
    expect(text).toContain('中东局势')
  })

  // ---- 3.8: 保存流程（无撞车直存 → emit saved）----
  it('calls createManualLane and emits saved when no crash', async () => {
    const cands = [makeCandidate('1', { embedding: [1, 0] }), makeCandidate('2', { embedding: [0.99, 0.01] })]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    createManualLane.mockResolvedValue({ success: true, data: { topic: { id: '20', label: '美伊博弈', status: 'active', source: 'manual' }, skipped: [] } })
    const wrapper = await mountPanel()
    await wrapper.find('.app-input-stub').setValue('美伊博弈')
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    const save = wrapper.findAll('.app-button-stub').find(b => b.text().includes('保存为新泳道'))!
    await save.trigger('click')
    await nextTick()
    await nextTick()
    expect(createManualLane).toHaveBeenCalledWith(1, '美伊博弈', ['1'])
    expect(wrapper.emitted('saved')).toBeTruthy()
  })

  // ---- 3.8: 撞车走 AppDialog 确认 ----
  it('opens an AppDialog on save when sections will move out', async () => {
    const cands = [
      makeCandidate('1', { embedding: [1, 0], persistentTopicId: '7', persistentTopic: { id: '7', label: '中东局势' } }),
      makeCandidate('2', { embedding: [0.99, 0.01], persistentTopicId: '7', persistentTopic: { id: '7', label: '中东局势' } }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    createManualLane.mockResolvedValue({ success: true, data: { topic: { id: '20', label: 'X', status: 'active', source: 'manual' }, skipped: [] } })
    const wrapper = await mountPanel()
    await wrapper.find('.app-input-stub').setValue('新泳道')
    await wrapper.find('[data-cand-id="1"]').trigger('click')
    // 保存 → moveOutCount>0 → 弹窗
    const save = wrapper.findAll('.app-button-stub').find(b => b.text().includes('保存为新泳道'))!
    await save.trigger('click')
    await nextTick()
    expect(wrapper.find('.app-dialog-stub[data-title="保存将移出部分 section"]').exists()).toBe(true)
    // 坚持新建 → doSave
    const confirm = wrapper.findAll('.app-button-stub').find(b => b.text().includes('坚持新建'))!
    await confirm.trigger('click')
    await nextTick()
    await nextTick()
    expect(createManualLane).toHaveBeenCalledWith(1, '新泳道', ['1'])
    expect(wrapper.emitted('saved')).toBeTruthy()
  })

  // ---- cancel emits cancel ----
  it('emits cancel on 取消 / 返回总览', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    const cancel = wrapper.findAll('.app-button-stub').find(b => b.text().includes('取消'))!
    await cancel.trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  // ---- 一键剔离群 ----
  it('removes only outlier selections via 一键剔除', async () => {
    const cands = [
      makeCandidate('1', { embedding: [1, 0] }),
      makeCandidate('2', { embedding: [0.99, 0.01] }),
      makeCandidate('3', { embedding: [-1, 0] }), // outlier
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel()
    for (const id of ['1', '2', '3']) await wrapper.find(`[data-cand-id="${id}"]`).trigger('click')
    expect(wrapper.findAll('.cp-node').length).toBe(3)
    const kick = wrapper.findAll('button').find(b => b.text().includes('一键剔除'))!
    await kick.trigger('click')
    // outlier removed, 2 tight kept
    expect(wrapper.findAll('.cp-node').length).toBe(2)
    expect(wrapper.find('[data-cand-id="3"]').classes()).not.toContain('cp-cand--checked')
  })
})

// ---- 切片④：候选池语义搜索 + 渐进排序 ----
describe('ComposePanel — 语义搜索（切片④）', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  // spec「文本搜索冷启动排序」
  it('输入文本后按查询向量重排未选候选', async () => {
    const cands = [
      makeCandidate('1', { embedding: [-1, 0], clusterLabel: '远项' }), // 到[1,0]距离 2
      makeCandidate('2', { embedding: [0, 1], clusterLabel: '中项' }), // 距离 1
      makeCandidate('3', { embedding: [0.5, 0.5], clusterLabel: '近项' }), // 距离 ~0.29
      makeCandidate('4', { embedding: [1, 0], clusterLabel: '贴项' }), // 距离 0
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    embedQuery.mockResolvedValue({ success: true, data: { embedding: [1, 0] } })
    const wrapper = await mountPanel()

    await wrapper.find('.cp-search__input').setValue('半导体')
    await vi.advanceTimersByTimeAsync(450)
    await nextTick()

    expect(embedQuery).toHaveBeenCalledWith(1, '半导体')
    // 未选按到 query 向量 [1,0] 距离升序：贴(4)→近(3)→中(2)→远(1)
    const ids = wrapper.findAll('[data-cand-id]').map(el => el.attributes('data-cand-id'))
    expect(ids).toEqual(['4', '3', '2', '1'])
  })

  // spec「勾选后聚合向量接管排序」+「已选置顶分组」
  it('勾选后排序切到聚合锡点，已选置顶分组', async () => {
    const cands = [
      makeCandidate('1', { embedding: [0, 1], clusterLabel: 'c1' }), // 到[1,0]距离 1
      makeCandidate('2', { embedding: [-1, 0], clusterLabel: 'c2' }), // 距离 2
      makeCandidate('3', { embedding: [0.9, 0.1], clusterLabel: 'c3' }), // 距离 ~0.005
      makeCandidate('4', { embedding: [1, 0], clusterLabel: '种子' }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    // query=[0,1] 但 anchor 优先 → 按 anchor=[1,0] 排
    embedQuery.mockResolvedValue({ success: true, data: { embedding: [0, 1] } })
    const wrapper = await mountPanel()

    await wrapper.find('[data-cand-id="4"]').trigger('click') // 选种子 → anchor=[1,0]
    await nextTick()
    expect(wrapper.text()).toContain('已选 1')

    await wrapper.find('.cp-search__input').setValue('任意')
    await vi.advanceTimersByTimeAsync(450)
    await nextTick()

    // 未选按到 anchor[1,0] 距离升序：c3 → c1 → c2（query=[0,1] 被忽略）
    const unselected = wrapper.findAll('[data-cand-id]')
      .filter(el => !el.classes().includes('cp-cand--checked'))
      .map(el => el.attributes('data-cand-id'))
    expect(unselected).toEqual(['3', '1', '2'])
  })

  // spec「清空回退默认排序」
  it('清空搜索框后回退 periodDate 倒序默认序', async () => {
    const cands = [
      makeCandidate('1', { embedding: [1, 0], periodDate: '2026-06-18T00:00:00Z' }),
      makeCandidate('2', { embedding: [0, 1], periodDate: '2026-06-19T00:00:00Z' }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    embedQuery.mockResolvedValue({ success: true, data: { embedding: [1, 0] } })
    const wrapper = await mountPanel()

    await wrapper.find('.cp-search__input').setValue('x')
    await vi.advanceTimersByTimeAsync(450)
    await nextTick()
    // 搜索后 [1,0]→#1(距离0) 在前
    expect(wrapper.findAll('[data-cand-id]').map(el => el.attributes('data-cand-id'))).toEqual(['1', '2'])

    // 清空 → 回退默认序（06-19 在前）
    await wrapper.find('.cp-search__input').setValue('')
    await vi.advanceTimersByTimeAsync(450)
    await nextTick()
    expect(wrapper.findAll('[data-cand-id]').map(el => el.attributes('data-cand-id'))).toEqual(['2', '1'])
  })

  // spec「搜索失败不阻断」
  it('embedQuery 失败时回退默认序 + 显示提示，不阻断勾选', async () => {
    const cands = [
      makeCandidate('1', { embedding: [1, 0], periodDate: '2026-06-18T00:00:00Z' }),
      makeCandidate('2', { embedding: [0, 1], periodDate: '2026-06-19T00:00:00Z' }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    embedQuery.mockResolvedValue({ success: false, error: '模型故障' })
    const wrapper = await mountPanel()

    await wrapper.find('.cp-search__input').setValue('x')
    await vi.advanceTimersByTimeAsync(450)
    await nextTick()

    expect(wrapper.find('.cp-search-err').text()).toContain('模型故障')
    // 回退默认序（periodDate 倒序）：#2(06-19) → #1(06-18)
    expect(wrapper.findAll('[data-cand-id]').map(el => el.attributes('data-cand-id'))).toEqual(['2', '1'])
  })

  // ---- 9.B: 候选话题引导区（累计命中口径的一键激活/并入）----
  it('guide: lists candidate topics and hides when none', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    // 无候选 → 引导区隐藏
    const w0 = await mountPanel()
    expect(w0.find('.cp-guide').exists()).toBe(false)

    // 有候选 → 渲染 label + 累计命中 N 次 + 含 M 条
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel({
      candidateTopics: [
        { id: 5, label: '美伊博弈', hitCount: 3, consecutiveHits: 1, canActivate: true, sectionCount: 4 },
        { id: 6, label: '油价波动', hitCount: 1, consecutiveHits: 1, canActivate: false, sectionCount: 1 },
      ],
    })
    const guide = wrapper.find('.cp-guide')
    expect(guide.exists()).toBe(true)
    expect(guide.text()).toContain('美伊博弈')
    expect(guide.text()).toContain('累计命中 3 次')
    expect(guide.text()).toContain('含 4 条')
    expect(guide.text()).toContain('油价波动')
  })

  it('guide: canActivate splits 可激活/观察中 groups', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel({
      candidateTopics: [
        { id: 6, label: '油价波动', hitCount: 1, consecutiveHits: 1, canActivate: false, sectionCount: 1 },
        { id: 5, label: '美伊博弈', hitCount: 3, consecutiveHits: 0, canActivate: true, sectionCount: 4 },
      ],
    })
    const lists = wrapper.findAll('.cp-guide__list')
    expect(lists.length).toBe(2)
    // 可激活组（can_activate）置顶，项高亮 ready
    expect(lists[0]!.text()).toContain('可激活')
    expect(lists[0]!.text()).toContain('美伊博弈')
    expect(lists[0]!.findAll('.cp-guide__item')[0]!.classes()).toContain('cp-guide__item--ready')
    // 观察中组
    expect(lists[1]!.text()).toContain('观察中')
    expect(lists[1]!.text()).toContain('油价波动')
    expect(lists[1]!.findAll('.cp-guide__item')[0]!.classes()).not.toContain('cp-guide__item--ready')
  })

  it('guide: 确认启用 emits activate-candidate', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel({
      candidateTopics: [
        { id: 5, label: '美伊博弈', hitCount: 3, consecutiveHits: 1, canActivate: true, sectionCount: 4 },
      ],
    })
    const readyItem = wrapper.findAll('.cp-guide__item')[0]!
    // 第一个按钮是「确认启用」
    await readyItem.findAll('button')[0]!.trigger('click')
    expect(wrapper.emitted('activate-candidate')).toBeTruthy()
    expect(wrapper.emitted('activate-candidate')![0]).toEqual([5])
  })

  it('guide: 并入新泳道 selects matching candidate-pool sections', async () => {
    const cands = [
      makeCandidate('10', { persistentTopicId: '5', persistentTopic: { id: '5', label: '美伊博弈' } }),
      makeCandidate('11', { persistentTopicId: '5', persistentTopic: { id: '5', label: '美伊博弈' } }),
      makeCandidate('12', { persistentTopicId: '9', persistentTopic: { id: '9', label: '其他' } }),
    ]
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: cands, matchThreshold: 0.3 } })
    const wrapper = await mountPanel({
      candidateTopics: [
        { id: 5, label: '美伊博弈', hitCount: 3, consecutiveHits: 1, canActivate: true, sectionCount: 2 },
      ],
    })
    // 第二个按钮是「并入新泳道」
    const readyItem = wrapper.findAll('.cp-guide__item')[0]!
    await readyItem.findAll('button')[1]!.trigger('click')
    await nextTick()
    // 归属 topic 5 的 section 10/11 被选中；topic 9 的 12 不受影响
    expect(wrapper.find('[data-cand-id="10"]').classes()).toContain('cp-cand--checked')
    expect(wrapper.find('[data-cand-id="11"]').classes()).toContain('cp-cand--checked')
    expect(wrapper.find('[data-cand-id="12"]').classes()).not.toContain('cp-cand--checked')
  })

  it('guide: 观察中 group has no 确认启用 (累计未达标)', async () => {
    getComposeCandidates.mockResolvedValue({ success: true, data: { sections: [], matchThreshold: 0.3 } })
    const wrapper = await mountPanel({
      candidateTopics: [
        { id: 7, label: '低频话题', hitCount: 1, consecutiveHits: 0, canActivate: false, sectionCount: 2 },
      ],
    })
    const lists = wrapper.findAll('.cp-guide__list')
    // 只有观察中组（无 can_activate 候选）
    expect(lists.length).toBe(1)
    expect(lists[0]!.text()).toContain('观察中')
    expect(lists[0]!.text()).toContain('累计命中 1 次')
    expect(lists[0]!.text()).not.toContain('连续') // 不再显示连续 N 天
    // 观察中项只有「并入新泳道」一个按钮，无「确认启用」
    const obsItem = lists[0]!.findAll('.cp-guide__item')[0]!
    expect(obsItem.findAll('button').length).toBe(1)
    expect(obsItem.text()).not.toContain('确认启用')
  })
})
