import { expect } from 'vitest'
import { axe } from 'vitest-axe'

export async function expectNoA11yViolations(container: HTMLElement) {
  const results = await axe(container)
  expect(results.violations).toEqual([])
}
