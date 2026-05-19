import { describe, expect, it } from 'vitest'
import { NAVIGATION_TARGETS } from './navigation'

describe('NAVIGATION_TARGETS', () => {
  it('exposes expense group tabs as separate command palette destinations', () => {
    const expenseGroupTargets = NAVIGATION_TARGETS.filter((target) =>
      target.id.startsWith('expense-groups-'),
    )

    expect(
      expenseGroupTargets.map((target) => ({
        id: target.id,
        subtitleKey: target.subtitleKey,
        path: target.path,
      })),
    ).toEqual([
      {
        id: 'expense-groups-categories',
        subtitleKey: 'expenseGroups.tabs.categories',
        path: '/expense-groups?tab=categories',
      },
      {
        id: 'expense-groups-buckets',
        subtitleKey: 'expenseGroups.tabs.buckets',
        path: '/expense-groups?tab=buckets',
      },
      {
        id: 'expense-groups-labels',
        subtitleKey: 'expenseGroups.tabs.labels',
        path: '/expense-groups?tab=labels',
      },
    ])
  })
})
