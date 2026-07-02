import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router-dom'
import Settings from './Settings'
import { renderWithProviders } from '@/test/render'
import { server } from '@/test/server'

vi.mock('@/lib/timezone', () => ({
  getBrowserTimezone: () => 'Asia/Calcutta',
  getTimezoneOptions: () => ['Asia/Calcutta'],
  normalizeTimezone: (timezone: string | undefined | null) =>
    timezone === 'Asia/Kolkata' ? 'Asia/Calcutta' : (timezone?.trim() ?? ''),
}))

function LocationProbe() {
  const location = useLocation()
  return <output data-testid="location">{`${location.pathname}${location.search}`}</output>
}

function renderSettings(route = '/settings') {
  return renderWithProviders(
    <>
      <Settings />
      <LocationProbe />
    </>,
    { route },
  )
}

describe('Settings', () => {
  it('uses the tab query param to control the visible section', async () => {
    renderSettings('/settings?tab=sync')

    expect(await screen.findByText('Sync status')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/settings?tab=sync')
  })

  it('updates the tab query param when the user clicks a tab', async () => {
    const user = userEvent.setup()

    renderSettings('/settings')

    await user.click(screen.getByRole('button', { name: 'Daemon' }))

    expect(await screen.findByText('Scan interval')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/settings?tab=daemon')
  })

  it('falls back to the general tab for an invalid tab value', async () => {
    renderSettings('/settings?tab=unknown')

    expect(await screen.findByText('Base currency')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/settings?tab=unknown')
  })

  it('starts the timezone setting from the browser timezone when the config is unset', async () => {
    server.use(
      http.get('/api/config/preferences', () =>
        HttpResponse.json({
          base_currency: 'USD',
          scan_interval: 60,
          lookback_days: 180,
          timezone: '',
          time_format: 'HH:mm',
        }),
      ),
    )

    renderSettings('/settings')

    expect(await screen.findByText('Timezone')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /Asia\/Calcutta/ })).toBeInTheDocument()
  })

  it('copies the version with the shared copy tooltip behavior', async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(window.navigator.clipboard, 'writeText').mockResolvedValue(undefined)

    renderSettings('/settings')

    expect(await screen.findByText('Version')).toBeInTheDocument()
    const copyButton = await screen.findByRole('button', { name: 'Copy version' })
    await user.hover(copyButton)
    expect(await screen.findByText('Copy version')).toBeInTheDocument()

    await user.click(copyButton)

    expect(writeText).toHaveBeenCalledWith('test')
    expect(await screen.findByText('Copied!')).toBeInTheDocument()
  })
})
