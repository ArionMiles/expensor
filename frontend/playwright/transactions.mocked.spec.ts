import { searchParam } from './utils/urls'
import { expect, test } from './fixtures/test'

test('transactions URL state survives reloads and filtering @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/transactions')

  await expect(page.getByRole('searchbox', { name: 'Search transactions' })).toBeVisible()
  await expect(page.getByText('Corner Coffee')).toBeVisible()
  await expect(page.getByText('City Apartments')).toBeVisible()

  await page.getByRole('searchbox', { name: 'Search transactions' }).fill('coffee')
  await expect.poll(() => searchParam(page.url(), 'q')).toBe('coffee')

  await page.getByRole('button', { name: 'Filters', exact: true }).click()
  await page.getByRole('button', { name: 'Add filter' }).click()
  await page.getByRole('menuitem', { name: 'Category' }).click()
  await page.getByRole('combobox', { name: 'Filter by category' }).fill('Food')
  await page.getByRole('combobox', { name: 'Filter by category' }).press('Enter')

  await expect.poll(() => searchParam(page.url(), 'category')).toBe('Food')
  await expect(page.getByText('Corner Coffee')).toBeVisible()
  await expect(page.getByText('City Apartments')).toHaveCount(0)

  await page.reload()

  await expect(page.getByRole('searchbox', { name: 'Search transactions' })).toHaveValue('coffee')
  await expect(page.getByRole('combobox', { name: 'Filter by category' })).toHaveValue('Food')
  await expect(page.getByText('Corner Coffee')).toBeVisible()
  await expect(page.getByText('City Apartments')).toHaveCount(0)
})

test('transaction labels can be selected from the dropdown by keyboard @mocked', async ({
  gotoMocked,
  page,
}) => {
  await gotoMocked('/transactions')

  const cityApartments = page.getByRole('row').filter({ hasText: 'City Apartments' })
  await cityApartments.getByRole('button', { name: 'Add label' }).press('Enter')

  const labelInput = page.getByRole('combobox', { name: 'Add transaction label' })
  await labelInput.fill('gro')
  await labelInput.press('ArrowDown')

  const activeOptionId = await labelInput.getAttribute('aria-activedescendant')
  expect(activeOptionId).toBeTruthy()

  const activeOption = page.locator(`[id="${activeOptionId}"]`)
  await expect(activeOption).toHaveText(/Groceries/)

  await labelInput.press('Enter')
  await expect(labelInput).toHaveCount(0)
})

test('transaction label input closes with one escape press @mocked', async ({ gotoMocked, page }) => {
  await gotoMocked('/transactions')

  const cityApartments = page.getByRole('row').filter({ hasText: 'City Apartments' })
  await cityApartments.getByRole('button', { name: 'Add label' }).press('Enter')

  const labelInput = page.getByRole('combobox', { name: 'Add transaction label' })
  await labelInput.fill('gro')
  await page.keyboard.press('Escape')

  await expect(labelInput).toHaveCount(0)
})

test('transaction filter controls close and select options by keyboard @mocked', async ({
  gotoMocked,
  page,
}) => {
  await gotoMocked('/transactions')

  await page.getByRole('button', { name: 'Filters', exact: true }).click()

  const sourceFilter = page.getByRole('combobox', { name: 'Filter by source' })
  await sourceFilter.focus()
  await expect(page.getByRole('listbox', { name: 'Filter by source options' })).toBeVisible()

  await page.keyboard.press('Tab')
  await expect(page.getByRole('listbox', { name: 'Filter by source options' })).toHaveCount(0)

  const addFilter = page.getByRole('button', { name: 'Add filter' })
  await addFilter.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('menu', { name: 'Add filter options' })).toBeVisible()

  await page.keyboard.press('Escape')
  await expect(page.getByRole('menu', { name: 'Add filter options' })).toHaveCount(0)

  await addFilter.focus()
  await page.keyboard.press('Enter')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')

  await expect(page.getByRole('combobox', { name: 'Filter by category' })).toBeVisible()
})
