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
    const revokedTokens: string[] = []
    const setupTokenRequests: string[] = []
    const updatedUsers: Array<{ id: string; patch: Record<string, unknown> }> = []
    const createdUsers: Array<Record<string, unknown>> = []
    const deletedUsers: string[] = []
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
      http.delete('/api/tokens/:id', ({ params }) => {
        revokedTokens.push(String(params.id))
        return new HttpResponse(null, { status: 204 })
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
      http.post('/api/admin/users', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        createdUsers.push(body)
        return HttpResponse.json(
          {
            user_id: 'user-c',
            tenant_id: 'user-c',
            email: body.email,
            display_name: body.display_name,
            role: body.role,
            avatar_key: body.avatar_key,
            disabled_at: null,
            created_at: '2026-06-01T10:00:00Z',
            updated_at: '2026-06-01T10:00:00Z',
          },
          { status: 201 },
        )
      }),
      http.patch('/api/admin/users/:id', async ({ params, request }) => {
        const body = (await request.json()) as Record<string, unknown>
        updatedUsers.push({ id: String(params.id), patch: body })
        return HttpResponse.json({
          user_id: params.id,
          tenant_id: params.id,
          email: 'b@example.com',
          display_name: 'B',
          role: body.role ?? 'user',
          avatar_key: 'ledger',
          disabled_at: body.disabled === true ? '2026-06-04T10:00:00Z' : null,
          created_at: '2026-06-01T10:00:00Z',
          updated_at: '2026-06-04T10:00:00Z',
        })
      }),
      http.delete('/api/admin/users/:id', ({ params }) => {
        deletedUsers.push(String(params.id))
        return new HttpResponse(null, { status: 204 })
      }),
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
    expect(within(adminRow).queryByRole('button', { name: /edit user/i })).not.toBeInTheDocument()
    expect(within(adminRow).queryByRole('button', { name: 'Disable' })).not.toBeInTheDocument()
    const profileSection = screen.getByRole('heading', { name: 'Profile' }).closest('section')
    if (!profileSection) throw new Error('Profile section missing')
    const profileInputs = within(profileSection).getAllByRole('textbox')
    expect(profileInputs[0]).toHaveAccessibleName('Email')
    expect(profileInputs[1]).toHaveAccessibleName('Display name')
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

    expect(screen.queryByRole('textbox', { name: 'Token name' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'New token' }))
    const createTokenDialog = await screen.findByRole('dialog', { name: 'New token' })
    await user.type(within(createTokenDialog).getByLabelText('Token name'), 'Deploy key')
    await user.click(within(createTokenDialog).getByRole('button', { name: 'Create token' }))

    const tokenDialog = await screen.findByRole('dialog', { name: 'Deploy key' })
    expect(within(tokenDialog).getByText('expensor_pat_visible_once')).toBeInTheDocument()
    const copyTokenButton = within(tokenDialog).getByRole('button', { name: 'Copy token' })
    await user.hover(copyTokenButton)
    expect(await screen.findByText('Copy token')).toBeInTheDocument()
    await user.click(copyTokenButton)
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('expensor_pat_visible_once'))
    expect(await screen.findByText('Copied!')).toBeInTheDocument()
    await user.click(within(tokenDialog).getByRole('button', { name: 'Close' }))
    expect(createdTokens).toEqual(['Deploy key'])

    const tokenRow = await screen.findByRole('row', { name: /CI/i })
    await user.click(within(tokenRow).getByRole('button', { name: 'Revoke token CI' }))
    const revokeDialog = await screen.findByRole('dialog', { name: 'Revoke token' })
    await user.click(within(revokeDialog).getByRole('button', { name: 'Revoke' }))
    expect(revokedTokens).toEqual(['token-1'])

    expect(screen.queryByRole('textbox', { name: 'User email' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'New user' }))
    const newUserDialog = await screen.findByRole('dialog', { name: 'New user' })
    await user.type(within(newUserDialog).getByLabelText('User email'), 'c@example.com')
    await user.type(within(newUserDialog).getByLabelText('User display name'), 'C')
    await user.click(within(newUserDialog).getByRole('button', { name: 'Default avatar' }))
    await user.click(within(newUserDialog).getByRole('button', { name: 'Ledger avatar' }))
    await user.click(within(newUserDialog).getByRole('button', { name: 'Create user' }))
    await waitFor(() =>
      expect(createdUsers).toContainEqual({
        email: 'c@example.com',
        display_name: 'C',
        role: 'user',
        avatar_key: 'ledger',
      }),
    )

    const invitedRow = await screen.findByRole('row', { name: /b@example.com/i })
    expect(within(invitedRow).getByText('USER')).toBeInTheDocument()
    expect(within(invitedRow).getByText('ACTIVE')).toBeInTheDocument()
    expect(within(invitedRow).queryByRole('button', { name: 'USER' })).not.toBeInTheDocument()
    expect(within(invitedRow).queryByRole('button', { name: 'ADMIN' })).not.toBeInTheDocument()
    expect(within(invitedRow).queryByRole('button', { name: 'Disable' })).not.toBeInTheDocument()

    const copySetupButton = within(invitedRow).getByRole('button', { name: 'Copy setup link' })
    await user.hover(copySetupButton)
    expect(await screen.findByText('Copy setup link')).toBeInTheDocument()
    await user.click(copySetupButton)

    expect(writeText).toHaveBeenCalledWith('/account-setup?token=expensor_setup_visible_once')
    expect(await screen.findByText('Copied!')).toBeInTheDocument()
    expect(setupTokenRequests).toEqual(['user-b'])

    await user.click(within(invitedRow).getByRole('button', { name: 'Edit user B' }))
    const editDialog = await screen.findByRole('dialog', { name: 'Edit user B' })
    await user.click(within(editDialog).getByRole('button', { name: 'Admin' }))
    await user.click(within(editDialog).getByRole('button', { name: 'Disabled' }))
    await user.click(within(editDialog).getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(updatedUsers).toContainEqual({
        id: 'user-b',
        patch: { role: 'admin', disabled: true },
      }),
    )

    await user.click(within(invitedRow).getByRole('button', { name: 'Edit user B' }))
    const deleteDialog = await screen.findByRole('dialog', { name: 'Edit user B' })
    await user.click(within(deleteDialog).getByRole('button', { name: 'Delete user' }))
    const confirmDeleteDialog = await screen.findByRole('dialog', { name: 'Delete user' })
    await user.click(within(confirmDeleteDialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(deletedUsers).toEqual(['user-b']))
    expect(screen.queryByRole('dialog', { name: 'Edit user B' })).not.toBeInTheDocument()
  }, 10000)

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

    await user.click(await screen.findByRole('button', { name: 'New token' }))
    const createTokenDialog = await screen.findByRole('dialog', { name: 'New token' })
    await user.type(within(createTokenDialog).getByLabelText('Token name'), 'test')
    await user.click(within(createTokenDialog).getByRole('button', { name: 'Create token' }))

    expect(await screen.findByText('Token test already exists.')).toBeInTheDocument()
  })
})
