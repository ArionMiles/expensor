import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PluginInfo } from '@/api/types'
import { Wizard } from './Wizard'
import { renderWithProviders } from '@/test/render'

const apiMocks = vi.hoisted(() => ({
  authStart: vi.fn(),
  setBaseCurrency: vi.fn(),
  daemonStart: vi.fn(),
  saveReaderConfig: vi.fn(),
}))

const gmailReader: PluginInfo = {
  name: 'gmail',
  description: 'Read Gmail transaction emails',
  auth_type: 'oauth',
  requires_credentials_upload: true,
  config_schema: [],
}

const thunderbirdReader: PluginInfo = {
  name: 'thunderbird',
  description: 'Read Thunderbird mailbox files',
  auth_type: 'config',
  requires_credentials_upload: false,
  config_schema: [{ name: 'mailbox', label: 'Mailbox', type: 'string', required: true }],
}

let setupStatus = { required: false, missing: [] as string[] }
let readers: PluginInfo[] = [gmailReader]
let readerStatus = {
  ready: false,
  credentials_uploaded: false,
  authenticated: false,
  config_present: false,
  auth_state: 'reauthorization_required',
}
let authStatus: { authenticated: boolean; auth_state: string; expiry?: string } = {
  authenticated: false,
  auth_state: 'reauthorization_required',
}
let activeReader = ''

vi.mock('@/api/client', () => ({
  api: {
    config: { setBaseCurrency: apiMocks.setBaseCurrency },
    daemon: { start: apiMocks.daemonStart },
    readers: { auth: { start: apiMocks.authStart } },
  },
}))

vi.mock('@/api/queries', () => ({
  queryKeys: {
    readerAuthStatus: (name: string) => ['readers', name, 'auth', 'status'],
    readerStatus: (name: string) => ['readers', name, 'status'],
    activeReader: ['config', 'active-reader'],
    setupStatus: ['config', 'setup-status'],
    status: ['status'],
  },
  useDisconnectReader: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useActiveReader: () => ({ data: activeReader }),
  useReaderAuthStatus: () => ({ data: authStatus }),
  useReaderGuide: () => ({
    data: {
      sections: [
        {
          title: 'Create credentials',
          steps: [{ text: 'Open the console' }],
        },
      ],
    },
  }),
  useReaderStatus: () => ({ data: readerStatus, isLoading: false }),
  useThunderbirdMailboxes: () => ({ data: ['Inbox', 'Receipts'], isLoading: false }),
  useThunderbirdProfiles: () => ({
    data: ['/home/user/.thunderbird/default'],
    isLoading: false,
  }),
  useReaders: () => ({
    data: readers,
    isLoading: false,
    error: null,
  }),
  useRevokeToken: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSaveReaderConfig: () => ({
    mutate: (
      payload: { readerName: string; config: Record<string, string> },
      options?: { onSuccess?: () => void },
    ) => {
      apiMocks.saveReaderConfig(payload)
      options?.onSuccess?.()
    },
    isPending: false,
    error: null,
  }),
  useSetupStatus: () => ({ data: setupStatus, isLoading: false }),
  useStatus: () => ({ data: { daemon: { running: false } } }),
  useSetTimeFormat: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetTimezone: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useTimeFormat: () => ({ data: 'HH:mm', isLoading: false }),
  useTimezone: () => ({ data: 'UTC', isLoading: false }),
  useUploadCredentials: () => ({
    mutate: vi.fn(),
    isPending: false,
    error: null,
    isSuccess: false,
  }),
}))

vi.mock('@/lib/timezone', () => ({
  getBrowserTimezone: () => 'Asia/Calcutta',
  getTimezoneOptions: () => ['Asia/Calcutta', 'UTC'],
  normalizeTimezone: (timezone: string | undefined | null) => timezone?.trim() ?? '',
}))

