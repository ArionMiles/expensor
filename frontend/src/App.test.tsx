import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App, queryClient } from './App'
import { server } from './test/server'

vi.mock('@/pages/Dashboard', () => ({
  default: () => <h1>Dashboard</h1>,
}))

vi.mock('@/pages/setup/Wizard', () => ({
  Wizard: () => <h1>Preferences</h1>,
}))

afterEach(() => {
  queryClient.clear()
})

const routeWait = { timeout: 10_000 }

describe('App first-run routing', () => {
  it('redirects the homepage to setup when required preferences are missing', async () => {
    window.history.pushState({}, '', '/')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({
          required: true,
          missing: ['base_currency', 'timezone', 'time_format'],
        }),
      ),
    )

    render(<App />)

    expect(await screen.findByText('Preferences', {}, routeWait)).toBeInTheDocument()
    expect(window.location.pathname).toBe('/setup')
  }, 15_000)

  it('redirects settings to setup when required preferences are missing', async () => {
    window.history.pushState({}, '', '/settings')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({
          required: true,
          missing: ['base_currency', 'timezone', 'time_format'],
        }),
      ),
    )

    render(<App />)

    expect(await screen.findByText('Preferences', {}, routeWait)).toBeInTheDocument()
    expect(window.location.pathname).toBe('/setup')
  }, 15_000)
})

