import { test, expect } from '@playwright/test'

/**
 * Baseline e2e smoke test.
 *
 * This test verifies:
 * 1. Playwright can connect to the dev server at localhost:3000
 * 2. The home page loads without crash and the Vue app shell renders
 *
 * Note: This baseline test ensures the test infrastructure works.
 */

test.describe('baseline', () => {
  test('server responds and page loads', async ({ page }) => {
    // Navigate to the home page
    const response = await page.goto('/')

    // Server should respond
    expect(response?.status()).toBeLessThan(500)

    // Page should have some content (the Vue app shell)
    await expect(page.locator('body')).toBeVisible()
  })

  test('home page renders Vue app content', async ({ page }) => {
    await page.goto('/')

    // Wait for the page to have any content
    await page.waitForLoadState('networkidle')

    // Check that some element exists on the page (verifying Vue rendered)
    const html = await page.content()
    expect(html.length).toBeGreaterThan(100)
  })
})
