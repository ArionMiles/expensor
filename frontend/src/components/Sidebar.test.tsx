import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { Sidebar } from './Sidebar'

describe('Sidebar', () => {
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
})
