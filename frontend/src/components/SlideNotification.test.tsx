import { act, fireEvent, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SlideNotification } from './SlideNotification'
import { renderWithProviders } from '@/test/render'

describe('SlideNotification', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('calls onAction when an action button is clicked', () => {
    const onAction = vi.fn()

    renderWithProviders(
      <SlideNotification
        onAction={onAction}
        actions={[
          { label: 'Dismiss', value: false },
          { label: 'Apply', value: true, primary: true },
        ]}
      >
        Apply label to merchant?
      </SlideNotification>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(onAction).toHaveBeenCalledWith(true)
  })

  it('triggers the default action when the timeout elapses', () => {
    const onAction = vi.fn()

    renderWithProviders(
      <SlideNotification
        duration={1000}
        onAction={onAction}
        actions={[
          { label: 'Apply to all', value: true, primary: true },
          { label: 'Just this one', value: false },
        ]}
      >
        Apply label to merchant?
      </SlideNotification>,
    )

    act(() => {
      vi.advanceTimersByTime(1300)
    })

    expect(onAction).toHaveBeenCalledWith(false)
  })
})
