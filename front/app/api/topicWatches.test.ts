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
      // type 缺省：后端列默认 label，响应无 type 字段时归一化为 label（向后兼容）
      expect(res.data!.type).toBe('label')
      expect(res.data!.instantHitCount).toBeUndefined()
    })

    it('passes type=keyword through in the body and surfaces instant_hit_count', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: {
          id: 43,
          semantic_board_id: 1974,
          label: 'ASML|镓锗 出口',
          type: 'keyword',
          status: 'active',
          instant_hit_count: 3,
          created_at: '2026-08-24T00:00:00Z',
          updated_at: '2026-08-24T00:00:00Z',
        },
      })

      const api = useTopicWatchesApi()
      const res = await api.createWatch(1974, 'ASML|镓锗 出口', 'keyword')

      // type 透传到 body
      expect(postMock).toHaveBeenCalledWith('/semantic-boards/1974/topic-watches', { label: 'ASML|镓锗 出口', type: 'keyword' })
      expect(res.success).toBe(true)
      expect(res.data).toMatchObject({
        id: '43',
        type: 'keyword',
        label: 'ASML|镓锗 出口',
      })
      // instant_hit_count → instantHitCount（keyword 即时回扫命中数）
      expect(res.data!.instantHitCount).toBe(3)
    })

    it('passes type=label through explicitly (body carries type even for label)', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: { id: 44, semantic_board_id: 1974, label: 'x', type: 'label', status: 'active', created_at: 'a', updated_at: 'b' },
      })
      const api = useTopicWatchesApi()
      const res = await api.createWatch(1974, 'x', 'label')
      expect(postMock).toHaveBeenCalledWith('/semantic-boards/1974/topic-watches', { label: 'x', type: 'label' })
      expect(res.data!.type).toBe('label')
      // label 类无即时回扫，响应不带 instant_hit_count
      expect(res.data!.instantHitCount).toBeUndefined()
    })

    it('normalizes unexpected type values defensively to label', async () => {
      getMock.mockResolvedValue({
        success: true,
        data: [
          { id: 1, semantic_board_id: 1974, label: 'legacy', status: 'active', created_at: 'a', updated_at: 'b' },
          { id: 2, semantic_board_id: 1974, label: 'kw', type: 'keyword', status: 'paused', created_at: 'c', updated_at: 'd' },
          { id: 3, semantic_board_id: 1974, label: 'weird', type: 'surprise', status: 'active', created_at: 'e', updated_at: 'f' },
        ],
      })
      const api = useTopicWatchesApi()
      const res = await api.listWatches(1974)
      expect(res.data![0]!.type).toBe('label') // 历史行无 type → label
      expect(res.data![1]!.type).toBe('keyword')
      expect(res.data![2]!.type).toBe('label') // 未知值防御性归 label
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

    it('normalizes unexpected type values defensively to label', async () => {
      getMock.mockResolvedValue({
        success: true,
        data: [
          { id: 1, semantic_board_id: 1974, label: 'legacy', status: 'active', created_at: 'a', updated_at: 'b' },
          { id: 2, semantic_board_id: 1974, label: 'kw', type: 'keyword', status: 'paused', created_at: 'c', updated_at: 'd' },
          { id: 3, semantic_board_id: 1974, label: 'weird', type: 'surprise', status: 'active', created_at: 'e', updated_at: 'f' },
        ],
      })
      const api = useTopicWatchesApi()
      const res = await api.listWatches(1974)
      expect(res.data![0]!.type).toBe('label') // 历史行无 type → label
      expect(res.data![1]!.type).toBe('keyword')
      expect(res.data![2]!.type).toBe('label') // 未知值防御性归 label
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

  describe('materialized tracks (watch-materialized-topic)', () => {
    it('createWatch passes query through for sentence_topic', async () => {
      postMock.mockResolvedValue({ success: true, data: {
        id: 300, semantic_board_id: 7, label: 'AI 进展', query: 'AI coding 进展',
        type: 'sentence_topic', persistent_topic_id: 42, status: 'active',
        created_at: '2026-08-25T10:00:00Z', updated_at: '2026-08-25T10:00:00Z',
      } })
      const api = useTopicWatchesApi()
      const res = await api.createWatch(7, 'AI 进展', 'sentence_topic', 'AI coding 进展')
      expect(postMock).toHaveBeenCalledWith('/semantic-boards/7/topic-watches', {
        label: 'AI 进展', type: 'sentence_topic', query: 'AI coding 进展',
      })
      expect(res.data).toMatchObject({
        id: '300', type: 'sentence_topic', query: 'AI coding 进展', persistentTopicId: '42',
      })
    })

    it('normalizes keyword_topic type and omits absent query/topic link', async () => {
      postMock.mockResolvedValue({ success: true, data: {
        id: 301, semantic_board_id: 7, label: 'harness',
        type: 'keyword_topic', status: 'active',
        created_at: '2026-08-25T10:00:00Z', updated_at: '2026-08-25T10:00:00Z',
      } })
      const api = useTopicWatchesApi()
      const res = await api.createWatch(7, 'harness', 'keyword_topic')
      expect(res.data).toMatchObject({ id: '301', type: 'keyword_topic' })
      expect(res.data!.query).toBeUndefined()
      expect(res.data!.persistentTopicId).toBeUndefined()
    })

    it('deleteWatch appends confirm_archive_topic for sentence deletions', async () => {
      deleteMock.mockResolvedValue({ success: true, data: null })
      const api = useTopicWatchesApi()
      await api.deleteWatch('300', true)
      expect(deleteMock).toHaveBeenCalledWith('/topic-watches/300?confirm_archive_topic=true')
      await api.deleteWatch('301')
      expect(deleteMock).toHaveBeenLastCalledWith('/topic-watches/301')
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
