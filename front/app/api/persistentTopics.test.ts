import { beforeEach, describe, expect, it, vi } from 'vitest'

const { postMock, getMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  getMock: vi.fn(),
}))

vi.mock('./client', () => ({
  apiClient: {
    get: getMock,
    post: postMock,
  },
}))

import { usePersistentTopicsApi } from './persistentTopics'

describe('usePersistentTopicsApi — 手动建泳道编排态（切片③）', () => {
  beforeEach(() => {
    postMock.mockReset()
    getMock.mockReset()
  })

  describe('getComposeCandidates', () => {
    it('GETs compose-candidates with days and normalizes snake_case + string ids', async () => {
      getMock.mockResolvedValue({
        success: true,
        data: {
          sections: [
            {
              id: 101,
              period_date: '2026-06-18T00:00:00Z',
              cluster_label: '霍尔木兹油轮遇袭',
              embedding: [0.12, 0.34, 0.56],
              persistent_topic_id: 7,
              topic_match_confidence: 'anchor_hit',
              persistent_topic: { id: 7, label: '中东局势', status: 'active', color: '#b44f45' },
            },
            {
              id: 102,
              period_date: '2026-06-19T00:00:00Z',
              cluster_label: '无归属 section',
              embedding: [0.01, 0.02],
            },
          ],
          match_threshold: 0.3,
        },
      })

      const api = usePersistentTopicsApi()
      const res = await api.getComposeCandidates(1974, 14)

      expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/persistent-topics/compose-candidates?days=14')
      expect(res.success).toBe(true)
      const data = res.data!
      expect(data.matchThreshold).toBe(0.3)
      expect(data.sections).toHaveLength(2)
      // snake_case → camelCase
      expect(data.sections[0]).toMatchObject({
        id: '101',
        periodDate: '2026-06-18T00:00:00Z',
        clusterLabel: '霍尔木兹油轮遇袭',
        persistentTopicId: '7',
        topicMatchConfidence: 'anchor_hit',
      })
      // numeric id → string
      expect(typeof data.sections[0]!.id).toBe('string')
      expect(data.sections[0]!.persistentTopic).toEqual({ id: '7', label: '中东局势', status: 'active' })
      // embedding 原样透传（number[]）
      expect(data.sections[0]!.embedding).toEqual([0.12, 0.34, 0.56])
      // 无归属 section 不带 persistent* 字段
      expect(data.sections[1]!.persistentTopicId).toBeUndefined()
      expect(data.sections[1]!.persistentTopic).toBeUndefined()
    })

    it('tolerates empty sections array', async () => {
      getMock.mockResolvedValue({ success: true, data: { sections: [], match_threshold: 0.3 } })
      const api = usePersistentTopicsApi()
      const res = await api.getComposeCandidates(1, 7)
      expect(res.success).toBe(true)
      expect(res.data!.sections).toEqual([])
    })

    it('propagates failure without throwing', async () => {
      getMock.mockResolvedValue({ success: false, error: 'boom' })
      const api = usePersistentTopicsApi()
      const res = await api.getComposeCandidates(1, 14)
      expect(res.success).toBe(false)
      expect(res.error).toBe('boom')
    })
  })

  describe('createManualLane', () => {
    it('POSTs {label, section_ids:number[]} and normalizes the topic + skipped ids', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: {
          topic: { id: 20, label: '美伊博弈', status: 'active', source: 'manual' },
          skipped: [999],
        },
        message: '1 条 section 因无向量被跳过',
      })

      const api = usePersistentTopicsApi()
      const res = await api.createManualLane(1974, '美伊博弈', ['101', '102', '999'])

      // string ids → numbers in request body (backend receives []uint)
      expect(postMock).toHaveBeenCalledWith(
        '/semantic-boards/1974/persistent-topics/manual',
        { label: '美伊博弈', section_ids: [101, 102, 999] },
      )
      expect(res.success).toBe(true)
      expect(res.data!.topic).toEqual({ id: '20', label: '美伊博弈', status: 'active', source: 'manual' })
      expect(res.data!.skipped).toEqual(['999'])
      expect(res.message).toContain('跳过')
    })

    it('defaults skipped to [] when absent', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: { topic: { id: 21, label: 'X', status: 'active', source: 'manual' } },
      })
      const api = usePersistentTopicsApi()
      const res = await api.createManualLane(1, 'X', ['1'])
      expect(res.success).toBe(true)
      expect(res.data!.skipped).toEqual([])
    })

    it('propagates backend error without throwing', async () => {
      postMock.mockResolvedValue({ success: false, error: 'no usable vector' })
      const api = usePersistentTopicsApi()
      const res = await api.createManualLane(1, 'X', ['1'])
      expect(res.success).toBe(false)
      expect(res.error).toBe('no usable vector')
    })
  })

  describe('embedQuery', () => {
    it('POSTs {query} and returns embedding number[]', async () => {
      postMock.mockResolvedValue({
        success: true,
        data: { embedding: [0.1, 0.2, 0.3] },
      })
      const api = usePersistentTopicsApi()
      const res = await api.embedQuery(1974, '半导体出口管制')
      expect(postMock).toHaveBeenCalledWith(
        '/semantic-boards/1974/persistent-topics/embed-query',
        { query: '半导体出口管制' },
      )
      expect(res.success).toBe(true)
      expect(res.data!.embedding).toEqual([0.1, 0.2, 0.3])
    })

    it('tolerates missing embedding (defaults to [])', async () => {
      postMock.mockResolvedValue({ success: true, data: {} })
      const api = usePersistentTopicsApi()
      const res = await api.embedQuery(1, 'x')
      expect(res.success).toBe(true)
      expect(res.data!.embedding).toEqual([])
    })

    it('propagates failure without throwing', async () => {
      postMock.mockResolvedValue({ success: false, error: 'model down' })
      const api = usePersistentTopicsApi()
      const res = await api.embedQuery(1, 'x')
      expect(res.success).toBe(false)
      expect(res.error).toBe('model down')
    })
  })
})