describe('App auth routing', () => {
  it('redirects anonymous users to login and returns to the requested page after sign in', async () => {
    const user = userEvent.setup()
    let loggedIn = false
    window.history.pushState({}, '', '/transactions')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () => {
        if (!loggedIn)
          return HttpResponse.json({ error: 'authentication required' }, { status: 401 })
        return HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        })
      }),
      http.post('/api/session', async ({ request }) => {
        const body = (await request.json()) as { email?: string; password?: string }
        if (
          body.email !== 'admin@example.com' ||
          body.password !== 'correct horse battery staple'
        ) {
          return HttpResponse.json({ error: 'invalid email or password' }, { status: 401 })
        }
        loggedIn = true
        return HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        })
      }),
    )

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Sign in' }, routeWait)).toBeInTheDocument()
    expect(screen.getByTestId('auth-centered-surface')).toBeInTheDocument()
    expect(screen.getByTestId('auth-form-logo')).toBeInTheDocument()
    expect(screen.queryByText('Instance access')).not.toBeInTheDocument()
    expect(screen.queryByText('Sign in to Expensor')).not.toBeInTheDocument()
    expect(screen.queryByText('First run setup')).not.toBeInTheDocument()
    expect(screen.queryByText('Set up Expensor')).not.toBeInTheDocument()
    expect(window.location.pathname).toBe('/login')
    await user.type(screen.getByLabelText('Email'), 'admin@example.com')
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(window.location.pathname).toBe('/transactions'))
  }, 15_000)

  it('signs out from the command palette action', async () => {
    const user = userEvent.setup()
    let logoutRequests = 0
    window.history.pushState({}, '', '/')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({ required: false, missing: [] }),
      ),
      http.delete('/api/session', () => {
        logoutRequests += 1
        return new HttpResponse(null, { status: 204 })
      }),
    )

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dashboard' }, routeWait)).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument()
    await user.type(screen.getByRole('textbox', { name: 'Search commands' }), 'logout')
    await user.keyboard('{Enter}')

    await waitFor(() => expect(logoutRequests).toBe(1))
    await waitFor(() => expect(window.location.pathname).toBe('/login'))
  }, 15_000)

  it('offers admin command palette actions and runs daemon actions', async () => {
    const user = userEvent.setup()
    let rescanRequests = 0
    let clearCheckpointRequests = 0
    const syncSettingPatches: Array<Record<string, unknown>> = []
    const statusIndicatorEvents: boolean[] = []
    window.addEventListener('expensor:scanningStatusBreathingChanged', ((event: CustomEvent) => {
      statusIndicatorEvents.push(Boolean(event.detail?.enabled))
    }) as EventListener)
    window.history.pushState({}, '', '/')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'default',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({ required: false, missing: [] }),
      ),
      http.get('/api/scanning/settings', () =>
        HttpResponse.json({ active_reader: 'gmail', enabled: true }),
      ),
      http.get('/api/config/sync/settings', () =>
        HttpResponse.json({ automatic_sync_enabled: true }),
      ),
      http.patch('/api/config/sync/settings', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        syncSettingPatches.push(body)
        return HttpResponse.json(body)
      }),
      http.post('/api/scanning/rescans', async ({ request }) => {
        const body = (await request.json()) as { reader?: string }
        if (body.reader === 'gmail') rescanRequests += 1
        return HttpResponse.json({ status: 'rescanning' })
      }),
      http.delete('/api/config/providers/:reader/checkpoint', ({ params }) => {
        if (params.reader === 'gmail') clearCheckpointRequests += 1
        return new HttpResponse(null, { status: 204 })
      }),
    )

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dashboard' }, routeWait)).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    const palette = await screen.findByRole('dialog', { name: 'Command palette' })
    expect(within(palette).getByText('Actions')).toBeInTheDocument()
    expect(within(palette).getByText('Navigation')).toBeInTheDocument()
    expect(within(palette).getByRole('button', { name: /Create new rule/ })).toBeInTheDocument()
    expect(
      within(palette).getByRole('button', { name: /Create rule from emails/ }),
    ).toBeInTheDocument()
    expect(within(palette).getByRole('button', { name: /Create access token/ })).toBeInTheDocument()
    expect(within(palette).getByRole('button', { name: /Create new user/ })).toBeInTheDocument()
    expect(
      within(palette).getByRole('button', { name: /Toggle status indicator/ }),
    ).toBeInTheDocument()
    expect(
      within(palette).getByRole('button', { name: /Toggle automatic community sync/ }),
    ).toBeInTheDocument()
    expect(within(palette).getByRole('button', { name: /Force rescan/ })).toBeEnabled()
    expect(within(palette).getByRole('button', { name: /Clear checkpoint/ })).toBeEnabled()

    await user.type(within(palette).getByRole('textbox', { name: 'Search commands' }), 'rescan')
    await user.keyboard('{Enter}')
    await waitFor(() => expect(rescanRequests).toBe(1))

    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    const checkpointPalette = await screen.findByRole('dialog', { name: 'Command palette' })
    await user.type(
      within(checkpointPalette).getByRole('textbox', { name: 'Search commands' }),
      'checkpoint',
    )
    await user.keyboard('{Enter}')
    await waitFor(() => expect(clearCheckpointRequests).toBe(1))

    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    const statusPalette = await screen.findByRole('dialog', { name: 'Command palette' })
    await user.type(
      within(statusPalette).getByRole('textbox', { name: 'Search commands' }),
      'status indicator',
    )
    await user.keyboard('{Enter}')
    expect(statusIndicatorEvents).toEqual([false])

    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    const communityPalette = await screen.findByRole('dialog', { name: 'Command palette' })
    await user.type(
      within(communityPalette).getByRole('textbox', { name: 'Search commands' }),
      'automatic community sync',
    )
    await user.keyboard('{Enter}')
    await waitFor(() => expect(syncSettingPatches).toEqual([{ automatic_sync_enabled: false }]))
  }, 15_000)

  it('hides admin-only command palette actions from regular users', async () => {
    window.history.pushState({}, '', '/')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'user-b',
          tenant_id: 'user-b',
          email: 'b@example.com',
          display_name: 'B',
          role: 'user',
          avatar_key: 'default',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({ required: false, missing: [] }),
      ),
    )

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dashboard' }, routeWait)).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })

    const palette = await screen.findByRole('dialog', { name: 'Command palette' })
    expect(
      within(palette).queryByRole('button', { name: /Create new user/ }),
    ).not.toBeInTheDocument()
    expect(within(palette).getByRole('button', { name: /Create access token/ })).toBeInTheDocument()
  }, 15_000)

  it('shows custom login email validation instead of browser-native validation', async () => {
    const user = userEvent.setup()
    let loginAttempts = 0
    window.history.pushState({}, '', '/login')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.post('/api/session', () => {
        loginAttempts += 1
        return HttpResponse.json({ error: 'invalid email or password' }, { status: 401 })
      }),
    )

    render(<App />)

    const emailInput = await screen.findByLabelText('Email', {}, routeWait)
    expect(emailInput).toHaveAttribute('type', 'text')
    expect(emailInput).toHaveAttribute('inputmode', 'email')
    expect(screen.getByTestId('login-email-feedback')).toHaveClass('min-h-5')
    await user.type(emailInput, 'sdsdsd')
    await user.tab()

    expect(screen.getByText('Enter a valid email address.')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Password'), 'password')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(loginAttempts).toBe(0)
  }, 15_000)

  it('sends fresh instances to first-admin bootstrap before private routes', async () => {
    window.history.pushState({}, '', '/')
    server.use(http.get('/api/bootstrap', () => HttpResponse.json({ required: true })))

    render(<App />)

    expect(
      await screen.findByRole('heading', { name: 'Initialize this Expensor instance' }, routeWait),
    ).toBeInTheDocument()
    expect(screen.getByText('First run setup')).toBeInTheDocument()
    expect(screen.getByText('Set up Expensor')).toBeInTheDocument()
    expect(
      screen.getByText('Create the first admin account before connecting Gmail or Thunderbird.'),
    ).toBeInTheDocument()
    expect(
      screen.getByText('Transactions, rules, and reader credentials stay on this server.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Protected local expense workspace')).not.toBeInTheDocument()
    expect(screen.queryByText('Workspace lock')).not.toBeInTheDocument()
    expect(screen.queryByText('Account gate')).not.toBeInTheDocument()
    expect(screen.queryByText('Local data')).not.toBeInTheDocument()
    expect(screen.queryByText('2.4k')).not.toBeInTheDocument()
    expect(screen.queryByText('Token access')).not.toBeInTheDocument()
    expect(window.location.pathname).toBe('/bootstrap')
  }, 15_000)

  it('creates the first admin and continues to setup', async () => {
    const user = userEvent.setup()
    const bootstrapRequests: Array<{
      email?: string
      display_name?: string
      password?: string
      avatar_key?: string
    }> = []
    window.history.pushState({}, '', '/bootstrap')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: true })),
      http.post('/api/bootstrap', async ({ request }) => {
        const body = (await request.json()) as {
          email?: string
          display_name?: string
          password?: string
          avatar_key?: string
        }
        bootstrapRequests.push(body)
        return HttpResponse.json(
          {
            user_id: 'admin',
            tenant_id: 'admin',
            email: body.email,
            display_name: body.display_name,
            role: 'admin',
            avatar_key: 'ledger',
          },
          { status: 201 },
        )
      }),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'admin',
          tenant_id: 'admin',
          email: 'admin@example.com',
          display_name: 'Admin',
          role: 'admin',
          avatar_key: 'ledger',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({ required: true, missing: ['base_currency'] }),
      ),
    )

    render(<App />)

    const emailInput = await screen.findByLabelText('Email', {}, routeWait)
    expect(screen.getByTestId('email-floating-label')).toHaveClass('top-1/2')
    await user.click(emailInput)
    expect(screen.getByTestId('email-floating-label')).toHaveClass('top-1.5')
    await user.type(emailInput, 'admin@example.com')
    await user.type(screen.getByLabelText('Display name'), 'Admin')
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    expect(screen.getByText('Password strength: Good')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId('password-strength-hint')).toHaveTextContent(
        'Include an uppercase character.',
      ),
    )
    expect(screen.queryByText('Include a number.')).not.toBeInTheDocument()
    expect(screen.queryByText('Include a symbol.')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Change avatar' })).not.toBeInTheDocument()
    expect(screen.queryByText('Avatar: Default')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Ledger avatar' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Default avatar' }))
    await user.click(screen.getByRole('button', { name: 'Ledger avatar' }))
    expect(screen.queryByRole('button', { name: 'Ledger avatar' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Wallet avatar' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Initialize instance' }))

    await waitFor(() => expect(window.location.pathname).toBe('/setup'))
    expect(bootstrapRequests).toEqual([
      {
        email: 'admin@example.com',
        display_name: 'Admin',
        password: 'correct horse battery staple',
        avatar_key: 'ledger',
      },
    ])
  }, 15_000)

  it('shows local validation before first-admin bootstrap submission', async () => {
    const user = userEvent.setup()
    window.history.pushState({}, '', '/bootstrap')
    server.use(http.get('/api/bootstrap', () => HttpResponse.json({ required: true })))

    render(<App />)

    await screen.findByLabelText('Email', {}, routeWait)
    expect(screen.getByTestId('email-feedback')).toHaveClass('min-h-5')
    expect(screen.getByTestId('password-entry-group')).toHaveClass('space-y-2')
    expect(screen.getByTestId('password-strength-feedback')).toHaveClass('min-h-16')
    expect(screen.getByTestId('password-strength-track')).toHaveClass('opacity-0')
    expect(screen.queryByText(/Password strength:/)).not.toBeInTheDocument()
    expect(screen.getByTestId('password-strength-hint')).toHaveTextContent('')
    expect(screen.getByRole('button', { name: 'Default avatar' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(screen.queryByRole('button', { name: 'Ledger avatar' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Wallet avatar' })).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('Email'), 'sas')
    await user.tab()
    await user.type(screen.getByLabelText('Display name'), 'Admin')
    await user.type(screen.getByLabelText('Password'), 'short')

    expect(screen.getByText('Enter a valid email address.')).toBeInTheDocument()
    expect(screen.getByText('Password strength: Weak')).toBeInTheDocument()
    expect(screen.getByTestId('password-strength-track')).toHaveClass('opacity-100')
    await waitFor(() =>
      expect(screen.getByTestId('password-strength-hint')).toHaveTextContent(
        'Include an uppercase character.',
      ),
    )
    expect(screen.queryByText('Use at least 12 characters.')).not.toBeInTheDocument()
    expect(screen.queryByText('Include a number.')).not.toBeInTheDocument()
    expect(screen.queryByText('Include a symbol.')).not.toBeInTheDocument()
    expect(screen.getByTestId('password-strength-meter')).toHaveClass('bg-warning')
    expect(screen.getByRole('button', { name: 'Initialize instance' })).toBeDisabled()

    await user.clear(screen.getByLabelText('Password'))
    expect(screen.getByTestId('password-strength-track')).toHaveClass('opacity-0')
    expect(screen.getByTestId('password-strength-hint')).toHaveTextContent('')
    await user.type(screen.getByLabelText('Password'), 'Correct horse battery staple!')
    expect(screen.getByText('Password strength: Good')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId('password-strength-hint')).toHaveTextContent('Include a number.'),
    )

    await user.clear(screen.getByLabelText('Password'))
    await user.type(screen.getByLabelText('Password'), 'Correct horse battery staple 1!')
    expect(screen.getByText('Password strength: Strong')).toBeInTheDocument()
    expect(screen.getByTestId('password-strength-meter')).toHaveClass('bg-success')
    await waitFor(() => expect(screen.getByTestId('password-strength-hint')).toHaveTextContent(''))
  }, 15_000)

  it('opens avatar options in order and collapses on outside focus', async () => {
    const user = userEvent.setup()
    window.history.pushState({}, '', '/bootstrap')
    server.use(http.get('/api/bootstrap', () => HttpResponse.json({ required: true })))

    render(<App />)

    await screen.findByRole('button', { name: 'Default avatar' }, routeWait)
    expect(screen.getByTestId('avatar-picker-surface')).not.toHaveClass('rounded-lg')
    expect(screen.getByTestId('avatar-picker-surface')).not.toHaveClass('border')
    await user.click(screen.getByRole('button', { name: 'Default avatar' }))
    expect(screen.getByTestId('avatar-picker-surface')).toHaveClass('opacity-100')
    expect(
      screen
        .getAllByRole('button', { name: /avatar$/ })
        .map((button) => button.getAttribute('data-testid')),
    ).toEqual(['avatar-option-default', 'avatar-option-ledger', 'avatar-option-wallet'])
    expect(screen.getByTestId('avatar-option-default')).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByTestId('avatar-option-ledger')).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByTestId('avatar-option-wallet')).toHaveAttribute('aria-pressed', 'false')

    await user.click(screen.getByRole('button', { name: 'Wallet avatar' }))
    expect(screen.getByTestId('avatar-option-wallet')).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByTestId('avatar-option-ledger')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Wallet avatar' }))
    expect(screen.getByTestId('avatar-option-default')).toBeInTheDocument()
    expect(screen.getByTestId('avatar-option-ledger')).toBeInTheDocument()
    await user.click(screen.getByLabelText('Email'))
    expect(screen.queryByTestId('avatar-option-default')).not.toBeInTheDocument()
    expect(screen.getByTestId('avatar-option-wallet')).toHaveAttribute('aria-pressed', 'true')
  }, 15_000)

  it('completes invited account setup from a setup token', async () => {
    const user = userEvent.setup()
    window.history.pushState({}, '', '/account-setup?token=expensor_setup_test')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.get('/api/account-setup', ({ request }) => {
        const token = new URL(request.url).searchParams.get('token')
        if (token !== 'expensor_setup_test') {
          return HttpResponse.json({ error: 'invalid or expired setup token' }, { status: 401 })
        }
        return HttpResponse.json({
          email: 'b@example.com',
          avatar_key: 'default',
        })
      }),
      http.post('/api/account-setup', async ({ request }) => {
        const body = (await request.json()) as {
          token?: string
          display_name?: string
          password?: string
          avatar_key?: string
        }
        if (
          body.token !== 'expensor_setup_test' ||
          body.display_name !== 'B Updated' ||
          body.avatar_key !== 'wallet' ||
          body.password !== 'correct horse battery staple'
        ) {
          return HttpResponse.json({ error: 'invalid or expired setup token' }, { status: 401 })
        }
        return HttpResponse.json(
          {
            user_id: 'user-b',
            tenant_id: 'user-b',
            email: 'b@example.com',
            display_name: 'B Updated',
            role: 'user',
            avatar_key: 'wallet',
          },
          { status: 201 },
        )
      }),
      http.get('/api/session', () =>
        HttpResponse.json({
          user_id: 'user-b',
          tenant_id: 'user-b',
          email: 'b@example.com',
          display_name: 'B Updated',
          role: 'user',
          avatar_key: 'wallet',
        }),
      ),
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({ required: false, missing: [] }),
      ),
    )

    render(<App />)

    expect(
      await screen.findByRole('heading', { name: 'Set up your account' }, routeWait),
    ).toBeInTheDocument()
    expect(screen.getByTestId('auth-centered-surface')).toBeInTheDocument()
    expect(screen.getByTestId('auth-form-logo')).toBeInTheDocument()
    expect(
      screen.queryByText('Choose how your account appears before setting your password.'),
    ).not.toBeInTheDocument()
    expect(screen.queryByText('First run setup')).not.toBeInTheDocument()
    expect(screen.queryByText('Set up Expensor')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.getByLabelText('Email')).toHaveValue('b@example.com'))
    expect(screen.getByLabelText('Email')).toBeDisabled()
    await user.type(screen.getByLabelText('Display name'), 'B Updated')
    await user.click(screen.getByRole('button', { name: 'Default avatar' }))
    await user.click(screen.getByRole('button', { name: 'Wallet avatar' }))
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: 'Finish setup' }))

    await waitFor(() => expect(window.location.pathname).toBe('/'))
  }, 15_000)
})
