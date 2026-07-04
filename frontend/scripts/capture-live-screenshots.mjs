import { chromium } from '@playwright/test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')
const outDir = path.join(rootDir, 'docs', 'screenshots')
const baseURL = trimTrailingSlash(process.env.EXPENSOR_SCREENSHOT_BASE_URL ?? 'http://127.0.0.1:4173')
const apiURL = trimTrailingSlash(process.env.EXPENSOR_SCREENSHOT_API_URL ?? 'http://127.0.0.1:18080')
const email = process.env.EXPENSOR_SCREENSHOT_EMAIL ?? 'john.smith@example.com'
const password = process.env.EXPENSOR_SCREENSHOT_PASSWORD ?? 'component admin password'

const shots = [
  { path: '/?summary=all_time', name: 'dashboard-light.png', theme: 'light' },
  { path: '/transactions', name: 'transactions-light.png', theme: 'light' },
  { path: '/?summary=all_time', name: 'dashboard-dark.png', theme: 'dark' },
  { path: '/transactions', name: 'transactions-dark.png', theme: 'dark' },
]

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, '')
}

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 2048, height: 1200 }, deviceScaleFactor: 2 })
const page = await context.newPage()

try {
  const health = await context.request.get(`${apiURL}/api/health`)
  if (!health.ok()) {
    throw new Error(`screenshot backend health check failed: ${health.status()} ${health.statusText()}`)
  }

  const login = await context.request.post(`${baseURL}/api/session`, {
    data: { email, password },
  })
  if (!login.ok()) {
    throw new Error(`screenshot login failed: ${login.status()} ${login.statusText()}`)
  }

  for (const shot of shots) {
    await page.goto('about:blank')
    await page.addInitScript((selectedTheme) => {
      window.localStorage.setItem('theme', selectedTheme)
      window.localStorage.setItem('sidebar_collapsed', 'false')
      window.localStorage.setItem('expensor.dashboard.summaryMode', 'all_time')
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

    if (shot.name.startsWith('dashboard-')) {
      await page.getByRole('heading', { name: 'Dashboard Summary' }).waitFor({ state: 'visible', timeout: 15000 })
      await page.getByText('Spending Patterns').waitFor({ state: 'visible', timeout: 15000 })
    } else {
      await page.getByPlaceholder('Search transactions...').waitFor({ state: 'visible', timeout: 15000 })
      await page.getByText('Swiggy').first().waitFor({ state: 'visible', timeout: 15000 })
    }

    await page.screenshot({ path: path.join(outDir, shot.name), fullPage: true, omitBackground: true })
  }
} finally {
  await browser.close()
}
