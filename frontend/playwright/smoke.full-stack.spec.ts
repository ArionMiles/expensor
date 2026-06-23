import { expect, test } from '@playwright/test'

test('real stack smoke renders seeded dashboard and transactions @smoke', async ({ page }) => {
  const login = await page.request.post('/api/session', {
    data: {
      email: 'component-admin@example.com',
      password: 'component admin password',
    },
  })
  expect(login.ok()).toBeTruthy()
  const setCookie = login.headers()['set-cookie']
  const session = /expensor_session=([^;]+)/.exec(setCookie ?? '')
  expect(session?.[1]).toBeTruthy()
  await page.context().addCookies([
    {
      name: 'expensor_session',
      value: session![1],
      url: 'http://127.0.0.1:4173',
      httpOnly: true,
      sameSite: 'Lax',
    },
  ])

  await page.goto('/')
  const nav = page.getByRole('navigation', { name: 'Main navigation' })

  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await nav.getByRole('link', { name: 'Transactions', exact: true }).click()
  await expect(page.locator('tbody tr', { hasText: 'Swiggy' }).first()).toBeVisible()
  await expect(page.locator('tbody tr', { hasText: 'BLINKIT' }).first()).toBeVisible()
})
