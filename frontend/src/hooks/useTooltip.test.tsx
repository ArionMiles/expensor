import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useTooltip } from './useTooltip'

function TooltipHarness({ placement }: { placement?: 'below' | 'right' }) {
  const { handlers, tip } = useTooltip(placement)
  return (
    <>
      <button type="button" {...handlers(`Tooltip ${placement ?? 'below'}`)}>
        trigger
      </button>
      {tip}
    </>
  )
}

describe('useTooltip', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('aligns below tooltip to the right edge when trigger is near viewport edge', async () => {
    const user = userEvent.setup()
    render(<TooltipHarness />)
    const trigger = screen.getByRole('button', { name: 'trigger' })
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      x: 1180,
      y: 120,
      left: 1180,
      right: 1200,
      top: 120,
      bottom: 140,
      width: 20,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)
    vi.stubGlobal('innerWidth', 1210)

    await user.hover(trigger)

    const tooltip = screen.getByText('Tooltip below')
    expect(tooltip.className).toContain('-translate-x-full')
    expect(tooltip).toHaveStyle({ left: '1200px' })
  })

  it('preserves right placement beside the trigger', async () => {
    const user = userEvent.setup()
    render(<TooltipHarness placement="right" />)
    const trigger = screen.getByRole('button', { name: 'trigger' })
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      x: 40,
      y: 100,
      left: 40,
      right: 80,
      top: 100,
      bottom: 120,
      width: 40,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    await user.hover(trigger)

    const tooltip = screen.getByText('Tooltip right')
    expect(tooltip.className).toContain('-translate-y-1/2')
    expect(tooltip).toHaveStyle({ left: '88px', top: '110px' })
  })
})
