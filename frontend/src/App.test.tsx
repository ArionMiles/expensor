import { render, screen, waitFor } from '@testing-library/react'
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
    expect(window.location.pathname).toBe('/login')
    await user.type(screen.getByLabelText('Email'), 'admin@example.com')
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(window.location.pathname).toBe('/transactions'))
  }, 15_000)

  it('sends fresh instances to first-admin bootstrap before private routes', async () => {
    window.history.pushState({}, '', '/')
    server.use(http.get('/api/bootstrap', () => HttpResponse.json({ required: true })))

    render(<App />)

    expect(
      await screen.findByRole('heading', { name: 'Initialize this Expensor instance' }, routeWait),
    ).toBeInTheDocument()
    expect(screen.getByText('Protected local expense workspace')).toBeInTheDocument()
    expect(screen.getByText('Local data stays on this server.')).toBeInTheDocument()
    expect(screen.getByText('First admin required')).toBeInTheDocument()
    expect(screen.queryByText('2.4k')).not.toBeInTheDocument()
    expect(window.location.pathname).toBe('/bootstrap')
  }, 15_000)

  it('creates the first admin and continues to setup', async () => {
    const user = userEvent.setup()
    window.history.pushState({}, '', '/bootstrap')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: true })),
      http.post('/api/bootstrap', async ({ request }) => {
        const body = (await request.json()) as { email?: string; display_name?: string }
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

    await user.type(await screen.findByLabelText('Email', {}, routeWait), 'admin@example.com')
    await user.type(screen.getByLabelText('Display name'), 'Admin')
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: 'Create admin' }))

    await waitFor(() => expect(window.location.pathname).toBe('/setup'))
  }, 15_000)

  it('completes invited account setup from a setup token', async () => {
    const user = userEvent.setup()
    window.history.pushState({}, '', '/account-setup?token=expensor_setup_test')
    server.use(
      http.get('/api/bootstrap', () => HttpResponse.json({ required: false })),
      http.post('/api/account-setup', async ({ request }) => {
        const body = (await request.json()) as { token?: string; password?: string }
        if (
          body.token !== 'expensor_setup_test' ||
          body.password !== 'correct horse battery staple'
        ) {
          return HttpResponse.json({ error: 'invalid or expired setup token' }, { status: 401 })
        }
        return HttpResponse.json(
          {
            user_id: 'user-b',
            tenant_id: 'user-b',
            email: 'b@example.com',
            display_name: 'B',
            role: 'user',
            avatar_key: 'default',
          },
          { status: 201 },
        )
      }),
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

    expect(
      await screen.findByRole('heading', { name: 'Set password' }, routeWait),
    ).toBeInTheDocument()
    await user.type(screen.getByLabelText('Password'), 'correct horse battery staple')
    await user.click(screen.getByRole('button', { name: 'Finish setup' }))

    await waitFor(() => expect(window.location.pathname).toBe('/'))
  }, 15_000)
})
