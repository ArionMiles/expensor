import { fireEvent, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DateRangePicker } from './DateRangePicker'
import { renderWithProviders } from '@/test/render'

function ControlledPicker({ initialValue = {} }: { initialValue?: { from?: Date; to?: Date } }) {
  const [value, setValue] = useState(initialValue)
  return <DateRangePicker value={value} onChange={setValue} />
}

function getSelectableDayButton() {
  return screen
    .getAllByRole('button')
    .find(
      (button) =>
        !!button.getAttribute('aria-label') &&
        /^\d+$/.test(button.textContent ?? '') &&
        button.getAttribute('aria-label')?.includes(','),
    )
}

describe('DateRangePicker', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('opens and closes the picker', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    expect(document.getElementById('date-picker-portal')).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })

    expect(document.getElementById('date-picker-portal')).not.toBeInTheDocument()
  })

  it('returns focus to the trigger when closed with escape', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    const trigger = screen.getByRole('button', { name: 'Select date range' })
    fireEvent.click(trigger)
    fireEvent.keyDown(document, { key: 'Escape' })

    expect(trigger).toHaveFocus()
  })

  it('clears the current range', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-10T00:00:00Z'),
          to: new Date('2026-04-12T23:59:59Z'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Clear date range' }))

    expect(onChange).toHaveBeenCalledWith({})
  })

  it('keeps the picker open when clearing from the footer', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-10T00:00:00'),
          to: new Date('2026-04-12T23:59:59'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))

    expect(onChange).toHaveBeenCalledWith({})
    expect(document.getElementById('date-picker-portal')).toBeInTheDocument()
  })

  it('keeps apply disabled until a start date exists', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    expect(screen.getByRole('button', { name: 'Apply' })).toBeDisabled()
  })

  it('keeps clear disabled until a start date exists', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    expect(screen.getByRole('button', { name: 'Clear' })).toBeDisabled()
  })

  it('marks the selected preset as pressed', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    fireEvent.click(screen.getByRole('button', { name: '7D' }))

    expect(screen.getByRole('button', { name: '7D' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Custom' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('places custom at the end and does not press it by default for an empty range', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    const presets = ['7D', '1M', 'This month', 'Last month', 'FY', 'Custom']
    const buttons = screen
      .getAllByRole('button')
      .filter((button) => presets.includes(button.textContent ?? ''))

    expect(buttons.map((button) => button.textContent)).toEqual(presets)
    expect(screen.getByRole('button', { name: 'Custom' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('uses compact single-letter weekday labels', () => {
    renderWithProviders(<DateRangePicker value={{}} onChange={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    expect(screen.getAllByText('S')).toHaveLength(4)
    expect(screen.queryByText('Su')).not.toBeInTheDocument()
  })

  it('uses custom time controls instead of native selects', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-01T00:00:00'),
          to: new Date('2026-04-01T23:59:59'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Increase start time by 15 minutes' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    const nextRange = onChange.mock.calls[0][0]
    expect(nextRange.from.getMinutes()).toBe(15)
  })

  it('lets users type directly into highlighted hour and minute segments', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-01T00:00:00'),
          to: new Date('2026-04-01T23:59:59'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    const startHour = screen.getByLabelText('Start hour')
    const startMinute = screen.getByLabelText('Start minute')
    fireEvent.change(startHour, { target: { value: '09' } })
    fireEvent.change(startMinute, { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    const nextRange = onChange.mock.calls[0][0]
    expect(nextRange.from.getHours()).toBe(9)
    expect(nextRange.from.getMinutes()).toBe(30)
  })

  it('keeps one-digit time edits while the segment is focused', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-01T00:00:00'),
          to: new Date('2026-04-01T23:59:59'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    const startHour = screen.getByLabelText('Start hour')
    fireEvent.focus(startHour)
    fireEvent.change(startHour, { target: { value: '9' } })

    expect(startHour).toHaveValue('9')
  })

  it('increments focused time segments with arrow keys', () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-01T09:30:00'),
          to: new Date('2026-04-01T23:59:59'),
        }}
        onChange={onChange}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    const startHour = screen.getByLabelText('Start hour')
    fireEvent.keyDown(startHour, { key: 'ArrowUp' })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    const nextRange = onChange.mock.calls[0][0]
    expect(nextRange.from.getHours()).toBe(10)
  })

  it('tabs between time segments without focusing increment buttons', () => {
    renderWithProviders(
      <DateRangePicker
        value={{
          from: new Date('2026-04-01T09:30:00'),
          to: new Date('2026-04-02T18:45:59'),
        }}
        onChange={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))

    const startHour = screen.getByLabelText('Start hour')
    const startMinute = screen.getByLabelText('Start minute')
    const endHour = screen.getByLabelText('End hour')
    startHour.focus()

    fireEvent.keyDown(startHour, { key: 'Tab' })
    expect(startMinute).toHaveFocus()

    fireEvent.keyDown(startMinute, { key: 'Tab' })
    expect(endHour).toHaveFocus()
  })

  it('applies the current fiscal year from the preset strip', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 4, 17, 12))
    const onChange = vi.fn()

    renderWithProviders(<DateRangePicker value={{}} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    fireEvent.click(screen.getByRole('button', { name: 'FY' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    const nextRange = onChange.mock.calls[0][0]
    expect(nextRange.from).toEqual(new Date(2026, 3, 1, 0, 0, 0, 0))
    expect(nextRange.to).toEqual(new Date(2027, 2, 31, 23, 59, 59, 999))
  })

  it('does not apply a one-sided custom date selection', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 6, 9, 12))
    const onChange = vi.fn()

    renderWithProviders(<DateRangePicker value={{}} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    fireEvent.click(screen.getByRole('button', { name: 'Monday, July 6th, 2026' }))

    expect(
      screen.getByText('Start date selected. Click an end date, or click it again for one day.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Apply' })).toBeDisabled()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('selects a single-day range when the same date is clicked twice', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 6, 9, 12))
    const onChange = vi.fn()

    renderWithProviders(<DateRangePicker value={{}} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Select date range' }))
    const day = screen.getByRole('button', { name: 'Monday, July 6th, 2026' })
    fireEvent.click(day)
    fireEvent.click(day)
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(onChange).toHaveBeenCalledWith({
      from: new Date(2026, 6, 6, 0, 0, 0, 0),
      to: new Date(2026, 6, 6, 23, 59, 59, 999),
    })
  })

  it('updates the rendered range text after applying a selection', () => {
    renderWithProviders(<ControlledPicker />)

    const trigger = screen.getByRole('button', { name: 'Select date range' })
    expect(within(trigger).getByText('All')).toBeInTheDocument()

    fireEvent.click(trigger)

    const dayButton = getSelectableDayButton()
    expect(dayButton).toBeDefined()

    fireEvent.click(dayButton!)
    fireEvent.click(dayButton!)
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(within(trigger).queryByText('All')).not.toBeInTheDocument()
  })
})
