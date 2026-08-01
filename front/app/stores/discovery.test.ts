import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { DiscoveryRecommendation } from '~/types/discovery'

const getRecommendationsMock = vi.fn()
const refreshRecommendationsMock = vi.fn()
const acceptRecommendationMock = vi.fn()
const dismissRecommendationMock = vi.fn()
const askMock = vi.fn()
const getCatalogStatusMock = vi.fn()
const syncCatalogMock = vi.fn()

const notifyErrorMock = vi.fn()
const notifySuccessMock = vi.fn()
const notifyWarnMock = vi.fn()

vi.mock('~/api/discovery', () => ({
  useDiscoveryApi: () => ({
    getRecommendations: getRecommendationsMock,
    refreshRecommendations: refreshRecommendationsMock,
    acceptRecommendation: acceptRecommendationMock,
    dismissRecommendation: dismissRecommendationMock,
    ask: askMock,
    getCatalogStatus: getCatalogStatusMock,
    syncCatalog: syncCatalogMock,
  }),
}))

vi.mock('~/composables/useNotify', () => ({
  useNotify: () => ({
    error: notifyErrorMock,
    success: notifySuccessMock,
    warn: notifyWarnMock,
  }),
}))

import { useDiscoveryStore } from './discovery'

function createCard(overrides: Partial<DiscoveryRecommendation> = {}): DiscoveryRecommendation {
  return {
    id: '1',
    routeId: '10',
    boardId: '3',
    boardLabel: 'AI 前沿',
    source: 'manual_refresh',
    score: 0.8,
    llmReason: '理由',
    status: 'pending',
    routeNamespace: 'bilibili',
    routePath: '/user/video/:uid',
    routeName: 'UP 主投稿',
    routeExample: '/bilibili/user/video/2267573',
    usableDirectly: false,
    requiresParameters: true,
    parameters: '{"uid":"用户 id"}',
    paramOptions: {},
    createdAt: '2026-07-25T00:00:00Z',
    ...overrides,
  }
}

describe('useDiscoveryStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads pending recommendations into cards', async () => {
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard()] })
    const store = useDiscoveryStore()
    await store.loadRecommendations()
    expect(store.cards).toHaveLength(1)
    expect(store.cards[0]!.routeName).toBe('UP 主投稿')
    expect(notifyErrorMock).not.toHaveBeenCalled()
  })

  it('notifies on load failure', async () => {
    getRecommendationsMock.mockResolvedValue({ success: false, error: '网络错误' })
    const store = useDiscoveryStore()
    await store.loadRecommendations()
    expect(notifyErrorMock).toHaveBeenCalledWith('网络错误')
  })

  it('refresh reloads cards and toasts inserted count', async () => {
    refreshRecommendationsMock.mockResolvedValue({
      success: true,
      data: { candidates: 5, inserted: 2, skipped: 3, cooldownBlocked: 0 },
    })
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard()] })
    const store = useDiscoveryStore()
    await store.refresh()
    expect(notifySuccessMock).toHaveBeenCalledWith('换了一批新推荐（新增 2 条）')
    expect(getRecommendationsMock).toHaveBeenCalled()
  })

  it('refresh warns when no candidates', async () => {
    refreshRecommendationsMock.mockResolvedValue({
      success: true,
      data: { candidates: 0, inserted: 0, skipped: 0, cooldownBlocked: 0 },
    })
    getRecommendationsMock.mockResolvedValue({ success: true, data: [] })
    const store = useDiscoveryStore()
    await store.refresh()
    expect(notifyWarnMock).toHaveBeenCalled()
  })

  it('ask trims question and skips empty input', async () => {
    const store = useDiscoveryStore()
    await store.ask('   ')
    expect(askMock).not.toHaveBeenCalled()
  })

  it('ask reloads recommendations on success', async () => {
    askMock.mockResolvedValue({ success: true, data: [createCard({ source: 'qa' })] })
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard({ source: 'qa' })] })
    const store = useDiscoveryStore()
    await store.ask('我想看 AI 资讯')
    expect(askMock).toHaveBeenCalledWith('我想看 AI 资讯')
    expect(getRecommendationsMock).toHaveBeenCalled()
    expect(store.cards[0]!.source).toBe('qa')
  })

  it('accept removes card and toasts feed title', async () => {
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard()] })
    acceptRecommendationMock.mockResolvedValue({
      success: true,
      data: { id: '9', title: '新源', url: 'http://x' },
    })
    const store = useDiscoveryStore()
    await store.loadRecommendations()
    const ok = await store.accept('1', { parameters: { uid: '123' } })
    expect(ok).toBe(true)
    expect(acceptRecommendationMock).toHaveBeenCalledWith('1', { parameters: { uid: '123' } })
    expect(store.cards).toHaveLength(0)
    expect(notifySuccessMock).toHaveBeenCalledWith('已订阅「新源」')
  })

  it('accept keeps card and notifies on validation failure', async () => {
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard()] })
    acceptRecommendationMock.mockResolvedValue({ success: false, error: 'feed fetch 验证失败' })
    const store = useDiscoveryStore()
    await store.loadRecommendations()
    const ok = await store.accept('1')
    expect(ok).toBe(false)
    expect(store.cards).toHaveLength(1)
    expect(notifyErrorMock).toHaveBeenCalledWith('feed fetch 验证失败')
  })

  it('dismiss removes card on success', async () => {
    getRecommendationsMock.mockResolvedValue({ success: true, data: [createCard()] })
    dismissRecommendationMock.mockResolvedValue({ success: true })
    const store = useDiscoveryStore()
    await store.loadRecommendations()
    const ok = await store.dismiss('1')
    expect(ok).toBe(true)
    expect(store.cards).toHaveLength(0)
  })

  it('syncCatalog reloads catalog status', async () => {
    syncCatalogMock.mockResolvedValue({
      success: true,
      data: { Inserted: 100, Updated: 0, Gone: 0, Total: 3245, NewToEmbed: 100 },
    })
    getCatalogStatusMock.mockResolvedValue({
      success: true,
      data: { total: 3245, ok: 0, broken: 0, unknown: 3245, gone: 0, embedded: 0 },
    })
    const store = useDiscoveryStore()
    const ok = await store.syncCatalog()
    expect(ok).toBe(true)
    expect(store.catalogStatus?.total).toBe(3245)
  })
})
