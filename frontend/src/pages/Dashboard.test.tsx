import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { ChartData, Stats } from '@/api/types'
import { I18nProvider } from '@/i18n/I18nProvider'
import {
  BreakdownTimeline,
  DEFAULT_SPEND_BREAKDOWN_MODE,
  DEFAULT_HEATMAP_METRIC,
  MetricToggle,
  SummarySection,
  dashboardBreakdownData,
  dashboardBreakdownParams,
  displayBucketLabel,
  topBreakdownSlices,
} from './Dashboard'

const chartData: ChartData = {
  monthly_spend: [],
  daily_spend: [],
  by_category: { Food: 1200, Travel: 500 },
  by_bucket: { Needs: 1000, Wants: 700 },
  by_label: { Online: 900, Store: 300 },
  by_source: {},
  by_source_type: { 'Credit Card': 1400, UPI: 300 },
  by_bank: { HDFC: 1100, ICICI: 600 },
  by_category_monthly: {
    Food: { current: 1200, prior: 900 },
    Travel: { current: 500, prior: 700 },
  },
}

const stats: Stats = {
  total_count: 3,
  total_base: 1700,
  base_currency: 'INR',
  total_by_category: { Food: 1200, Travel: 500 },
  total_category_count: { Food: 2, Travel: 1 },
  top_merchants: [],
}

function renderSummarySection(
  summaryMode: 'current_month' | 'all_time',
  charts: ChartData = chartData,
) {
  return render(
    <I18nProvider>
      <MemoryRouter>
        <SummarySection
          summary={{ label: 'April 2026', stats, charts }}
          currency="INR"
          locale="en-IN"
          summaryMode={summaryMode}
          currentMonthRange={{
            from: '2026-04-01T00:00:00Z',
            to: '2026-04-30T23:59:59Z',
          }}
        />
      </MemoryRouter>
    </I18nProvider>,
  )
}

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

describe('SummarySection', () => {
  it('removes the duplicate bucket donut and combines bank/type into one donut', () => {
    renderSummarySection('current_month')

    expect(screen.queryByText('By bucket')).not.toBeInTheDocument()
    expect(screen.queryByText('By source type')).not.toBeInTheDocument()
    expect(screen.queryByText('By bank')).not.toBeInTheDocument()
    expect(screen.getByText('By Bank and Transaction Type')).toBeInTheDocument()
    expect(screen.queryByText(/Bank:\s*\d+/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Type:\s*\d+/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Total:/)).not.toBeInTheDocument()
  })

  it('shows the category monthly chart for all time without prior comparison copy', () => {
    renderSummarySection('all_time')

    expect(screen.getByText('Spend By Category')).toBeInTheDocument()
    expect(screen.queryByText(/March 2026/)).not.toBeInTheDocument()
    expect(screen.queryByText(/\+33%/)).not.toBeInTheDocument()
  })

  it('keeps uncategorized category spend visible even outside the top five', () => {
    renderSummarySection('current_month', {
      ...chartData,
      by_category_monthly: {
        Food: { current: 600, prior: 500 },
        Travel: { current: 500, prior: 400 },
        Shopping: { current: 400, prior: 300 },
        Utilities: { current: 300, prior: 200 },
        Healthcare: { current: 200, prior: 100 },
        Uncategorized: { current: 50, prior: 0 },
      },
    })

    expect(screen.getByText('Uncategorized')).toBeInTheDocument()
    expect(screen.queryByText('Healthcare')).not.toBeInTheDocument()
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
