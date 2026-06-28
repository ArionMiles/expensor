import { FormEvent, useEffect, useState } from 'react'
import {
  useAccessTokens,
  useAdminUsers,
  useCreateAccessToken,
  useCreateSetupToken,
  useCreateUser,
  useRevokeAccessToken,
  useSession,
  useUpdateAdminUser,
  useUpdateProfile,
} from '@/api/queries'
import type { AccountUser, AvatarKey, UserRole } from '@/api/types'
import { useDisplay } from '@/contexts/DisplayContext'
import { useI18n } from '@/i18n/I18nProvider'
import { AvatarPicker } from '@/pages/auth/AuthPages'
import { cn, formatDate } from '@/lib/utils'

function TextField({
  label,
  value,
  type = 'text',
  disabled = false,
  onChange,
}: {
  label: string
  value: string
  type?: string
  disabled?: boolean
  onChange: (value: string) => void
}) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
      <input
        type={type}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.currentTarget.value)}
        className="w-full rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground outline-none transition-colors focus:border-primary focus:ring-1 focus:ring-ring disabled:opacity-70"
      />
    </label>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-4 border-t border-border pt-6 first:border-t-0 first:pt-0">
      <h2 className="text-sm font-medium text-foreground">{title}</h2>
      {children}
    </section>
  )
}

function ErrorText({ error }: { error: unknown }) {
  const { t } = useI18n()

  if (!error) return null
  return (
    <p className="text-sm text-destructive">
      {error instanceof Error ? error.message : t('auth.error.requestFailed')}
    </p>
  )
}

