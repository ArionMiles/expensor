import { expect, test } from '@playwright/test'

test('real stack smoke renders seeded dashboard and transactions @smoke', async ({ page }) => {
  await page.goto('/')
  const nav = page.getByRole('navigation', { name: 'Main navigation' })

  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await nav.getByRole('link', { name: 'Transactions', exact: true }).click()
  await expect(page.locator('tbody tr', { hasText: 'Swiggy' }).first()).toBeVisible()
  await expect(page.locator('tbody tr', { hasText: 'BLINKIT' }).first()).toBeVisible()
})
