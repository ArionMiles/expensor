import { test as base, expect } from '@playwright/test'
import { enableBrowserMocks } from '../mocks/session'

type BrowserFixtures = {
  gotoMocked: (path: string) => Promise<void>
}

export const test = base.extend<BrowserFixtures>({
  gotoMocked: async ({ page }, use) => {
    await use(async (path: string) => {
      await enableBrowserMocks(page)
      await page.goto(path)
    })
  },
})

export { expect }
