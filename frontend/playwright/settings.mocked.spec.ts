import { searchParam } from './utils/urls'
import { expect, test } from './fixtures/test'

test('settings tab persists in the URL and on reload @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/settings')

  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
  await page.getByRole('button', { name: 'Community' }).click()

  await expect.poll(() => searchParam(page.url(), 'tab')).toBe('sync')
  await expect(page.getByRole('heading', { name: 'Sync status' })).toBeVisible()

  await page.reload()

  await expect.poll(() => searchParam(page.url(), 'tab')).toBe('sync')
  await expect(page.getByRole('heading', { name: 'Sync status' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sync now' })).toBeVisible()
})
