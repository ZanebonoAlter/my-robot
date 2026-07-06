import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock, putMock, deleteMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn(),
  deleteMock: vi.fn(),
}))

vi.mock('./client', () => ({
  apiClient: {
    get: getMock,
    post: postMock,
    put: putMock,
    delete: deleteMock,
  },
}))

import { useBoardEnrichmentApi } from './boardEnrichment'

describe('useBoardEnrichmentApi — endpoint contracts (data-enrichment P6)', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
    putMock.mockReset()
    deleteMock.mockReset()
    getMock.mockResolvedValue({ success: true, data: {} })
    postMock.mockResolvedValue({ success: true, data: {} })
    putMock.mockResolvedValue({ success: true, data: {} })
    deleteMock.mockResolvedValue({ success: true, data: {} })
  })

  // ── Table 1: topic_lifeline_context (topic dimension) ───────────────────
  describe('table 1 — contexts', () => {
    it('listContexts → GET /persistent-topics/:topicId/enrichment/contexts', async () => {
      const api = useBoardEnrichmentApi()
      await api.listContexts(7)
      expect(getMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/contexts')
    })

    it('getContext → GET .../contexts/:granularity', async () => {
      const api = useBoardEnrichmentApi()
      await api.getContext(7, 'week')
      expect(getMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/contexts/week')
    })

    it('updateContext → PUT .../contexts/:granularity with {content}', async () => {
      const api = useBoardEnrichmentApi()
      await api.updateContext(7, 'month', { content: 'hello' })
      expect(putMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/contexts/month', { content: 'hello' })
    })

    it('regenerateContext → POST .../contexts/:granularity/regenerate', async () => {
      const api = useBoardEnrichmentApi()
      await api.regenerateContext(7, 'year')
      expect(postMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/contexts/year/regenerate')
    })
  })

  // ── Table 2: topic_enrichment_result (topic dimension) ──────────────────
  describe('table 2 — results', () => {
    it('listResults → GET .../results', async () => {
      const api = useBoardEnrichmentApi()
      await api.listResults(7)
      expect(getMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/results')
    })

    it('getResult → GET .../results/:id (topicId kept in path per backend route)', async () => {
      const api = useBoardEnrichmentApi()
      await api.getResult(7, 42)
      expect(getMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/results/42')
    })

    it('triggerEnrichment → POST .../results/trigger', async () => {
      const api = useBoardEnrichmentApi()
      await api.triggerEnrichment(7)
      expect(postMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/results/trigger')
    })
  })

  // ── Table 3: topic_enrichment_review (topic dimension) ──────────────────
  describe('table 3 — reviews', () => {
    it('listReviews → GET .../reviews', async () => {
      const api = useBoardEnrichmentApi()
      await api.listReviews(7)
      expect(getMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/reviews')
    })

    it('createReview → POST .../reviews with manual body', async () => {
      const api = useBoardEnrichmentApi()
      await api.createReview(7, { curr_result_id: 10, deviation_summary: '手动批注' })
      expect(postMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/reviews', {
        curr_result_id: 10,
        deviation_summary: '手动批注',
      })
    })

    it('createReview forwards optional prev_result_id', async () => {
      const api = useBoardEnrichmentApi()
      await api.createReview(7, { curr_result_id: 10, deviation_summary: 'x', prev_result_id: 9 })
      expect(postMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/reviews', {
        curr_result_id: 10,
        deviation_summary: 'x',
        prev_result_id: 9,
      })
    })

    it('updateReviewDeviation → PUT .../reviews/:id with {deviation_summary}', async () => {
      const api = useBoardEnrichmentApi()
      await api.updateReviewDeviation(7, 3, { deviation_summary: '调整' })
      expect(putMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/reviews/3', { deviation_summary: '调整' })
    })

    it('applyReview → POST .../reviews/:id/apply', async () => {
      const api = useBoardEnrichmentApi()
      await api.applyReview(7, 3)
      expect(postMock).toHaveBeenCalledWith('/persistent-topics/7/enrichment/reviews/3/apply')
    })
  })

  // ── Board data sources (board dimension) ────────────────────────────────
  describe('board — data sources', () => {
    it('listDataSources → GET /semantic-boards/:id/data-sources', async () => {
      const api = useBoardEnrichmentApi()
      await api.listDataSources(1974)
      expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/data-sources')
    })

    it('upsertDataSource → PUT /semantic-boards/:id/data-sources with body', async () => {
      const api = useBoardEnrichmentApi()
      await api.upsertDataSource(1974, {
        source_type: 'etf_quote',
        config: { keywords: ['半导体'] },
        enabled: true,
      })
      expect(putMock).toHaveBeenCalledWith('/semantic-boards/1974/data-sources', {
        source_type: 'etf_quote',
        config: { keywords: ['半导体'] },
        enabled: true,
      })
    })

    it('deleteDataSource → DELETE /semantic-boards/:id/data-sources/:sourceType', async () => {
      const api = useBoardEnrichmentApi()
      await api.deleteDataSource(1974, 'etf_quote')
      expect(deleteMock).toHaveBeenCalledWith('/semantic-boards/1974/data-sources/etf_quote')
    })
  })
})
