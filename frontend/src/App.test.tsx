import { render, screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { server } from './test/server'

vi.mock('@/pages/Dashboard', () => ({
  default: () => {
    throw new Error('Dashboard should not render before first-run setup')
  },
}))

vi.mock('@/pages/setup/Wizard', () => ({
  Wizard: () => <h1>Preferences</h1>,
}))

describe('App first-run routing', () => {
  it('redirects the homepage to setup when required preferences are missing', async () => {
    window.history.pushState({}, '', '/')
    server.use(
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({
          required: true,
          missing: ['base_currency', 'timezone', 'time_format'],
        }),
      ),
    )

    render(<App />)

    expect(await screen.findByText('Preferences')).toBeInTheDocument()
    expect(window.location.pathname).toBe('/setup')
  })

  it('redirects settings to setup when required preferences are missing', async () => {
    window.history.pushState({}, '', '/settings')
    server.use(
      http.get('/api/config/setup-status', () =>
        HttpResponse.json({
          required: true,
          missing: ['base_currency', 'timezone', 'time_format'],
        }),
      ),
    )

    render(<App />)

    expect(await screen.findByText('Preferences')).toBeInTheDocument()
    expect(window.location.pathname).toBe('/setup')
  })
})
