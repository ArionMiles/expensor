import { screen, waitFor, within } from '@testing-library/react'
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

function mockOpenAISettings({
  credentialsStored = false,
  ready = false,
  model = 'gpt-5.4-mini',
  baseUrl = 'https://api.openai.com/v1',
  requests,
}: {
  credentialsStored?: boolean
  ready?: boolean
  model?: string
  baseUrl?: string
  requests?: Array<{ path: string; body?: unknown }>
} = {}) {
  server.use(
    http.get('/api/llm/providers', () =>
      HttpResponse.json([
        {
          name: 'openai',
          display_name: 'OpenAI',
          description: 'OpenAI API',
          auth_type: 'api_key',
          capabilities: ['text_generation', 'json_schema'],
          model_options: [
            {
              id: 'gpt-5.4-mini',
              display_name: 'GPT-5.4 mini',
              quality: 'Balanced',
              cost: 'Lower',
              description: 'Recommended for rule drafting.',
              recommended: true,
            },
            {
              id: 'gpt-5.4',
              display_name: 'GPT-5.4',
              quality: 'High',
              cost: 'Medium',
              description: 'Use when drafts need more reasoning headroom.',
            },
          ],
        },
      ]),
    ),
    http.get('/api/llm/providers/openai/status', () =>
      HttpResponse.json({
        name: 'openai',
        config: { model, base_url: baseUrl },
        config_present: true,
        credentials_stored: credentialsStored,
        active: ready,
        ready,
      }),
    ),
    http.put('/api/llm/providers/openai/config', async ({ request }) => {
      const body = await request.json()
      requests?.push({ path: '/api/llm/providers/openai/config', body })
      return HttpResponse.json({})
    }),
    http.put('/api/llm/providers/openai/credentials', async ({ request }) => {
      const body = await request.json()
      requests?.push({ path: '/api/llm/providers/openai/credentials', body })
      return HttpResponse.json({})
    }),
    http.post('/api/llm/providers/openai/healthcheck', () => {
      requests?.push({ path: '/api/llm/providers/openai/healthcheck' })
      return HttpResponse.json({ status: 'ok', message: 'OpenAI connection is healthy.' })
    }),
    http.post('/api/llm/providers/openai/activate', () => {
      requests?.push({ path: '/api/llm/providers/openai/activate' })
      return HttpResponse.json({ status: 'activated' })
    }),
    http.delete('/api/llm/providers/openai', () => {
      requests?.push({ path: '/api/llm/providers/openai' })
      return new HttpResponse(null, { status: 204 })
    }),
  )
}

