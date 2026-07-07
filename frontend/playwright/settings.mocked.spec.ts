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

test('AI settings can test an OpenAI API connection @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/settings?tab=ai')

  await expect(page.getByRole('heading', { name: 'OpenAI' })).toBeVisible()
  await expect(page.getByText('Needs setup')).toBeVisible()
  await expect(page.locator('input[aria-label="Base URL"]')).toHaveCount(0)

  await page.getByRole('button', { name: 'Edit base URL' }).click()
  await expect(page.getByRole('textbox', { name: 'Base URL' })).toHaveValue(
    'https://api.openai.com/v1',
  )

  await expect(page.getByLabel('Model')).toHaveValue('GPT-5.4 mini')
  await page.getByLabel('API key').fill('sk-playwright')
  await page.getByRole('button', { name: 'Test' }).click()

  await expect(page.getByText('OpenAI connection is healthy.')).toBeVisible()
})
