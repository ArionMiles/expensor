import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { CommandPalette, type CommandPaletteAction } from './CommandPalette'
import { I18nProvider } from '@/i18n/I18nProvider'
import { NAVIGATION_TARGETS, type NavigationTarget } from '@/lib/navigation'

function renderCommandPalette({
  targets,
  actions,
  onClose = vi.fn(),
  onNavigate = vi.fn(),
  onAction = vi.fn(),
}: {
  targets: NavigationTarget[]
  actions?: CommandPaletteAction[]
  onClose?: () => void
  onNavigate?: (path: string) => void
  onAction?: (id: string) => void
}) {
  return render(
    <I18nProvider>
      <CommandPalette
        open
        targets={targets}
        actions={actions}
        onClose={onClose}
        onNavigate={onNavigate}
        onAction={onAction}
      />
    </I18nProvider>,
  )
}

describe('CommandPalette', () => {
  it('renders as a named modal dialog', () => {
    renderCommandPalette({
      targets: [
        {
          id: 'rules',
          titleKey: 'nav.rules',
          descriptionKey: 'nav.rules.description',
          path: '/rules',
        },
      ],
    })

    expect(screen.getByRole('dialog', { name: 'Command palette' })).toHaveAttribute(
      'aria-modal',
      'true',
    )
    expect(screen.getByRole('textbox', { name: 'Search commands' })).toHaveAttribute(
      'autocomplete',
      'off',
    )
  })

  it('searches command descriptions', async () => {
    const user = userEvent.setup()

    renderCommandPalette({
      targets: [
        {
          id: 'rules',
          titleKey: 'nav.rules',
          descriptionKey: 'nav.rules.description',
          path: '/rules',
        },
      ],
    })

    await user.type(screen.getByRole('textbox'), 'extraction')

    expect(screen.getByText('Rules')).toBeInTheDocument()
    expect(screen.getByText('Tune email extraction patterns')).toBeInTheDocument()
    expect(screen.queryByText('/rules')).not.toBeInTheDocument()
  })

  it('navigates options by keyboard', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()
    const targets: NavigationTarget[] = [
      {
        id: 'dashboard',
        titleKey: 'nav.dashboard',
        descriptionKey: 'nav.dashboard.description',
        path: '/',
      },
      {
        id: 'transactions',
        titleKey: 'nav.transactions',
        descriptionKey: 'nav.transactions.description',
        path: '/transactions',
      },
    ]

    renderCommandPalette({ targets, onNavigate })

    await user.keyboard('{ArrowDown}{Enter}')

    expect(onNavigate).toHaveBeenCalledWith('/transactions')
  })

  it('closes on escape even when focus is outside the search input', () => {
    const onClose = vi.fn()

    renderCommandPalette({
      targets: [
        {
          id: 'rules',
          titleKey: 'nav.rules',
          descriptionKey: 'nav.rules.description',
          path: '/rules',
        },
      ],
      onClose,
    })

    screen.getByRole('dialog', { name: 'Command palette' }).focus()
    fireEvent.keyDown(document, { key: 'Escape' })

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes once when escape is pressed in the search input', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()

    renderCommandPalette({
      targets: [
        {
          id: 'rules',
          titleKey: 'nav.rules',
          descriptionKey: 'nav.rules.description',
          path: '/rules',
        },
      ],
      onClose,
    })

    await user.keyboard('{Escape}')

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('renders subtitle as breadcrumb text when present', () => {
    renderCommandPalette({
      targets: [
        {
          id: 'settings-sync',
          titleKey: 'nav.settings',
          subtitleKey: 'nav.settings.sync.subtitle',
          descriptionKey: 'nav.settings.sync.description',
          path: '/settings?tab=sync',
        },
      ],
    })

    expect(screen.getByText('Settings / Community')).toBeInTheDocument()
  })

  it('navigates directly to the matching expense group tab', async () => {
    const user = userEvent.setup()
    const onNavigate = vi.fn()

    renderCommandPalette({ targets: NAVIGATION_TARGETS, onNavigate })

    await user.type(screen.getByRole('textbox'), 'labels')
    await user.keyboard('{Enter}')

    expect(screen.getByText('Expense Groups / Labels')).toBeInTheDocument()
    expect(onNavigate).toHaveBeenCalledWith('/expense-groups?tab=labels')
  })

  it('renders quick actions separately from navigation and runs the selected action', async () => {
    const user = userEvent.setup()
    const onAction = vi.fn()
    const onNavigate = vi.fn()

    renderCommandPalette({
      targets: [
        {
          id: 'dashboard',
          titleKey: 'nav.dashboard',
          descriptionKey: 'nav.dashboard.description',
          path: '/',
        },
      ],
      actions: [
        {
          id: 'logout',
          titleKey: 'sidebar.signOut',
          descriptionKey: 'command.actions.signOut.description',
          variant: 'destructive',
        },
      ],
      onAction,
      onNavigate,
    })

    expect(screen.getByText('Navigation')).toBeInTheDocument()
    expect(screen.getByText('Actions')).toBeInTheDocument()
    expect(screen.getByText('End this browser session')).toBeInTheDocument()

    const signOut = screen.getByRole('button', { name: /Sign out/ })
    expect(signOut).toHaveClass('text-destructive')
    await user.click(signOut)

    expect(onAction).toHaveBeenCalledWith('logout')
    expect(onNavigate).not.toHaveBeenCalled()
  })

  it('orders actions before navigation in the result list', () => {
    renderCommandPalette({
      targets: [
        {
          id: 'dashboard',
          titleKey: 'nav.dashboard',
          descriptionKey: 'nav.dashboard.description',
          path: '/',
        },
      ],
      actions: [
        {
          id: 'create-rule',
          titleKey: 'command.actions.createRule',
          descriptionKey: 'command.actions.createRule.description',
        },
      ],
    })

    const labels = screen.getAllByText(/Actions|Navigation/).map((element) => element.textContent)
    expect(labels).toEqual(['Actions', 'Navigation'])
  })

  it('includes command actions in keyboard search results', async () => {
    const user = userEvent.setup()
    const onAction = vi.fn()

    renderCommandPalette({
      targets: [
        {
          id: 'dashboard',
          titleKey: 'nav.dashboard',
          descriptionKey: 'nav.dashboard.description',
          path: '/',
        },
      ],
      actions: [
        {
          id: 'logout',
          titleKey: 'sidebar.signOut',
          descriptionKey: 'command.actions.signOut.description',
          variant: 'destructive',
        },
      ],
      onAction,
    })

    await user.type(screen.getByRole('textbox'), 'logout')
    await user.keyboard('{Enter}')

    expect(onAction).toHaveBeenCalledWith('logout')
  })
})
