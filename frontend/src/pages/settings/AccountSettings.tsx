import { Check, Copy } from 'lucide-react'
import { FormEvent, useEffect, useRef, useState } from 'react'
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
    <div className="inline-flex rounded-md border border-border bg-card p-0.5">
      {(['user', 'admin'] as const).map((role) => (
        <button
          key={role}
          type="button"
          onClick={() => onChange(role)}
          className={cn(
            'rounded px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider transition-colors',
            value === role
              ? role === 'admin'
                ? 'bg-primary/15 text-primary'
                : 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {t(`account.role.${role}`).toUpperCase()}
        </button>
      ))}
    </div>
  )
}

function RoleChip({ role }: { role: UserRole }) {
  const { t } = useI18n()
  return (
    <span
      className={cn(
        'inline-flex rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider',
        role === 'admin'
          ? 'bg-primary/15 text-primary'
          : 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
      )}
    >
      {t(`account.role.${role}`).toUpperCase()}
    </span>
  )
}

function StatusChip({ disabled }: { disabled: boolean }) {
  const { t } = useI18n()
  return (
    <span
      className={cn(
        'inline-flex rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider',
        disabled ? 'bg-destructive/15 text-destructive' : 'bg-success/15 text-success',
      )}
    >
      {disabled
        ? t('account.status.disabled').toUpperCase()
        : t('account.status.active').toUpperCase()}
    </span>
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
  const profileUserID = useRef('')
  const [tokenName, setTokenName] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [tokenCopied, setTokenCopied] = useState(false)

  const isAdmin = session?.role === 'admin'
  const formatAccountDate = (value?: string | null) =>
    value ? formatDate(value, true, timezone, timeFormat) : t('account.never')

  useEffect(() => {
    if (!session) return
    if (profileUserID.current === session.user_id) return
    setDisplayName(session.display_name)
    setAvatarKey(session.avatar_key)
    profileUserID.current = session.user_id
  }, [session])

  useEffect(() => {
    if (!session || profileUserID.current !== session.user_id) return undefined
    const nextDisplayName = displayName.trim()
    if (nextDisplayName.length === 0) return undefined
    if (nextDisplayName === session.display_name && avatarKey === session.avatar_key)
      return undefined

    const timer = window.setTimeout(() => {
      updateProfile.mutate({ display_name: nextDisplayName, avatar_key: avatarKey })
    }, 500)
    return () => window.clearTimeout(timer)
  }, [avatarKey, displayName, session?.avatar_key, session?.display_name, session?.user_id])

  const submitToken = (event: FormEvent) => {
    event.preventDefault()
    createToken.mutate(tokenName, {
      onSuccess: (token) => {
        setNewToken(token.token ?? null)
        setTokenName('')
      },
    })
  }

  const copyToken = async () => {
    if (!newToken) return
    await window.navigator.clipboard?.writeText(newToken)
    setTokenCopied(true)
    window.setTimeout(() => setTokenCopied(false), 2000)
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
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_220px]">
          <div className="space-y-4">
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
            <div className="min-h-5">
              {updateProfile.isPending && (
                <span className="text-xs text-muted-foreground">{t('common.saving')}</span>
              )}
              {updateProfile.isSuccess && !updateProfile.isPending && (
                <span className="text-xs text-success">{t('account.saved')}</span>
              )}
              <ErrorText error={updateProfile.error} />
            </div>
          </div>
          <div className="flex justify-center lg:justify-end">
            <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
          </div>
        </div>
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
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 break-all font-mono text-sm text-foreground">
                {newToken}
              </code>
              <button
                type="button"
                onClick={copyToken}
                aria-label={t('account.tokens.copy')}
                className="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors hover:border-primary hover:text-primary"
              >
                {tokenCopied ? <Check size={15} /> : <Copy size={15} />}
              </button>
            </div>
            {tokenCopied && (
              <p className="mt-1 text-xs text-success">{t('account.tokens.copied')}</p>
            )}
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
  const { data: session } = useSession()
  const { data: users = [] } = useAdminUsers(true)
  const createUser = useCreateUser()
  const updateUser = useUpdateAdminUser()
  const setupToken = useCreateSetupToken()
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [role, setRole] = useState<UserRole>('user')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>('default')
  const [visibleSetupToken, setVisibleSetupToken] = useState<string | null>(null)
  const [copiedSetupUserID, setCopiedSetupUserID] = useState<string | null>(null)

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
      onSuccess: async (token) => {
        const link = `/account-setup?token=${token.token}`
        setVisibleSetupToken(link)
        await window.navigator.clipboard?.writeText(link)
        setCopiedSetupUserID(userID)
        window.setTimeout(() => setCopiedSetupUserID(null), 2000)
      },
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
                canChangeRole={account.user_id !== session?.user_id}
                canDisable={account.user_id !== session?.user_id}
                setupCopied={copiedSetupUserID === account.user_id}
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
  canChangeRole,
  canDisable,
  setupCopied,
  updating,
  setupPending,
  onRole,
  onDisabled,
  onSetupToken,
}: {
  account: AccountUser
  canChangeRole: boolean
  canDisable: boolean
  setupCopied: boolean
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
        {canChangeRole ? (
          <RoleButtons value={account.role} onChange={onRole} />
        ) : (
          <RoleChip role={account.role} />
        )}
      </td>
      <td className="py-2 pr-3">
        <StatusChip disabled={disabled} />
      </td>
      <td className="space-x-2 py-2 text-right">
        {setupCopied && (
          <span className="text-xs text-success">{t('account.users.setupLinkCopied')}</span>
        )}
        <button
          type="button"
          disabled={setupPending}
          onClick={onSetupToken}
          className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {t('account.users.generateSetupLink')}
        </button>
        {canDisable && (
          <button
            type="button"
            disabled={updating}
            onClick={() => onDisabled(!disabled)}
            className={cn(
              'rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-50',
              disabled
                ? 'hover:border-success hover:text-success'
                : 'hover:border-destructive hover:text-destructive',
            )}
          >
            {disabled ? t('account.users.enable') : t('account.users.disable')}
          </button>
        )}
      </td>
    </tr>
  )
}
