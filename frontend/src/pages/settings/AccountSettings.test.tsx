import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import Settings from '../Settings'
import { server } from '@/test/server'
import { renderWithProviders } from '@/test/render'

const adminPrincipal = {
  user_id: 'admin',
  tenant_id: 'admin',
  email: 'admin@example.com',
  display_name: 'Admin',
  role: 'admin',
  avatar_key: 'default',
}

describe('AccountSettings', () => {
  it('updates profile details, creates access tokens, and generates setup links for invited users', async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(window.navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    const updatedProfiles: Array<{ display_name?: string; avatar_key?: string }> = []
    const createdTokens: string[] = []
    const setupTokenRequests: string[] = []
    server.use(
      http.get('/api/session', () => HttpResponse.json(adminPrincipal)),
      http.patch('/api/profile', async ({ request }) => {
        const body = (await request.json()) as { display_name?: string; avatar_key?: string }
        updatedProfiles.push(body)
        return HttpResponse.json({
          ...adminPrincipal,
          display_name: body.display_name,
          avatar_key: body.avatar_key,
        })
      }),
      http.get('/api/tokens', () =>
        HttpResponse.json([
          {
            id: 'token-1',
            name: 'CI',
            created_at: '2026-06-01T10:00:00Z',
            expires_at: null,
            last_used_at: null,
          },
        ]),
      ),
      http.post('/api/tokens', async ({ request }) => {
        const body = (await request.json()) as { name?: string }
        createdTokens.push(body.name ?? '')
        return HttpResponse.json(
          {
            id: 'token-2',
            name: body.name,
            token: 'expensor_pat_visible_once',
            created_at: '2026-06-02T10:00:00Z',
            expires_at: null,
            last_used_at: null,
          },
          { status: 201 },
        )
      }),
      http.get('/api/admin/users', () =>
        HttpResponse.json([
          {
            user_id: 'admin',
            tenant_id: 'admin',
            email: 'admin@example.com',
            display_name: 'Admin',
            role: 'admin',
            avatar_key: 'default',
            disabled_at: null,
            created_at: '2026-06-01T10:00:00Z',
            updated_at: '2026-06-01T10:00:00Z',
          },
          {
            user_id: 'user-b',
            tenant_id: 'user-b',
            email: 'b@example.com',
            display_name: 'B',
            role: 'user',
            avatar_key: 'ledger',
            disabled_at: null,
            created_at: '2026-06-01T10:00:00Z',
            updated_at: '2026-06-01T10:00:00Z',
          },
        ]),
      ),
      http.post('/api/admin/users/:id/setup-tokens', ({ params }) => {
        setupTokenRequests.push(String(params.id))
        return HttpResponse.json(
          {
            token: 'expensor_setup_visible_once',
            expires_at: '2026-06-03T10:00:00Z',
          },
          { status: 201 },
        )
      }),
    )

    renderWithProviders(<Settings />, { route: '/settings?tab=account' })

    expect(await screen.findByRole('heading', { name: 'Account' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save profile' })).not.toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
    const adminRow = await screen.findByRole('row', { name: /admin@example.com/i })
    expect(within(adminRow).getByText('ADMIN')).toBeInTheDocument()
    expect(within(adminRow).getByText('ACTIVE')).toBeInTheDocument()
    expect(within(adminRow).queryByRole('button', { name: 'user' })).not.toBeInTheDocument()
    expect(within(adminRow).queryByRole('button', { name: 'admin' })).not.toBeInTheDocument()
    expect(within(adminRow).queryByRole('button', { name: 'Disable' })).not.toBeInTheDocument()
    const profileSection = screen.getByRole('heading', { name: 'Profile' }).closest('section')
    if (!profileSection) throw new Error('Profile section missing')
    await user.clear(screen.getByLabelText('Display name'))
    await user.type(screen.getByLabelText('Display name'), 'Admin Updated')
    await user.click(within(profileSection).getByRole('button', { name: 'Default avatar' }))
    await user.click(within(profileSection).getByRole('button', { name: 'Ledger avatar' }))

    await waitFor(() =>
      expect(updatedProfiles).toContainEqual({
        display_name: 'Admin Updated',
        avatar_key: 'ledger',
      }),
    )

    await user.type(screen.getByLabelText('Token name'), 'Deploy key')
    await user.click(screen.getByRole('button', { name: 'Create token' }))

    expect(await screen.findByText('expensor_pat_visible_once')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Copy token' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('expensor_pat_visible_once'))
    expect(await screen.findByText('Token copied.')).toBeInTheDocument()
    expect(createdTokens).toEqual(['Deploy key'])

    const invitedRow = await screen.findByRole('row', { name: /b@example.com/i })
    expect(within(invitedRow).getByRole('button', { name: 'USER' })).toBeInTheDocument()
    expect(within(invitedRow).getByRole('button', { name: 'ADMIN' })).toBeInTheDocument()
    expect(within(invitedRow).getByText('ACTIVE')).toBeInTheDocument()
    expect(within(invitedRow).getByRole('button', { name: 'Disable' })).toHaveClass(
      'hover:border-destructive',
    )
    await user.click(within(invitedRow).getByRole('button', { name: 'Generate setup link' }))

    expect(await screen.findByText(/expensor_setup_visible_once/)).toBeInTheDocument()
    expect(writeText).toHaveBeenCalledWith('/account-setup?token=expensor_setup_visible_once')
    expect(await screen.findByText('Setup link copied.')).toBeInTheDocument()
    expect(setupTokenRequests).toEqual(['user-b'])
  })

  it('shows active duplicate access token names as conflicts', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('/api/session', () => HttpResponse.json(adminPrincipal)),
      http.get('/api/tokens', () => HttpResponse.json([])),
      http.get('/api/admin/users', () => HttpResponse.json([])),
      http.post('/api/tokens', () =>
        HttpResponse.json({ error: 'Token test already exists.' }, { status: 409 }),
      ),
    )

    renderWithProviders(<Settings />, { route: '/settings?tab=account' })

    await user.type(await screen.findByLabelText('Token name'), 'test')
    await user.click(screen.getByRole('button', { name: 'Create token' }))

    expect(await screen.findByText('Token test already exists.')).toBeInTheDocument()
  })
})
