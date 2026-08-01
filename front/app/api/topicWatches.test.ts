import { beforeEach, describe, expect, it, vi } from 'vitest'

const { postMock, getMock, patchMock, deleteMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  getMock: vi.fn(),
  patchMock: vi.fn(),
  deleteMock: vi.fn(),
}))

vi.mock('./client', () => ({
  apiClient: {
    get: getMock,
    post: postMock,
    patch: patchMock,
    delete: deleteMock,
  },
}))

import { useTopicWatchesApi } from './topicWatches'

describe('useTopicWatchesApi — normalizer & signatures', () => {
  beforeEach(() => {
    postMock.mockReset()
    getMock.mockReset()
    patchMock.mockReset()
    deleteMock.mockReset()
  })

  describe('createWatch', () => {
    it('POSTs to /semantic-boards/:boardId/topic-watches with {label} and normalizes the result', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: {
          id: 42,
          semantic_board_id: 1974,
          label: '美伊会不会真打起来',
          status: 'active',
          created_at: '2026-06-30T01:00:00Z',
          updated_at: '2026-06-30T01:00:00Z',
        },
      })

      const api = useTopicWatchesApi()
      const res = await api.createWatch(1974, '美伊会不会真打起来')

      expect(postMock).toHaveBeenCalledWith('/semantic-boards/1974/topic-watches', { label: '美伊会不会真打起来' })
      expect(res.success).toBe(true)
      // snake_case → camelCase
      expect(res.data).toMatchObject({
        semanticBoardId: '1974',
        label: '美伊会不会真打起来',
        status: 'active',
        createdAt: '2026-06-30T01:00:00Z',
        updatedAt: '2026-06-30T01:00:00Z',
      })
      // numeric id → string at the API boundary
      expect(res.data!.id).toBe('42')
      expect(typeof res.data!.id).toBe('string')
    })

    it('returns success:false without throwing on backend error', async () => {
      postMock.mockResolvedValue({ success: false, error: 'label too long' })
      const api = useTopicWatchesApi()
      const res = await api.createWatch(1974, 'x')
      expect(res.success).toBe(false)
      expect(res.error).toBe('label too long')
    })
  })

  describe('listWatches', () => {
    it('GETs /semantic-boards/:boardId/topic-watches and normalizes the array (incl. paused)', async () => {
      getMock.mockResolvedValue({
        success: true,
        data: [
          { id: 1, semantic_board_id: 1974, label: 'active one', status: 'active', created_at: 'a', updated_at: 'b' },
          { id: 2, semantic_board_id: 1974, label: 'paused one', status: 'paused', created_at: 'c', updated_at: 'd' },
        ],
      })

      const api = useTopicWatchesApi()
      const res = await api.listWatches(1974)

      expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/topic-watches')
      expect(res.success).toBe(true)
      expect(res.data).toHaveLength(2)
      expect(res.data![0]).toMatchObject({ id: '1', status: 'active' })
      expect(res.data![1]).toMatchObject({ id: '2', status: 'paused', semanticBoardId: '1974' })
    })

    it('returns [] when the backend reports failure (never throws)', async () => {
      getMock.mockResolvedValue({ success: false, error: 'boom' })
      const api = useTopicWatchesApi()
      const res = await api.listWatches(1974)
      expect(res.success).toBe(false)
      expect(res.data).toEqual([])
    })
  })

  describe('updateWatch', () => {
    it('PATCHes /topic-watches/:id with {label?, status?} and normalizes', async () => {
      patchMock.mockResolvedValue({
        success: true,
        data: { id: 5, semantic_board_id: 1974, label: 'renamed', status: 'paused', created_at: 'a', updated_at: 'b' },
      })

      const api = useTopicWatchesApi()
      const res = await api.updateWatch('5', { status: 'paused' })

      expect(patchMock).toHaveBeenCalledWith('/topic-watches/5', { status: 'paused' })
      expect(res.success).toBe(true)
      expect(res.data).toMatchObject({ id: '5', status: 'paused', label: 'renamed' })
    })

    it('accepts a label-only update', async () => {
      patchMock.mockResolvedValue({ success: true, data: { id: 5, semantic_board_id: 1974, label: 'new', status: 'active', created_at: 'a', updated_at: 'b' } })
      const api = useTopicWatchesApi()
      await api.updateWatch('5', { label: 'new' })
      expect(patchMock).toHaveBeenCalledWith('/topic-watches/5', { label: 'new' })
    })
  })

  describe('deleteWatch', () => {
    it('DELETEs /topic-watches/:id', async () => {
      deleteMock.mockResolvedValue({ success: true, message: 'deleted' })
      const api = useTopicWatchesApi()
      const res = await api.deleteWatch('5')
      expect(deleteMock).toHaveBeenCalledWith('/topic-watches/5')
      expect(res.success).toBe(true)
    })
  })

  describe('getWatchHits', () => {
    it('GETs /daily-reports/:id/watch-hits and normalizes ids to strings', async () => {
      getMock.mockResolvedValue({
        success: true,
        data: [
          { id: 100, watch_id: 1, section_id: 123, report_id: 9, period_date: '2026-06-29', reason: '事态升级信号' },
          { id: 101, watch_id: 1, section_id: 130, report_id: 9, period_date: '2026-06-29', reason: '武器级前置' },
        ],
      })

      const api = useTopicWatchesApi()
      const res = await api.getWatchHits(9)

      expect(getMock).toHaveBeenCalledWith('/daily-reports/9/watch-hits')
      expect(res.success).toBe(true)
      expect(res.data).toHaveLength(2)
      expect(res.data![0]).toMatchObject({
        id: '100',
        watchId: '1',
        sectionId: '123',
        reportId: '9',
        periodDate: '2026-06-29',
        reason: '事态升级信号',
      })
      expect(typeof res.data![0]!.sectionId).toBe('string')
    })

    it('returns [] on failure (never throws)', async () => {
      getMock.mockResolvedValue({ success: false, error: 'boom' })
      const api = useTopicWatchesApi()
      const res = await api.getWatchHits(9)
      expect(res.success).toBe(false)
      expect(res.data).toEqual([])
    })
  })

  it('exposes the full method surface', () => {
    const api = useTopicWatchesApi()
    expect(typeof api.createWatch).toBe('function')
    expect(typeof api.listWatches).toBe('function')
    expect(typeof api.updateWatch).toBe('function')
    expect(typeof api.deleteWatch).toBe('function')
    expect(typeof api.getWatchHits).toBe('function')
  })
})
