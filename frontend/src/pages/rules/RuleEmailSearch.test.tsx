import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '@/i18n/I18nProvider'
import { loadRuleEmailSearchDraft } from './emailSearchDraft'
import { RuleEmailSearch } from './RuleEmailSearch'

const queryMocks = vi.hoisted(() => ({
  activeReader: 'gmail',
  searchMessages: vi.fn(),
  searchData: undefined as
    | {
        results: Array<{
          id: string
          sender_email: string
          subject: string
          body: string
          received_at?: string
        }>
      }
    | undefined,
  isPending: false,
  error: null as Error | null,
}))

vi.mock('@/api/queries', () => ({
  useActiveReader: () => ({ data: queryMocks.activeReader }),
  useSearchReaderMessages: () => ({
    mutate: queryMocks.searchMessages,
    data: queryMocks.searchData,
    isPending: queryMocks.isPending,
    error: queryMocks.error,
  }),
}))

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname + location.search}</div>
}

function renderSearch(route = '/rules/new/search') {
  render(
    <I18nProvider>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route
            path="/rules/new/search"
            element={
              <>
                <RuleEmailSearch />
                <LocationProbe />
              </>
            }
          />
          <Route path="/rules/new" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </I18nProvider>,
  )
}

describe('RuleEmailSearch', () => {
  beforeEach(() => {
    queryMocks.activeReader = 'gmail'
    queryMocks.searchMessages.mockReset()
    queryMocks.searchData = undefined
    queryMocks.isPending = false
    queryMocks.error = null
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('stores search state in the URL and searches the active reader', async () => {
    const user = userEvent.setup()
    renderSearch()

    await user.click(screen.getByRole('button', { name: 'Rows per page: 10' }))
    await user.click(screen.getByRole('menuitem', { name: '25' }))
    await user.type(screen.getByPlaceholderText('Search subject lines...'), 'Card spend{Enter}')

    expect(screen.getByTestId('location')).toHaveTextContent(
      '/rules/new/search?q=Card+spend&limit=25',
    )
    await waitFor(() =>
      expect(queryMocks.searchMessages).toHaveBeenCalledWith({
        reader: 'gmail',
        subject: 'Card spend',
        limit: 25,
      }),
    )
  })

  it('lets users inspect, select, and start a rule from email results', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(new Date('2026-07-06T12:00:00Z').getTime())
    const user = userEvent.setup()
    queryMocks.searchData = {
      results: [
        {
          id: 'message-1',
          sender_email: 'alerts@example.com',
          subject: 'Card spend approved',
          body: 'INR 42.00 at Coffee\nAvailable limit updated.',
          received_at: '2026-07-06T11:00:00Z',
        },
      ],
    }
    renderSearch('/rules/new/search?q=Card+spend&limit=10')

    const results = screen.getByLabelText('Email search results')
    expect(within(results).getByText('Card spend approved')).toBeInTheDocument()
    expect(within(results).getByText('1 hour ago')).toBeInTheDocument()

    await user.click(within(results).getByRole('button', { expanded: false }))
    expect(screen.getByText(/Available limit updated/)).toBeInTheDocument()
    await user.hover(screen.getByText('1 hour ago'))
    expect(screen.getByText(/2026/)).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Select Card spend approved' }))
    await user.click(screen.getByRole('button', { name: 'Create Rule' }))

    const location = screen.getByTestId('location').textContent ?? ''
    expect(location).toMatch(/^\/rules\/new\?draft=/)
    const draftID = new URLSearchParams(location.split('?')[1]).get('draft')
    expect(draftID).toBeTruthy()
    const draft = loadRuleEmailSearchDraft(draftID ?? '')
    expect(draft?.subjectQuery).toBe('Card spend')
    expect(draft?.messages[0]?.sender_email).toBe('alerts@example.com')
  })
})
