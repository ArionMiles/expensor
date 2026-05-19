import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { DaemonStatusBar } from './DaemonStatusBar'

const useStatus = vi.hoisted(() => vi.fn())

vi.mock('@/api/queries', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/queries')>()
  return {
    ...actual,
    useStatus: () => useStatus(),
  }
})

describe('DaemonStatusBar', () => {
  beforeEach(() => {
    useStatus.mockReturnValue({
      data: { daemon: { running: true }, stats: { base_currency: 'USD' } },
      isLoading: false,
      error: null,
    })
  })

  it('shows the command palette shortcut on the right side of the status bar', () => {
    renderWithProviders(<DaemonStatusBar />)

    expect(screen.getByText('daemon running')).toBeInTheDocument()
    expect(screen.getByText('Command palette')).toBeInTheDocument()
    expect(screen.getByText(/(⌘|Ctrl) \+ K/)).toBeInTheDocument()
  })
})
