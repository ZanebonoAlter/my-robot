import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock, patchMock, deleteMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
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

import { useDailyReportsApi } from './dailyReports'

describe('useDailyReportsApi — getBoardSectionTimeline time-range params (task 2.5)', () => {
  beforeEach(() => {
    getMock.mockReset()
    getMock.mockResolvedValue({ success: true, data: { sections: [], relations: [] } })
  })

  it('sends ?days=7 for the 7-day window', async () => {
    const api = useDailyReportsApi()
    await api.getBoardSectionTimeline(1974, 7)
    expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/section-timeline?days=7')
  })

  it('sends ?days=14 for the default 14-day window', async () => {
    const api = useDailyReportsApi()
    await api.getBoardSectionTimeline(1974, 14)
    expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/section-timeline?days=14')
  })

  it('sends ?days=30 for the 30-day window', async () => {
    const api = useDailyReportsApi()
    await api.getBoardSectionTimeline(1974, 30)
    expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/section-timeline?days=30')
  })

  it('sends ?days=0 for 全部 (all history), not an empty query', async () => {
    // "全部" 约定：days=0 → 显式 ?days=0，后端 <=0 即不限天。
    const api = useDailyReportsApi()
    await api.getBoardSectionTimeline(1974, 0)
    expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/section-timeline?days=0')
  })

  it('omits the query when days is undefined (backend default)', async () => {
    const api = useDailyReportsApi()
    await api.getBoardSectionTimeline(1974)
    expect(getMock).toHaveBeenCalledWith('/semantic-boards/1974/section-timeline')
  })
})
