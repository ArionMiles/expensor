import { searchParam } from './utils/urls'
import { expect, test } from './fixtures/test'

test('dashboard year state persists and drilldown reaches transactions @mocked', async ({
  gotoMocked,
  page,
}) => {
  await gotoMocked('/')

  await expect(page.getByRole('heading', { name: 'Dashboard Summary' })).toBeVisible()

  await page.getByRole('button', { name: 'Previous year' }).click()
  await expect.poll(() => searchParam(page.url(), 'heatmap_year')).toBe('2025')

  await page.reload()
  await expect.poll(() => searchParam(page.url(), 'heatmap_year')).toBe('2025')

  await page.getByRole('button', { name: /Total spend/i }).click()
  await expect(page).toHaveURL(/\/transactions\?/)
  await expect(page).toHaveURL(/date_from=/)
  await expect(page).toHaveURL(/date_to=/)
  await expect(page.getByRole('searchbox', { name: 'Search transactions' })).toBeVisible()
})
