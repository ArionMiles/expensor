import type { Page } from '@playwright/test'

const E2E_MOCKS_KEY = 'expensor:e2e-mocks'

export async function enableBrowserMocks(page: Page) {
  await page.addInitScript((storageKey) => {
    window.localStorage.setItem(storageKey, '1')
  }, E2E_MOCKS_KEY)
}
