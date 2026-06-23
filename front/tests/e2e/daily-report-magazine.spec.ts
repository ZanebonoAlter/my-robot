import { expect, test, type Page, type Route } from '@playwright/test'

const board = {
  id: 1974,
  label: '中东地缘政治与美伊关系',
  slug: 'middle-east-fixture',
  aliases: [],
  ref_count: 19,
  tag_count: 2,
  description: '日报杂志阅读器 E2E fixture',
  display_order: 1,
  source: 'manual',
  status: 'active',
  protected: false,
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-21T00:00:00Z',
}

const reports = [
  {
    id: 60,
    semantic_board_id: board.id,
    period_date: '2026-06-21',
    title: '伊朗宣布关闭霍尔木兹海峡致通行归零',
    summary: '海峡封锁与谈判并行，地区风险继续上升。',
    status: 'completed',
    cluster_count: 2,
    article_count: 3,
    event_tag_count: 2,
    created_at: '2026-06-21T08:00:00Z',
  },
  {
    id: 59,
    semantic_board_id: board.id,
    period_date: '2026-06-20',
    title: '美伊恢复瑞士谈判',
    summary: '谈判窗口重新开启。',
    status: 'completed',
    cluster_count: 1,
    article_count: 1,
    event_tag_count: 1,
    created_at: '2026-06-20T08:00:00Z',
  },
]

function reportDetail(id: number) {
  const report = reports.find(item => item.id === id) || reports[0]
  const current = report.id === 60
  return {
    ...report,
    highlights: [{ title: report.title, reason: report.summary, tag_ids: [1] }],
    dynamics: current ? '航运、外交与能源市场同时承压。' : '外交磋商恢复。',
    sections: current
      ? [
          {
            id: 11,
            cluster_index: 0,
            cluster_label: '霍尔木兹海峡封锁解除与航运恢复',
            cluster_tag_ids: [1],
            article_count: 2,
            best_tier: 1,
            avg_score: 0.96,
            persistent_topic_id: 501,
            persistent_topic: {
              id: 501,
              label: '霍尔木兹海峡封锁解除与航运恢复',
              status: 'active',
              color: '#b34d37',
              consecutive_hits: 4,
              can_activate: false,
            },
            threads: [{
              id: 101,
              report_id: 60,
              section_id: 11,
              title: '海峡封锁从威胁转为现实',
              summary: '通行量归零后，航运公司开始绕行。',
              tag_ids: [1],
              confidence: 0.95,
              related_article_ids: [9001],
              created_at: '2026-06-21T08:00:00Z',
            }],
          },
          {
            id: 12,
            cluster_index: 1,
            cluster_label: '谈判核心议题与资产解冻',
            cluster_tag_ids: [2],
            article_count: 1,
            best_tier: 2,
            avg_score: 0.81,
            persistent_topic_id: 502,
            persistent_topic: {
              id: 502,
              label: '谈判核心议题与资产解冻',
              status: 'candidate',
              color: '#7d6aa5',
              consecutive_hits: 1,
              can_activate: false,
            },
            threads: [],
          },
        ]
      : [],
  }
}

const article = {
  id: 9001,
  feed_id: 1,
  title: '霍尔木兹海峡通行量降至零',
  description: '航运监测显示海峡通行量已降至零。',
  content: '<p>这是用于验证日报文章预览的数据。</p>',
  link: 'https://example.com/articles/9001',
  pub_date: '2026-06-21T06:00:00Z',
  created_at: '2026-06-21T06:00:00Z',
  read: false,
  favorite: false,
  tags: [],
}

async function json(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data }) })
}

async function installFixture(page: Page) {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname

    if (path === '/api/semantic-boards') return json(route, { items: [board], total: 1 })
    if (path === `/api/semantic-boards/${board.id}/composition`) return json(route, { items: [], total: 0 })
    if (path === '/api/auxiliary-labels') return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data: { items: [] }, pagination: { page: 1, pages: 1, total: 0 } }),
    })
    if (path === '/api/auxiliary-labels/clusters') return json(route, { clusters: [], unclustered_count: 0 })
    if (path === `/api/semantic-boards/${board.id}/daily-reports`) return json(route, { reports })
    if (path === '/api/daily-reports/60') return json(route, { report: reportDetail(60) })
    if (path === '/api/daily-reports/59') return json(route, { report: reportDetail(59) })
    if (path === '/api/daily-reports/topics/501/lifeline') return json(route, {
      sections: [
        { id: 10, report_id: 59, period_date: '2026-06-20', cluster_label: '海峡封锁预警', status: 'continuing', article_count: 1, thread_count: 1, persistent_topic_id: 501 },
        { id: 11, report_id: 60, period_date: '2026-06-21', cluster_label: '海峡封锁成为现实', status: 'continuing', article_count: 2, thread_count: 1, persistent_topic_id: 501 },
      ],
      relations: [
        { from_id: 10, to_id: 11, distance: 0, relation_type: 'identity' },
        { from_id: 10, to_id: 12, distance: 0.2, relation_type: 'similarity' },
      ],
    })
    if (path === '/api/articles/9001') return json(route, article)
    return json(route, {})
  })
}

async function openLatestReport(page: Page) {
  await page.goto('/tags', { waitUntil: 'networkidle' })
  await page.getByText(board.label, { exact: true }).click()
  await page.getByRole('button', { name: '日报' }).click()
  await page.getByRole('button', { name: /6 月 21 日/ }).click()
  await expect(page.getByRole('dialog', { name: '日报详情' })).toBeVisible()
}

test.describe('daily report magazine reader', () => {
  test.beforeEach(async ({ page }) => {
    await installFixture(page)
  })

  test('covers both themes and the 1440/1000/720 responsive modes', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await openLatestReport(page)

    for (const viewport of [{ width: 1440, height: 1000 }, { width: 1000, height: 900 }, { width: 720, height: 900 }]) {
      await page.setViewportSize(viewport)
      await expect(page.getByTestId('daily-report-masthead')).toBeVisible()
      expect(await page.locator('.drm-reader').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
    }

    await page.getByRole('button', { name: '关闭日报' }).click()
    const themeToggle = page.getByRole('button', { name: /切换为.+模式/ })
    await themeToggle.click()
    await page.getByRole('button', { name: /6 月 21 日/ }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', /^(dark|editorial)$/)
    await expect(page.getByTestId('daily-report-masthead')).toBeVisible()
  })

  test('expands topic, lifeline day, thread article and navigates report dates', async ({ page }) => {
    await openLatestReport(page)
    const dialog = page.getByRole('dialog', { name: '日报详情' })

    const topic = dialog.getByRole('button', { name: /01 关心 · 持续追踪 霍尔木兹海峡封锁解除与航运恢复/ })
    if (await topic.getAttribute('aria-expanded') !== 'true') await topic.click()
    await expect(dialog.getByRole('heading', { name: '关心的话题' })).toBeVisible()
    await dialog.getByRole('button', { name: '2026-06-21，1 个节点' }).click()
    await expect(dialog.getByRole('region', { name: '历史节点详情' })).toBeVisible()

    await dialog.getByRole('button', { name: /海峡封锁从威胁转为现实/ }).click()
    await dialog.getByRole('button', { name: '霍尔木兹海峡通行量降至零' }).click()
    await expect(page.getByText('这是用于验证日报文章预览的数据。')).toBeVisible()

    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: /6 月 20 日/ }).last().click()
    await expect(dialog.getByRole('heading', { name: '美伊恢复瑞士谈判', level: 2 })).toBeVisible()
  })
})