function RoleButtons({
  value,
  onChange,
}: {
  value: UserRole
  onChange: (value: UserRole) => void
}) {
  const { t } = useI18n()

  return (
    <div className="inline-flex rounded-md border border-border p-0.5">
      {(['user', 'admin'] as const).map((role) => (
        <button
          key={role}
          type="button"
          onClick={() => onChange(role)}
          className={cn(
            'rounded px-2 py-1 text-xs capitalize transition-colors',
            value === role
              ? 'bg-accent text-accent-foreground'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {t(`account.role.${role}`)}
        </button>
      ))}
    </div>
  )
}

export function AccountSettings() {
  const { t } = useI18n()
  const { timezone, timeFormat } = useDisplay()
  const { data: session } = useSession()
  const updateProfile = useUpdateProfile()
  const { data: tokens = [] } = useAccessTokens()
  const createToken = useCreateAccessToken()
  const revokeToken = useRevokeAccessToken()
  const [displayName, setDisplayName] = useState('')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>('default')
  const [profileUserID, setProfileUserID] = useState('')
  const [tokenName, setTokenName] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)

  const isAdmin = session?.role === 'admin'
  const formatAccountDate = (value?: string | null) =>
    value ? formatDate(value, true, timezone, timeFormat) : t('account.never')

  useEffect(() => {
    if (!session) return
    if (profileUserID === session.user_id) return
    setDisplayName(session.display_name)
    setAvatarKey(session.avatar_key)
    setProfileUserID(session.user_id)
  }, [profileUserID, session])

  const saveProfile = (event: FormEvent) => {
    event.preventDefault()
    updateProfile.mutate({ display_name: displayName, avatar_key: avatarKey })
  }

  const submitToken = (event: FormEvent) => {
    event.preventDefault()
    createToken.mutate(tokenName, {
      onSuccess: (token) => {
        setNewToken(token.token ?? null)
        setTokenName('')
      },
    })
  }

  if (!session) {
    return (
      <div className="flex min-h-32 items-center justify-center">
        <span className="font-mono text-xs text-muted-foreground">{t('common.loading')}</span>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-lg font-semibold text-foreground">{t('account.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('account.summary')}</p>
      </div>

      <Section title={t('account.profile.title')}>
        <form onSubmit={saveProfile} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField
              label={t('account.email')}
              value={session?.email ?? ''}
              disabled
              onChange={() => {}}
            />
            <TextField
              label={t('account.displayName')}
              value={displayName}
              onChange={setDisplayName}
            />
          </div>
          <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={updateProfile.isPending}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {updateProfile.isPending ? t('common.saving') : t('account.saveProfile')}
            </button>
            {updateProfile.isSuccess && (
              <span className="text-xs text-success">{t('account.saved')}</span>
            )}
            <ErrorText error={updateProfile.error} />
          </div>
        </form>
      </Section>

      <Section title={t('account.tokens.title')}>
        <form onSubmit={submitToken} className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <div className="min-w-0 flex-1">
            <TextField label={t('account.tokens.name')} value={tokenName} onChange={setTokenName} />
          </div>
          <button
            type="submit"
            disabled={createToken.isPending || tokenName.trim().length === 0}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {createToken.isPending ? t('account.tokens.creating') : t('account.tokens.create')}
          </button>
        </form>
        <ErrorText error={createToken.error} />
        {newToken && (
          <div className="rounded-md border border-border bg-card px-3 py-2">
            <p className="mb-1 text-xs text-muted-foreground">{t('account.tokens.copyOnce')}</p>
            <code className="break-all font-mono text-sm text-foreground">{newToken}</code>
          </div>
        )}
        <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] text-left text-sm">
            <thead className="text-xs uppercase tracking-wider text-muted-foreground">
              <tr>
                <th className="py-2 pr-3 font-medium">{t('account.tokens.columns.name')}</th>
                <th className="py-2 pr-3 font-medium">{t('account.tokens.columns.created')}</th>
                <th className="py-2 pr-3 font-medium">{t('account.tokens.columns.lastUsed')}</th>
                <th className="py-2 text-right font-medium">
                  {t('account.tokens.columns.action')}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {tokens.map((token) => (
                <tr key={token.id}>
                  <td className="py-2 pr-3 text-foreground">{token.name}</td>
                  <td className="py-2 pr-3 text-muted-foreground">
                    {formatAccountDate(token.created_at)}
                  </td>
                  <td className="py-2 pr-3 text-muted-foreground">
                    {formatAccountDate(token.last_used_at)}
                  </td>
                  <td className="py-2 text-right">
                    <button
                      type="button"
                      onClick={() => revokeToken.mutate(token.id)}
                      className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-destructive hover:text-destructive"
                    >
                      {t('account.tokens.revoke')}
                    </button>
                  </td>
                </tr>
              ))}
              {tokens.length === 0 && (
                <tr>
                  <td colSpan={4} className="py-4 text-sm text-muted-foreground">
                    {t('account.tokens.empty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </Section>

      {isAdmin && <AdminUsersSection />}
    </div>
  )
}

function AdminUsersSection() {
  const { t } = useI18n()
  const { data: users = [] } = useAdminUsers(true)
  const createUser = useCreateUser()
  const updateUser = useUpdateAdminUser()
  const setupToken = useCreateSetupToken()
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [role, setRole] = useState<UserRole>('user')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>('default')
  const [visibleSetupToken, setVisibleSetupToken] = useState<string | null>(null)

  const submitUser = (event: FormEvent) => {
    event.preventDefault()
    createUser.mutate(
      { email, display_name: displayName, role, avatar_key: avatarKey },
      {
        onSuccess: () => {
          setEmail('')
          setDisplayName('')
          setRole('user')
          setAvatarKey('default')
        },
      },
    )
  }

  const generateSetupToken = (userID: string) => {
    setupToken.mutate(userID, {
      onSuccess: (token) => setVisibleSetupToken(`/account-setup?token=${token.token}`),
    })
  }

  return (
    <Section title={t('account.users.title')}>
      <form onSubmit={submitUser} className="space-y-4 rounded-md border border-border bg-card p-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <TextField
            label={t('account.users.email')}
            type="email"
            value={email}
            onChange={setEmail}
          />
          <TextField
            label={t('account.users.displayName')}
            value={displayName}
            onChange={setDisplayName}
          />
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <RoleButtons value={role} onChange={setRole} />
          <div className="min-w-64 flex-1">
            <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            type="submit"
            disabled={createUser.isPending || !email || !displayName}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {createUser.isPending ? t('account.users.creating') : t('account.users.create')}
          </button>
          <ErrorText error={createUser.error} />
        </div>
      </form>

      {visibleSetupToken && (
        <div className="rounded-md border border-border bg-card px-3 py-2">
          <p className="mb-1 text-xs text-muted-foreground">{t('account.users.setupLinkHint')}</p>
          <code className="break-all font-mono text-sm text-foreground">{visibleSetupToken}</code>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full min-w-[760px] text-left text-sm">
          <thead className="text-xs uppercase tracking-wider text-muted-foreground">
            <tr>
              <th className="py-2 pr-3 font-medium">{t('account.users.columns.user')}</th>
              <th className="py-2 pr-3 font-medium">{t('account.users.columns.role')}</th>
              <th className="py-2 pr-3 font-medium">{t('account.users.columns.status')}</th>
              <th className="py-2 text-right font-medium">{t('account.users.columns.actions')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {users.map((account) => (
              <AdminUserRow
                key={account.user_id}
                account={account}
                updating={updateUser.isPending}
                setupPending={setupToken.isPending}
                onRole={(nextRole) =>
                  updateUser.mutate({ id: account.user_id, patch: { role: nextRole } })
                }
                onDisabled={(disabled) =>
                  updateUser.mutate({ id: account.user_id, patch: { disabled } })
                }
                onSetupToken={() => generateSetupToken(account.user_id)}
              />
            ))}
            {users.length === 0 && (
              <tr>
                <td colSpan={4} className="py-4 text-sm text-muted-foreground">
                  {t('account.users.empty')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <ErrorText error={updateUser.error ?? setupToken.error} />
    </Section>
  )
}

function AdminUserRow({
  account,
  updating,
  setupPending,
  onRole,
  onDisabled,
  onSetupToken,
}: {
  account: AccountUser
  updating: boolean
  setupPending: boolean
  onRole: (role: UserRole) => void
  onDisabled: (disabled: boolean) => void
  onSetupToken: () => void
}) {
  const { t } = useI18n()
  const disabled = !!account.disabled_at
  return (
    <tr>
      <td className="py-2 pr-3">
        <div className="text-foreground">{account.display_name}</div>
        <div className="text-xs text-muted-foreground">{account.email}</div>
      </td>
      <td className="py-2 pr-3">
        <RoleButtons value={account.role} onChange={onRole} />
      </td>
      <td className="py-2 pr-3 text-muted-foreground">
        {disabled ? t('account.status.disabled') : t('account.status.active')}
      </td>
      <td className="space-x-2 py-2 text-right">
        <button
          type="button"
          disabled={setupPending}
          onClick={onSetupToken}
          className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {t('account.users.generateSetupLink')}
        </button>
        <button
          type="button"
          disabled={updating}
          onClick={() => onDisabled(!disabled)}
          className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {disabled ? t('account.users.enable') : t('account.users.disable')}
        </button>
      </td>
    </tr>
  )
}
