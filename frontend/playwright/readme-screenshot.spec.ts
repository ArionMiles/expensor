import { expect, test, type Route } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')
const fixturePath = path.join(rootDir, 'docs', 'screenshots', 'dashboard-seed.json')
const screenshotPath = path.join(rootDir, 'docs', 'screenshots', 'dashboard-light.png')

interface ScreenshotFixture {
  status: unknown
  dashboard: unknown
  heatmap: unknown
  annualHeatmap: { year: number; buckets: unknown[] }
  monthlyBreakdown: unknown
  transactions: Array<{ amount: number }>
  facets: unknown
  labels: unknown
  categories: unknown
  buckets: unknown
}

async function loadFixture(): Promise<ScreenshotFixture> {
  return JSON.parse(await readFile(fixturePath, 'utf-8')) as ScreenshotFixture
}

async function fulfillJSON(route: Route) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: '{}',
  })
}

test('captures README dashboard screenshot @screenshot', async ({ page }) => {
  const fixture = await loadFixture()

  await page.addInitScript(() => {
    window.localStorage.setItem('theme', 'light')
    window.localStorage.setItem('sidebar_collapsed', 'false')
  })

  await page.route('**/api/**', async (route) => {
    const requestURL = new URL(route.request().url())
    const pathname = requestURL.pathname.replace(/^\/api/, '')

    const json = async (body: unknown) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(body),
      })
    }

    if (pathname === '/status') return json(fixture.status)
    if (pathname === '/stats/dashboard') return json(fixture.dashboard)
    if (pathname === '/stats/heatmap') return json(fixture.heatmap)
    if (pathname === '/stats/heatmap/annual') {
      const year = Number(requestURL.searchParams.get('year') ?? fixture.annualHeatmap.year)
      return json({ ...fixture.annualHeatmap, year })
    }
    if (pathname === '/stats/labels/monthly') return json(fixture.monthlyBreakdown)
    if (pathname === '/config/timezone') return json({ timezone: 'UTC' })
    if (pathname === '/config/time-format') return json({ time_format: 'HH:mm' })
    if (pathname === '/config/base-currency') return json({ base_currency: 'INR' })
    if (pathname === '/config/labels') return json(fixture.labels)
    if (pathname === '/config/categories') return json(fixture.categories)
    if (pathname === '/config/buckets') return json(fixture.buckets)
    if (pathname === '/config/banks') return json([])
    if (pathname === '/transactions/facets') return json(fixture.facets)
    if (pathname === '/transactions' || pathname === '/transactions/search') {
      const pageSize = Number(requestURL.searchParams.get('page_size') ?? '20')
      const transactions = fixture.transactions.slice(0, Number.isFinite(pageSize) ? pageSize : 20)
      const totalAmount = fixture.transactions.reduce(
        (sum, transaction) => sum + transaction.amount,
        0,
      )
      return json({
        transactions,
        total: fixture.transactions.length,
        total_amount: totalAmount,
        base_currency: 'INR',
      })
    }
    if (pathname === '/version') return json({ version: 'screenshot' })
    if (pathname === '/health') return json({ status: 'ok' })

    await fulfillJSON(route)
  })

  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()
  await expect(page.getByText('Zomato')).toBeVisible()
  await page.screenshot({ path: screenshotPath, fullPage: false })
})
