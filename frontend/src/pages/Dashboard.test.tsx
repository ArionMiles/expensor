import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import {
  BreakdownTimeline,
  DEFAULT_SPEND_BREAKDOWN_MODE,
  DEFAULT_HEATMAP_METRIC,
  MetricToggle,
  dashboardBreakdownData,
  dashboardBreakdownParams,
  displayBucketLabel,
  topBreakdownSlices,
} from './Dashboard'

describe('BreakdownTimeline', () => {
  it('rerenders from populated data to empty data without changing hook order', () => {
    const populated = {
      labels: ['Groceries'],
      months: ['2026-03', '2026-04'],
      series: [{ label: 'Groceries', data: [10, 20] }],
    }
    const empty = {
      labels: [],
      months: ['2026-03', '2026-04'],
      series: [],
    }

    const { rerender } = render(
      <BreakdownTimeline
        data={populated}
        currency="INR"
        mode="labels"
        onModeChange={() => undefined}
      />,
    )

    rerender(
      <BreakdownTimeline
        data={empty}
        currency="INR"
        mode="labels"
        onModeChange={() => undefined}
      />,
    )

    expect(screen.getByText('No data')).toBeInTheDocument()
  })

  it('orders breakdown toggles as Categories, Buckets, Labels', () => {
    render(
      <BreakdownTimeline
        data={{ labels: [], months: [], series: [] }}
        currency="INR"
        mode="categories"
        onModeChange={() => undefined}
      />,
    )

    const buttons = screen.getAllByRole('button').map((button) => button.textContent)
    expect(buttons).toEqual(['Categories', 'Buckets', 'Labels'])
  })

  it('uses categories as the default breakdown mode', () => {
    expect(DEFAULT_SPEND_BREAKDOWN_MODE).toBe('categories')
  })
})

describe('MetricToggle', () => {
  it('uses count as the default heatmap metric', () => {
    expect(DEFAULT_HEATMAP_METRIC).toBe('count')
  })

  it('uses the dashboard segmented toggle styling', () => {
    render(<MetricToggle value="count" onChange={() => undefined} />)

    expect(screen.getByRole('group', { name: 'Heatmap metric' })).toHaveClass(
      'bg-card',
      'p-0.5',
      'text-xs',
    )
    expect(screen.getByRole('button', { name: 'Count' })).toHaveClass(
      'rounded',
      'px-3',
      'py-1.5',
      'bg-primary',
    )
    expect(screen.getByRole('button', { name: 'Amount' })).toHaveClass(
      'text-muted-foreground',
      'hover:text-foreground',
    )
  })
})

describe('dashboard uncategorized display helpers', () => {
  it('surfaces missing values as Uncategorized without changing configured labels', () => {
    expect(dashboardBreakdownData({ Food: 100, '': 50, Uncategorized: 25 })).toEqual({
      Food: 100,
      Uncategorized: 75,
    })
  })

  it('routes Uncategorized slices to missing-value transaction filters', () => {
    expect(dashboardBreakdownParams('category', 'Uncategorized')).toEqual({
      category_missing: '1',
      show_filters: '1',
    })
    expect(dashboardBreakdownParams('label', 'Uncategorized')).toEqual({
      label_missing: '1',
      show_filters: '1',
    })
    expect(dashboardBreakdownParams('source_type', 'Credit Card')).toEqual({
      source_type: 'Credit Card',
      show_filters: '1',
    })
    expect(dashboardBreakdownParams('bank', 'HDFC')).toEqual({
      bank: 'HDFC',
      show_filters: '1',
    })
  })

  it('keeps investments as the configured bucket label', () => {
    expect(displayBucketLabel('Investments')).toBe('Investments')
    expect(dashboardBreakdownParams('bucket', 'Investments')).toEqual({
      bucket: 'Investments',
      show_filters: '1',
    })
  })

  it('does not synthesize Misc slices for category donuts', () => {
    const slices = topBreakdownSlices({
      Food: 100,
      Transport: 90,
      Utilities: 80,
      Shopping: 70,
      Healthcare: 60,
      Miscellaneous: 50,
      Uncategorized: 40,
    })

    expect(slices.map((slice) => slice.label)).toEqual([
      'Food',
      'Transport',
      'Utilities',
      'Shopping',
      'Healthcare',
      'Miscellaneous',
      'Uncategorized',
    ])
    expect(slices.some((slice) => slice.label === 'Misc')).toBe(false)
  })
})