describe('Settings', () => {
  it('uses the tab query param to control the visible section', async () => {
    renderSettings('/settings?tab=sync')

    expect(await screen.findByText('Sync status')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/settings?tab=sync')
  })

  it('orders settings tabs with community after account and admin last', async () => {
    renderSettings('/settings')

    expect(await screen.findByText('Base currency')).toBeInTheDocument()

    const tabs = screen
      .getAllByRole('button')
      .map((button) => button.textContent?.trim())
      .filter((label) =>
        ['General', 'Scanning', 'Account', 'AI', 'Community', 'Admin'].includes(label ?? ''),
      )

    expect(tabs).toEqual(['General', 'Scanning', 'Account', 'AI', 'Community', 'Admin'])
  })

  it('updates the tab query param when the user clicks a tab', async () => {
    const user = userEvent.setup()

    renderSettings('/settings')

    await user.click(screen.getByRole('button', { name: 'Scanning' }))

    expect(await screen.findByText('Scan interval')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/settings?tab=daemon')
  })

  it('keeps status indicator settings on the general tab', async () => {
    const user = userEvent.setup()

    renderSettings('/settings')

    expect(await screen.findByText('Status indicator')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Scanning' }))

    expect(await screen.findByText('Scan interval')).toBeInTheDocument()
    expect(screen.queryByText('Status indicator')).not.toBeInTheDocument()
  })

  it('aligns settings row actions to the right of their explanatory text', async () => {
    const user = userEvent.setup()

    renderSettings('/settings?tab=general')

    expect(await screen.findByText('Status indicator')).toBeInTheDocument()
    expect(screen.getByTestId('status-indicator-row')).toHaveClass('sm:flex-row')

    await user.click(screen.getByRole('button', { name: 'Scanning' }))

    expect(await screen.findByRole('heading', { name: 'Force rescan' })).toBeInTheDocument()
    expect(screen.getByTestId('force-rescan-row')).toHaveClass('sm:flex-row')

    await user.click(screen.getByRole('button', { name: 'Community' }))

    expect(await screen.findByRole('heading', { name: 'Automatic sync' })).toBeInTheDocument()
    expect(screen.getByTestId('automatic-sync-row')).toHaveClass('sm:flex-row')
    expect(screen.getByTestId('manual-sync-row')).toHaveClass('sm:flex-row')
  })

  it('saves general preferences when a dropdown value is selected', async () => {
    const user = userEvent.setup()
    const patches: Array<Record<string, unknown>> = []
    server.use(
      http.get('/api/config/preferences', () =>
        HttpResponse.json({
          base_currency: 'INR',
          scan_interval: 60,
          lookback_days: 180,
          timezone: 'Asia/Calcutta',
          time_format: 'HH:mm',
        }),
      ),
      http.patch('/api/config/preferences', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        patches.push(body)
        return HttpResponse.json({
          base_currency: body.base_currency ?? 'INR',
          scan_interval: 60,
          lookback_days: 180,
          timezone: body.timezone ?? 'Asia/Calcutta',
          time_format: body.time_format ?? 'HH:mm',
        })
      }),
    )

    renderSettings('/settings?tab=general')

    expect(await screen.findByText('Base currency')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('combobox', { name: /Base currency INR/ }))
    await user.click(await screen.findByRole('option', { name: /USD/ }))

    await waitFor(() => expect(patches).toEqual([{ base_currency: 'USD' }]))
  })

  it('saves scanning preferences when the edited field loses focus', async () => {
    const user = userEvent.setup()
    const patches: Array<Record<string, unknown>> = []
    server.use(
      http.get('/api/config/preferences', () =>
        HttpResponse.json({
          base_currency: 'INR',
          scan_interval: 60,
          lookback_days: 180,
          timezone: 'Asia/Calcutta',
          time_format: 'HH:mm',
        }),
      ),
      http.patch('/api/config/preferences', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        patches.push(body)
        return HttpResponse.json({
          base_currency: 'INR',
          scan_interval: body.scan_interval ?? 60,
          lookback_days: body.lookback_days ?? 180,
          timezone: 'Asia/Calcutta',
          time_format: 'HH:mm',
        })
      }),
    )

    renderSettings('/settings?tab=daemon')

    expect(await screen.findByText('Scan interval')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()

    const scanInterval = await screen.findByDisplayValue('60')
    await user.clear(scanInterval)
    await user.type(scanInterval, '120')
    expect(patches).toEqual([])

    await user.tab()

    await waitFor(() => expect(patches).toEqual([{ scan_interval: 120 }]))
  })

  it('saves admin scanning capacity when the edited field loses focus', async () => {
    const user = userEvent.setup()
    const patches: Array<Record<string, unknown>> = []
    server.use(
      http.get('/api/admin/scanning/settings', () =>
        HttpResponse.json({ max_concurrent_scans: 4, updated_at: '2026-07-01T10:00:00Z' }),
      ),
      http.patch('/api/admin/scanning/settings', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        patches.push(body)
        return HttpResponse.json({
          max_concurrent_scans: body.max_concurrent_scans,
          updated_at: '2026-07-01T10:01:00Z',
        })
      }),
    )

    renderSettings('/settings?tab=admin')

    const capacityHeading = await screen.findByRole('heading', { name: 'Scanning capacity' })
    const usersHeading = await screen.findByRole('heading', { name: 'Users' })
    expect(
      Boolean(
        capacityHeading.compareDocumentPosition(usersHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ).toBe(true)

    const input = await screen.findByDisplayValue('4')
    await user.clear(input)
    await user.type(input, '6')
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
    expect(patches).toEqual([])

    await user.tab()

    await waitFor(() => expect(patches).toEqual([{ max_concurrent_scans: 6 }]))
  })

  it('saves admin log level when a dropdown value is selected', async () => {
    const user = userEvent.setup()
    const patches: Array<Record<string, unknown>> = []
    server.use(
      http.get('/api/admin/logging/settings', () => HttpResponse.json({ level: 'info' })),
      http.patch('/api/admin/logging/settings', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        patches.push(body)
        return HttpResponse.json(body)
      }),
    )

    renderSettings('/settings?tab=admin')

    const loggingHeading = await screen.findByRole('heading', { name: 'Log level' })
    const usersHeading = await screen.findByRole('heading', { name: 'Users' })
    expect(
      Boolean(
        loggingHeading.compareDocumentPosition(usersHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ).toBe(true)
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('combobox', { name: 'Log level Info' }))
    await user.click(await screen.findByRole('option', { name: /Debug/ }))

    await waitFor(() => expect(patches).toEqual([{ level: 'debug' }]))
    expect(screen.queryByText('Saved.')).not.toBeInTheDocument()
  })

  it('toggles automatic community sync from the community tab', async () => {
    const user = userEvent.setup()
    const patches: Array<Record<string, unknown>> = []
    server.use(
      http.get('/api/config/sync/settings', () =>
        HttpResponse.json({ automatic_sync_enabled: true }),
      ),
      http.patch('/api/config/sync/settings', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        patches.push(body)
        return HttpResponse.json(body)
      }),
    )

    renderSettings('/settings?tab=sync')

    expect(await screen.findByRole('button', { name: 'Community' })).toBeInTheDocument()
    const toggle = await screen.findByRole('switch', { name: 'Automatic sync' })
    expect(toggle).toHaveAttribute('aria-checked', 'true')

    await user.click(toggle)

    await waitFor(() => expect(patches).toEqual([{ automatic_sync_enabled: false }]))
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

  it('saves OpenAI settings and tests the connection with normalized config', async () => {
    const user = userEvent.setup()
    const requests: Array<{ path: string; body?: unknown }> = []
    mockOpenAISettings({ requests })

    renderSettings('/settings?tab=ai')

    expect(await screen.findByRole('heading', { name: 'OpenAI' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /OpenAI API docs/ })).toHaveAttribute(
      'href',
      'https://developers.openai.com/api/reference/overview',
    )
    expect(screen.queryByDisplayValue('https://api.openai.com/v1')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit base URL' }))
    expect(screen.getByDisplayValue('https://api.openai.com/v1')).toBeInTheDocument()

    const modelInput = screen.getByLabelText('Model')
    await user.clear(modelInput)
    await user.type(modelInput, 'GPT-5.4')
    await user.hover(await screen.findByText('Quality: High'))
    expect(
      await screen.findByText('Relative rule-drafting quality and reasoning headroom.'),
    ).toBeInTheDocument()
    await user.click(await screen.findByText('GPT-5.4'))
    await user.type(screen.getByLabelText('API key'), 'sk-test')
    await user.click(screen.getByRole('button', { name: 'Test' }))

    await waitFor(() =>
      expect(requests).toEqual([
        {
          path: '/api/llm/providers/openai/config',
          body: { config: { model: 'gpt-5.4', base_url: 'https://api.openai.com/v1' } },
        },
        { path: '/api/llm/providers/openai/credentials', body: { api_key: 'sk-test' } },
        { path: '/api/llm/providers/openai/healthcheck' },
      ]),
    )
    expect(await screen.findByText('OpenAI connection is healthy.')).toBeInTheDocument()
  })

  it('shows a stored OpenAI key placeholder and disconnects through confirmation', async () => {
    const user = userEvent.setup()
    const requests: Array<{ path: string; body?: unknown }> = []
    mockOpenAISettings({ credentialsStored: true, ready: true, requests })

    renderSettings('/settings?tab=ai')

    expect(await screen.findByText('Ready')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('••••••••••••••••')).toBeInTheDocument()
    expect(screen.queryByText('Stored encrypted in Expensor.')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(screen.getByRole('dialog', { name: 'Disconnect OpenAI?' })).toBeInTheDocument()
    await user.click(
      within(screen.getByRole('dialog', { name: 'Disconnect OpenAI?' })).getByRole('button', {
        name: 'Disconnect',
      }),
    )

    await waitFor(() => expect(requests).toContainEqual({ path: '/api/llm/providers/openai' }))
    expect(await screen.findByText('OpenAI disconnected.')).toBeInTheDocument()
  })
})