describe('Wizard guide layout', () => {
  beforeEach(() => {
    apiMocks.authStart.mockReset()
    apiMocks.setBaseCurrency.mockReset()
    apiMocks.setBaseCurrency.mockResolvedValue({})
    apiMocks.daemonStart.mockReset()
    apiMocks.saveReaderConfig.mockReset()
    setupStatus = { required: false, missing: [] }
    readers = [gmailReader]
    readerStatus = {
      ready: false,
      credentials_uploaded: false,
      authenticated: false,
      config_present: false,
      auth_state: 'reauthorization_required',
    }
    authStatus = { authenticated: false, auth_state: 'reauthorization_required' }
    activeReader = ''
  })

  it('renders the setup guide in a wider responsive panel', async () => {
    renderWithProviders(<Wizard />, { route: '/setup?step=guide&reader=gmail' })

    const guide = await screen.findByTestId('reader-setup-guide')
    expect(guide).toHaveClass('max-w-7xl')
    expect(screen.getByTestId('setup-form-shell')).toHaveClass('max-w-2xl')
    expect(screen.getByTestId('setup-guide-panel')).toHaveClass('2xl:max-w-xl')
  })

  it('returns from the first reader setup step to the overview instead of a duplicate reader select step', async () => {
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Set up →' }))
    expect(await screen.findByText('Upload credentials')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '← Back' }))

    expect(await screen.findByText('Reader configuration')).toBeInTheDocument()
    expect(screen.queryByText('Select a reader')).not.toBeInTheDocument()
  })

  it('does not render step progress for a single-step reader setup', async () => {
    readers = [thunderbirdReader]
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Set up →' }))

    expect(await screen.findByText('Configure reader')).toBeInTheDocument()
    expect(screen.queryByText('1')).not.toBeInTheDocument()
    expect(screen.queryByText('Configure')).not.toBeInTheDocument()
  })

  it('renders Thunderbird discovery dropdowns in fixed body portals', async () => {
    readers = [
      {
        ...thunderbirdReader,
        config_schema: [
          {
            name: 'profile_path',
            label: 'Profile path',
            type: 'thunderbird-profile',
            required: true,
          },
          {
            name: 'mailboxes',
            label: 'Mailboxes',
            type: 'thunderbird-mailboxes',
            required: true,
            depends_on: 'profile_path',
          },
        ],
      },
    ]
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Set up →' }))
    const profileInput = await screen.findByPlaceholderText(
      'e.g. /home/user/.thunderbird/abc.default',
    )
    await user.click(profileInput)

    const profileList = screen.getByRole('listbox', { name: 'Thunderbird profiles' })
    expect(profileList.parentElement).toBe(document.body)
    expect(profileList).toHaveClass('fixed')

    await user.click(screen.getByRole('option', { name: '/home/user/.thunderbird/default' }))
    await user.click(screen.getByPlaceholderText('Add mailbox (e.g. INBOX)'))

    const mailboxList = screen.getByRole('listbox', { name: 'Thunderbird mailboxes' })
    expect(mailboxList.parentElement).toBe(document.body)
    expect(mailboxList).toHaveClass('fixed')
  })

  it('shows preferences before reader setup when setup is incomplete', async () => {
    setupStatus = { required: true, missing: ['base_currency', 'timezone', 'time_format'] }

    renderWithProviders(<Wizard />, { route: '/setup' })

    expect(await screen.findByText('Preferences')).toBeInTheDocument()
    expect(
      screen.getByText('Set these preferences before connecting a reader.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Reader configuration')).not.toBeInTheDocument()
  })

  it('starts preference drafts from browser timezone and 12-hour time format', async () => {
    setupStatus = { required: true, missing: ['base_currency', 'timezone', 'time_format'] }

    renderWithProviders(<Wizard />, { route: '/setup' })

    expect(await screen.findByRole('button', { name: /Asia\/Calcutta/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /02 Jan 2006, 02:30 PM/ })).toBeInTheDocument()
  })

  it('gates reader-focused setup urls behind preferences when setup is incomplete', async () => {
    setupStatus = { required: true, missing: ['base_currency'] }

    renderWithProviders(<Wizard />, { route: '/setup?step=guide&reader=gmail' })

    expect(await screen.findByText('Preferences')).toBeInTheDocument()
    expect(screen.queryByTestId('reader-setup-guide')).not.toBeInTheDocument()
  })

  it('advances to reader configuration after preferences are saved', async () => {
    setupStatus = { required: true, missing: ['base_currency', 'timezone', 'time_format'] }
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Continue' }))

    expect(await screen.findByText('Reader configuration')).toBeInTheDocument()
  })

  it('does not offer reauthorization or access-token expiry details while connected', async () => {
    readerStatus = {
      ready: true,
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_state: 'connected',
    }
    authStatus = {
      authenticated: true,
      auth_state: 'connected',
      expiry: new Date(Date.now() + 86_400_000).toISOString(),
    }

    renderWithProviders(<Wizard />, { route: '/setup' })

    expect(await screen.findByText('● Connected')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Re-authorize' })).not.toBeInTheDocument()
    expect(screen.queryByText('expires tomorrow')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start tracking →' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Remove all data' })).toBeInTheDocument()
  })

  it('shows hover tooltips for the connected Gmail action icons', async () => {
    readerStatus = {
      ready: true,
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_state: 'connected',
    }
    authStatus = { authenticated: true, auth_state: 'connected' }
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    const disconnect = screen.getByRole('button', { name: 'Disconnect' })
    await user.hover(disconnect)

    const tooltip = await screen.findByText('Disconnect')
    expect(tooltip.parentElement).toBe(document.body)
    expect(screen.getByRole('button', { name: 'Remove all data' })).toBeInTheDocument()
  })

  it('marks the active reader with an Active label and explanatory tooltip', async () => {
    readers = [gmailReader, thunderbirdReader]
    readerStatus = {
      ready: true,
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_state: 'connected',
    }
    authStatus = { authenticated: true, auth_state: 'connected' }
    activeReader = 'gmail'
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    const activeBadge = await screen.findByText('Active')
    expect(screen.getAllByText('Active')).toHaveLength(1)
    expect(activeBadge.closest('[data-reader-card]')).toHaveAttribute('data-reader-card', 'gmail')

    await user.hover(activeBadge)

    const tooltip = await screen.findByText(
      'The background daemon imports new transactions from this reader.',
    )
    expect(tooltip.parentElement).toBe(document.body)
  })

  it('warns before switching a connected inactive reader to active', async () => {
    readers = [gmailReader, thunderbirdReader]
    readerStatus = {
      ready: true,
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_state: 'connected',
    }
    authStatus = { authenticated: true, auth_state: 'connected' }
    activeReader = 'gmail'
    apiMocks.daemonStart.mockResolvedValue({})
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Make active' }))

    const dialog = screen.getByRole('dialog', { name: 'Make Thunderbird active?' })
    expect(dialog).toBeInTheDocument()
    expect(screen.getByText(/double-count transactions/i)).toBeInTheDocument()
    expect(apiMocks.daemonStart).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole('button', { name: 'Make active' }))

    await waitFor(() => expect(apiMocks.daemonStart).toHaveBeenCalledWith('thunderbird'))
  })

  it('warns before making a newly configured second reader active', async () => {
    readers = [gmailReader, thunderbirdReader]
    activeReader = 'gmail'
    apiMocks.daemonStart.mockResolvedValue({})
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    const thunderbirdCard = await screen.findByText('Thunderbird')
    await user.click(
      within(thunderbirdCard.closest('[data-reader-card]') as HTMLElement).getByRole('button', {
        name: 'Set up →',
      }),
    )
    await user.type(await screen.findByLabelText(/Mailbox/), 'INBOX')
    await user.click(screen.getByRole('button', { name: 'Save & continue →' }))

    const dialog = screen.getByRole('dialog', { name: 'Make Thunderbird active?' })
    expect(dialog).toBeInTheDocument()
    expect(screen.getByText(/double-count transactions/i)).toBeInTheDocument()
    expect(apiMocks.daemonStart).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole('button', { name: 'Make active' }))

    await waitFor(() => expect(apiMocks.daemonStart).toHaveBeenCalledWith('thunderbird'))
  })

  it('starts authorization from the overview action without a second open-tab step', async () => {
    readerStatus = {
      ready: false,
      credentials_uploaded: true,
      authenticated: false,
      config_present: true,
      auth_state: 'reauthorization_required',
    }
    apiMocks.authStart.mockResolvedValue({
      data: {
        url: 'https://accounts.google.test/oauth',
        redirect_uri: 'http://localhost:8080/api/auth/callback',
      },
    })
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Authorize →' }))

    expect(apiMocks.authStart).toHaveBeenCalledWith('gmail')
    expect(openSpy).toHaveBeenCalledWith(
      'https://accounts.google.test/oauth',
      '_blank',
      'noopener,noreferrer',
    )
    expect(
      screen.queryByRole('button', { name: 'Open authorization tab →' }),
    ).not.toBeInTheDocument()
    expect(await screen.findByText('Waiting for authorization...')).toBeInTheDocument()
  })

  it('starts tracking automatically when overview authorization succeeds', async () => {
    readerStatus = {
      ready: false,
      credentials_uploaded: true,
      authenticated: false,
      config_present: true,
      auth_state: 'reauthorization_required',
    }
    authStatus = { authenticated: true, auth_state: 'connected' }
    apiMocks.authStart.mockResolvedValue({
      data: {
        url: 'https://accounts.google.test/oauth',
        redirect_uri: 'http://localhost:8080/api/auth/callback',
      },
    })
    apiMocks.daemonStart.mockResolvedValue({})
    vi.spyOn(window, 'open').mockImplementation(() => null)
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    await user.click(await screen.findByRole('button', { name: 'Authorize →' }))

    await waitFor(() => expect(apiMocks.daemonStart).toHaveBeenCalledWith('gmail'))
  })

  it('shows Thunderbird remove-all data as a right-aligned action with reader-specific copy', async () => {
    readers = [thunderbirdReader]
    readerStatus = {
      ready: true,
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_state: 'connected',
    }
    authStatus = { authenticated: true, auth_state: 'connected' }
    const user = userEvent.setup()

    renderWithProviders(<Wizard />, { route: '/setup' })

    expect(screen.queryByRole('button', { name: 'Disconnect' })).not.toBeInTheDocument()
    const removeAll = screen.getByRole('button', { name: 'Remove all data' })
    await user.hover(removeAll)
    expect(await screen.findByText('Remove all data')).toBeInTheDocument()

    await user.click(removeAll)

    expect(
      screen.getByRole('dialog', { name: 'Remove all data for Thunderbird?' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/mailbox configuration/)).toBeInTheDocument()
    expect(screen.getByText(/saved config/)).toBeInTheDocument()
  })
})
