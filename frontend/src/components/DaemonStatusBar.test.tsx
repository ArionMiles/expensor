import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { DaemonStatusBar } from './DaemonStatusBar'

const useScanningStatus = vi.hoisted(() => vi.fn())

vi.mock('@/api/queries', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/queries')>()
  return {
    ...actual,
    useScanningStatus: () => useScanningStatus(),
  }
})

describe('DaemonStatusBar', () => {
  beforeEach(() => {
    useScanningStatus.mockReturnValue({
      data: {
        active_reader: 'gmail',
        enabled: true,
        state: 'running',
        retry_count: 0,
        updated_at: new Date(Date.UTC(2026, 0, 1)).toISOString(),
      },
      isLoading: false,
      error: null,
    })
  })

  it('shows the command palette shortcut on the right side of the status bar', () => {
    renderWithProviders(<DaemonStatusBar />)

    expect(screen.getByText('scanning active')).toBeInTheDocument()
    expect(screen.getByText('Command palette')).toBeInTheDocument()
    expect(screen.getByText(/(⌘|Ctrl) \+ K/)).toBeInTheDocument()
  })
})
