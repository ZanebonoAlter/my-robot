/**
 * BoardEnrichmentPanel — 工作台信息架构收口（fix-board-analysis-material tasks 3.1）.
 *
 * Covers board-level-analysis spec scenarios:
 *  - 单一下拉选择泳道：泳道选择收敛于聚焦分析区，顶部旧话题选择条不再存在
 *  - 新闻背景入口保留：折叠 section 形态（非单 tab 栏），展开后周期筛选器可用
 */
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import BoardEnrichmentPanel from './BoardEnrichmentPanel.vue'

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" aria-hidden="true" />' },
}))

const topics = ref<{ id: number; label: string; status: string }[]>([
  { id: 101, label: '伊朗局势', status: 'active' },
  { id: 102, label: '美联储政策', status: 'active' },
])
const selectedTopicId = ref<number | null>(101)
const loadTopics = vi.fn().mockResolvedValue(undefined)
const loadDataSources = vi.fn().mockResolvedValue(undefined)
const loadBoardAnalysisResults = vi.fn().mockResolvedValue(undefined)
const loadAllTopicTables = vi.fn().mockResolvedValue(undefined)

vi.mock('~/features/tags/composables/useBoardEnrichment', () => ({
  useBoardEnrichment: () => ({
    // topic selector
    topics, topicsLoading: ref(false), selectedTopicId, loadTopics,
    // table 1
    contexts: ref([]), contextsLoading: ref(false), regenerating: ref(null),
    saveContext: vi.fn(), regenerateContext: vi.fn(),
    // table 2
    results: ref([]), triggering: ref(false), triggerEnrichment: vi.fn(),
    latestResultId: ref<number | null>(null), latestResultDetail: ref(null), latestResultDetailLoading: ref(false),
    // table 3
    reviews: ref([]),
    // data sources
    dataSources: ref([]), loadDataSources, saveDataSource: vi.fn(), removeDataSource: vi.fn(),
    // stock debates
    debates: ref([]), debateTriggering: ref(false), debateError: ref(''), debateStage: ref(''),
    loadDebates: vi.fn(), triggerDebate: vi.fn(),
    // qa
    qaList: ref([]), qaLoading: ref(false), qaError: ref(''), latestAnswer: ref(null),
    loadQA: vi.fn(), askQuestion: vi.fn(), sedimentAnswer: vi.fn(),
    // board-level analysis
    boardResults: ref([]), boardResultsLoading: ref(false),
    selectedBoardResult: ref(null), selectedBoardResultId: ref<number | null>(null),
    selectBoardResult: vi.fn(), boardAnalysisTriggering: ref(false),
    triggerBoardAnalysis: vi.fn(), loadBoardAnalysisResults, loadAllTopicTables,
    syncBoardAnalysisStatus: vi.fn().mockResolvedValue(undefined),
    // workbench UI（周期筛选器）
    selectedGran: ref('week'), selectedPeriodIdx: ref(0), periodList: ref<string[]>([]),
    currentContext: ref(null), setGran: vi.fn(), shiftPeriod: vi.fn(), selectPeriod: vi.fn(),
  }),
}))

vi.mock('~/composables/useNotify', () => ({
  useNotify: () => ({ success: vi.fn(), error: vi.fn() }),
}))
vi.mock('~/utils/markdown', () => ({
  renderMarkdown: (s: string) => s,
}))

const stubs = {
  CausalAnalysisReport: { name: 'CausalAnalysisReport', template: '<div class="causal-stub" />' },
  BoardAnalysisReport: { name: 'BoardAnalysisReport', props: ['result', 'loading'], template: '<div class="board-report-stub" />' },
  QAPanel: { name: 'QAPanel', template: '<div class="qa-stub" />' },
  DebateSection: { name: 'DebateSection', template: '<div class="debate-stub" />' },
  AppDialog: { name: 'AppDialog', props: ['modelValue', 'title'], template: '<div class="dialog-stub" />' },
  AppButton: { name: 'AppButton', template: '<button class="app-btn-stub"><slot /></button>' },
  AppToggle: { name: 'AppToggle', template: '<div class="toggle-stub" />' },
}

function mountPanel(): VueWrapper {
  return mount(BoardEnrichmentPanel, { props: { boardId: 7701 }, global: { stubs } })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('BoardEnrichmentPanel 工作台收口', () => {
  it('单一下拉选择泳道：泳道下拉仅聚焦分析区一处，旧话题选择条不存在', async () => {
    const wrapper = mountPanel()
    // 聚焦分析区先展开（唯一泳道选择点在折叠区内）
    const focusToggle = wrapper.findAll('.focus-toggle').find(b => b.text().includes('聚焦分析'))
    expect(focusToggle).toBeDefined()
    await focusToggle!.trigger('click')

    // 泳道选择下拉（placeholder=选择泳道…）全页面仅一处
    const laneSelects = wrapper.findAll('select').filter(s => s.text().includes('选择泳道…'))
    expect(laneSelects.length).toBe(1)

    // 旧顶栏（话题选择条）不复存在：无 toolbar 容器、无「选择话题…」placeholder
    expect(wrapper.find('.ew-toolbar').exists()).toBe(false)
    const legacySelects = wrapper.findAll('select').filter(s => s.text().includes('选择话题…'))
    expect(legacySelects.length).toBe(0)
    wrapper.unmount()
  })

  it('新闻背景入口保留：折叠 section 形态（非单 tab 栏），展开后周期筛选器可用', async () => {
    const wrapper = mountPanel()
    // 无 subtabs 导航容器（单 tab 栏已移除）
    expect(wrapper.find('.subtabs').exists()).toBe(false)
    expect(wrapper.find('.subtab').exists()).toBe(false)

    // 新闻背景折叠 toggle 存在，默认收起
    const newsToggle = wrapper.findAll('.focus-toggle').find(b => b.text().includes('新闻背景'))
    expect(newsToggle).toBeDefined()
    expect(wrapper.find('.period-picker').exists()).toBe(false)

    // 展开后周期筛选器（粒度切换）可达
    await newsToggle!.trigger('click')
    expect(wrapper.find('.period-picker').exists()).toBe(true)
    expect(wrapper.find('.gran-select').exists()).toBe(true)
    wrapper.unmount()
  })

  it('刷新入口位于版块分析区头部', () => {
    const wrapper = mountPanel()
    const boardHead = wrapper.find('.board-head')
    expect(boardHead.exists()).toBe(true)
    const refreshBtn = boardHead.find('button[title*="刷新"]')
    expect(refreshBtn.exists()).toBe(true)
    wrapper.unmount()
  })

  it('bootstrap 在挂载时拉取板块数据', () => {
    const wrapper = mountPanel()
    expect(loadTopics).toHaveBeenCalledWith(7701)
    expect(loadBoardAnalysisResults).toHaveBeenCalledWith(7701)
    wrapper.unmount()
  })
})
