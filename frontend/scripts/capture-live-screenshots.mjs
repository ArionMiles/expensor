import { chromium } from '@playwright/test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')
const outDir = path.join(rootDir, 'docs', 'screenshots')
const baseURL = process.env.VITE_API_PROXY_TARGET ?? 'http://127.0.0.1:18080'

const shots = [
  { path: '/', name: 'dashboard-light.png', theme: 'light' },
  { path: '/transactions', name: 'transactions-light.png', theme: 'light' },
  { path: '/', name: 'dashboard-dark.png', theme: 'dark' },
  { path: '/transactions', name: 'transactions-dark.png', theme: 'dark' },
]

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 2048, height: 1200 }, deviceScaleFactor: 2 })
const page = await context.newPage()

for (const shot of shots) {
  await page.goto('about:blank')
  await page.addInitScript((selectedTheme) => {
    window.localStorage.setItem('theme', selectedTheme)
    window.localStorage.setItem('sidebar_collapsed', 'false')
    window.localStorage.setItem('expensor.dashboard.summaryMode', 'current')
    window.localStorage.setItem('expensor.dashboard.showUncategorizedLabels', 'true')
  }, shot.theme)

  await page.goto(`${baseURL}${shot.path}`, { waitUntil: 'networkidle' })
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.addStyleTag({
    content: `
      html, body { background: transparent !important; }
      body { margin: 0 !important; }
      #root {
        overflow: hidden !important;
        border-radius: 28px !important;
        background: hsl(var(--background)) !important;
      }
      #root > div {
        height: auto !important;
        min-height: 100vh !important;
        overflow: visible !important;
        align-items: stretch !important;
      }
      #root > div > div,
      #root main {
        overflow: visible !important;
      }
      #root main {
        flex: 0 0 auto !important;
      }
    `,
  })

  if (shot.path === '/') {
    await page.getByRole('heading', { name: 'Dashboard Summary' }).waitFor({ state: 'visible', timeout: 15000 })
    await page.getByText('daemon running').waitFor({ state: 'visible', timeout: 15000 })
    await page.getByText('Spending Patterns').waitFor({ state: 'visible', timeout: 15000 })
  } else {
    await page.getByPlaceholder('Search transactions...').waitFor({ state: 'visible', timeout: 15000 })
    await page.getByText('Swiggy').first().waitFor({ state: 'visible', timeout: 15000 })
  }

  await page.screenshot({ path: path.join(outDir, shot.name), fullPage: true, omitBackground: true })
}

await browser.close()
