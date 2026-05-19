import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Wizard } from './Wizard'
import { renderWithProviders } from '@/test/render'

let setupStatus = { required: false, missing: [] as string[] }

vi.mock('@/api/queries', () => ({
  queryKeys: {
    readerAuthStatus: (name: string) => ['readers', name, 'auth', 'status'],
    readerStatus: (name: string) => ['readers', name, 'status'],
    setupStatus: ['config', 'setup-status'],
    status: ['status'],
  },
  useDisconnectReader: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useReaderAuthStatus: () => ({ data: { authenticated: false } }),
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
  useReaderStatus: () => ({
    data: {
      ready: false,
      credentials_uploaded: false,
      authenticated: false,
      config_present: false,
    },
    isLoading: false,
  }),
  useReaders: () => ({
    data: [
      {
        name: 'gmail',
        description: 'Read Gmail transaction emails',
        auth_type: 'oauth',
        requires_credentials_upload: true,
        config_schema: [],
      },
    ],
    isLoading: false,
    error: null,
  }),
  useRevokeToken: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
    setupStatus = { required: false, missing: [] }
  })

  it('renders the setup guide in a wider responsive panel', async () => {
    renderWithProviders(<Wizard />, { route: '/setup?step=guide&reader=gmail' })

    const guide = await screen.findByTestId('reader-setup-guide')
    expect(guide).toHaveClass('max-w-5xl')
    expect(screen.getByTestId('setup-form-shell')).toHaveClass('max-w-2xl')
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
})
