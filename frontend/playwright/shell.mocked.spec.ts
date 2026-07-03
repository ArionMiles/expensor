import { expect, test } from './fixtures/test'

test('shell navigation works across the main sections @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/')
  const nav = page.getByRole('navigation', { name: 'Main navigation' })

  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await nav.getByRole('link', { name: 'Transactions', exact: true }).click()
  await expect(page.getByRole('searchbox', { name: 'Search transactions' })).toBeVisible()

  await nav.getByRole('link', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()

  await nav.getByRole('link', { name: 'Dashboard', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()
})

test('command palette closes with one escape press @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/')
  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await page.evaluate(() => {
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'k', code: 'KeyK', ctrlKey: true, bubbles: true }),
    )
  })
  await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible()

  await page.keyboard.press('Escape')

  await expect(page.getByRole('dialog', { name: 'Command palette' })).toHaveCount(0)
})

test('command palette closes with one escape press after typing in the search input @mocked', async ({
  gotoMocked,
  page,
}) => {
  await gotoMocked('/')
  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await page.evaluate(() => {
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'k', code: 'KeyK', ctrlKey: true, bubbles: true }),
    )
  })
  await page.getByRole('textbox', { name: 'Search commands' }).fill('trans')

  await page.keyboard.press('Escape')

  await expect(page.getByRole('dialog', { name: 'Command palette' })).toHaveCount(0)
})
