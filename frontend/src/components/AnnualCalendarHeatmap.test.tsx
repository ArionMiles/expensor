import { render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AnnualCalendarHeatmap } from './AnnualCalendarHeatmap'

vi.mock('@/api/queries', () => ({
  useAnnualHeatmapData: vi.fn((year: number) => ({
    data: {
      year,
      buckets: [],
    },
    isLoading: false,
  })),
}))

describe('AnnualCalendarHeatmap', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the current year as a rolling year ending today', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 4, 17, 12))

    const { container } = render(
      <AnnualCalendarHeatmap year={2026} metric="count" currency="INR" />,
    )

    const renderedDates = Array.from(container.querySelectorAll('rect[data-date]')).map((cell) =>
      cell.getAttribute('data-date'),
    )

    expect(renderedDates[0]).toBe('2025-05-18')
    expect(renderedDates[renderedDates.length - 1]).toBe('2026-05-17')
    expect(renderedDates).not.toContain('2025-01-01')
  })
})
