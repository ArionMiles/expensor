import AxeBuilder from '@axe-core/playwright'
import { expect, test } from './fixtures/test'

const routes = ['/', '/transactions', '/rules', '/settings', '/setup']

for (const route of routes) {
  test(`has no critical accessibility violations on ${route} @mocked`, async ({ gotoMocked, page }) => {
    await gotoMocked(route)

    const results = await new AxeBuilder({ page }).disableRules(['color-contrast']).analyze()
    const critical = results.violations.filter((violation) => violation.impact === 'critical' || violation.impact === 'serious')

    expect(critical).toEqual([])
  })
}
