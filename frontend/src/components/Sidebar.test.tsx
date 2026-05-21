import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useExtractionDiagnostics, useReaderStatus } from '@/api/queries'
import { renderWithProviders } from '@/test/render'
import { Sidebar } from './Sidebar'

vi.mock('@/api/queries', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/queries')>()
  return {
    ...actual,
    useReaderStatus: vi.fn(),
    useExtractionDiagnostics: vi.fn(),
  }
})

const mockUseReaderStatus = vi.mocked(useReaderStatus)
const mockUseExtractionDiagnostics = vi.mocked(useExtractionDiagnostics)

describe('Sidebar', () => {
  beforeEach(() => {
    mockUseReaderStatus.mockReturnValue({
      data: {
        credentials_uploaded: true,
        authenticated: false,
        config_present: true,
        auth_type: 'oauth',
        ready: false,
      },
      isSuccess: true,
    } as ReturnType<typeof useReaderStatus>)
    mockUseExtractionDiagnostics.mockReturnValue({
      data: [{ id: 'diag-1' }, { id: 'diag-2' }, { id: 'diag-3' }],
    } as ReturnType<typeof useExtractionDiagnostics>)
  })

  it('shows the full Expensor logo in the expanded sidebar', () => {
    const { container } = renderWithProviders(
      <Sidebar collapsed={false} onToggle={() => undefined} />,
    )

    expect(screen.getByLabelText('Expensor home')).toHaveAttribute('href', '/')
    expect(container.querySelector('img[src="/brand/expensor-wordmark.svg"]')).toBeInTheDocument()
    expect(screen.getByLabelText('Expensor home')).toContainElement(
      container.querySelector('img[src="/brand/expensor-wallet.svg"]'),
    )
  })

  it('uses the wallet icon for the collapsed sidebar toggle', () => {
    const onToggle = vi.fn()
    const { container } = renderWithProviders(<Sidebar collapsed={true} onToggle={onToggle} />)

    const button = screen.getByRole('button', { name: /Open sidebar \((⌘|Ctrl) \+ \.\)/ })
    expect(button).toContainElement(
      container.querySelector('img[src="/brand/expensor-wallet.svg"]'),
    )
  })

  it('shows the sidebar shortcut on the expanded sidebar toggle', () => {
    renderWithProviders(<Sidebar collapsed={false} onToggle={() => undefined} />)

    expect(
      screen.getByRole('button', { name: /Close sidebar \((⌘|Ctrl) \+ \.\)/ }),
    ).toBeInTheDocument()
  })

  it('shows a subtle setup attention dot without changing the onboarding icon', () => {
    renderWithProviders(<Sidebar collapsed={false} onToggle={() => undefined} />)

    expect(screen.getByTestId('setup-attention-dot')).toBeInTheDocument()
    expect(screen.getByText('Onboarding').closest('a')).toContainElement(
      screen.getByTestId('setup-attention-dot'),
    )
  })

  it('shows the setup attention dot when collapsed', () => {
    renderWithProviders(<Sidebar collapsed={true} onToggle={() => undefined} />)

    expect(screen.getByTestId('setup-attention-dot')).toBeInTheDocument()
  })

  it('shows the open diagnostics count in expanded mode', () => {
    renderWithProviders(<Sidebar collapsed={false} onToggle={() => undefined} />)

    const badge = screen.getByTestId('diagnostics-count-badge')
    expect(badge).toHaveTextContent('3')
    expect(badge).toHaveAttribute('aria-label', '3 open diagnostics')
  })

  it('centers the collapsed diagnostics count without clipping it', () => {
    renderWithProviders(<Sidebar collapsed={true} onToggle={() => undefined} />)

    const badge = screen.getByTestId('diagnostics-count-badge')
    expect(badge).toHaveClass('items-center')
    expect(badge).toHaveClass('justify-center')
    expect(screen.getByTestId('nav-link-diagnostics')).toHaveClass('overflow-visible')
  })
})
