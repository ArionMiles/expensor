import { Check, Copy, Pencil, Plus, Trash2 } from 'lucide-react'
import {
  FormEvent,
  useEffect,
  useId,
  useRef,
  useState,
  type InputHTMLAttributes,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { useSearchParams } from 'react-router-dom'
import {
  useAccessTokens,
  useAdminUsers,
  useCreateAccessToken,
  useCreateSetupToken,
  useCreateUser,
  useDeleteAdminUser,
  useRevokeAccessToken,
  useSession,
  useUpdateAdminUser,
  useUpdateProfile,
} from '@/api/queries'
import type { AccessToken, AccountUser, AvatarKey, UserRole } from '@/api/types'
import { useDisplay } from '@/contexts/DisplayContext'
import { useI18n } from '@/i18n/I18nProvider'
import { AvatarPicker } from '@/pages/auth/AuthPages'
import { cn, formatDate } from '@/lib/utils'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useCopyTooltip } from '@/hooks/useCopyTooltip'

function TextField({
  label,
  value,
  type = 'text',
  inputMode,
  disabled = false,
  message,
  tone,
  onChange,
}: {
  label: string
  value: string
  type?: string
  inputMode?: InputHTMLAttributes<HTMLInputElement>['inputMode']
  disabled?: boolean
  message?: string
  tone?: 'warning' | 'destructive'
  onChange: (value: string) => void
}) {
  const id = useId()
  const messageId = `${id}-message`

  return (
    <div className="block">
      <label
        htmlFor={id}
        className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground"
      >
        {label}
      </label>
      <input
        id={id}
        type={type}
        value={value}
        disabled={disabled}
        inputMode={inputMode}
        aria-invalid={tone === 'warning' || tone === 'destructive'}
        aria-describedby={message ? messageId : undefined}
        onChange={(event) => onChange(event.currentTarget.value)}
        className={cn(
          'w-full rounded-md border bg-input px-3 py-2 text-sm text-foreground outline-none transition-colors focus:ring-1 disabled:opacity-70',
          tone === 'warning' && 'border-warning focus:border-warning focus:ring-warning/40',
          tone === 'destructive' &&
            'border-destructive focus:border-destructive focus:ring-destructive/40',
          !tone && 'border-border focus:border-primary focus:ring-ring',
        )}
      />
      {message && (
        <p
          id={messageId}
          className={cn(
            'mt-1.5 text-xs',
            tone === 'destructive' ? 'text-destructive' : 'text-warning',
          )}
        >
          {message}
        </p>
      )}
    </div>
  )
}

function Section({
  title,
  action,
  children,
}: {
  title: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="space-y-4 border-t border-border pt-6 first:border-t-0 first:pt-0">
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-sm font-medium text-foreground">{title}</h2>
        {action}
      </div>
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

function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

function userEmailConflictMessage(error: unknown, email: string) {
  if (!(error instanceof Error)) return ''
  const match = /^User (.+) already exists\.$/.exec(error.message)
  if (!match) return ''
  return match[1].toLowerCase() === email.trim().toLowerCase() ? error.message : ''
}

function AccountModal({
  title,
  children,
  footer,
  onClose,
}: {
  title: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
}) {
  const titleId = useId()

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 px-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="w-full max-w-lg rounded-lg border border-border bg-card p-6 shadow-xl"
      >
        <div className="mb-5 flex items-start justify-between gap-4">
          <h2 id={titleId} className="text-sm font-semibold text-foreground">
            {title}
          </h2>
        </div>
        <div className="space-y-4">{children}</div>
        {footer && <div className="mt-6 flex flex-wrap justify-end gap-2">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}

function AccountTable({
  label,
  colgroup,
  headers,
  children,
}: {
  label: string
  colgroup: ReactNode
  headers: string[]
  children: ReactNode
}) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-sm">
      <table aria-label={label} className="w-full min-w-[46rem] table-fixed">
        <colgroup>{colgroup}</colgroup>
        <thead>
          <tr className="border-b border-border bg-secondary/50">
            {headers.map((header, index) => (
              <th
                key={header}
                scope="col"
                className={cn(
                  'px-4 py-3 text-left text-[10px] font-semibold uppercase tracking-wider text-muted-foreground',
                  index === headers.length - 1 && 'text-right',
                )}
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  )
}

function RoleChoice({ value, onChange }: { value: UserRole; onChange: (value: UserRole) => void }) {
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
          {t(`account.roleTitle.${role}`)}
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

function StatusChoice({
  disabled,
  onChange,
}: {
  disabled: boolean
  onChange: (value: boolean) => void
}) {
  const { t } = useI18n()
  return (
    <div className="inline-flex rounded-md border border-border bg-card p-0.5">
      {[
        { disabled: false, label: t('account.status.active') },
        { disabled: true, label: t('account.status.disabled') },
      ].map((option) => (
        <button
          key={option.label}
          type="button"
          onClick={() => onChange(option.disabled)}
          className={cn(
            'rounded px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider transition-colors',
            disabled === option.disabled
              ? option.disabled
                ? 'bg-destructive/15 text-destructive'
                : 'bg-success/15 text-success'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
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
  const [searchParams, setSearchParams] = useSearchParams()
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
  const [creatingToken, setCreatingToken] = useState(false)
  const [newToken, setNewToken] = useState<{ name: string; token: string } | null>(null)
  const [tokenCopied, setTokenCopied] = useState(false)
  const tokenCopiedTimerRef = useRef<number | null>(null)
  const [revokeCandidate, setRevokeCandidate] = useState<AccessToken | null>(null)

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

  useEffect(
    () => () => {
      if (tokenCopiedTimerRef.current !== null) window.clearTimeout(tokenCopiedTimerRef.current)
    },
    [],
  )

  const submitToken = (event: FormEvent) => {
    event.preventDefault()
    createToken.mutate(tokenName, {
      onSuccess: (token) => {
        if (token.token) setNewToken({ name: token.name, token: token.token })
        setTokenName('')
        setCreatingToken(false)
        setTokenCopied(false)
      },
    })
  }

  const openCreateToken = () => {
    createToken.reset()
    setTokenName('')
    setCreatingToken(true)
  }

  const closeCreateToken = () => {
    createToken.reset()
    setTokenName('')
    setCreatingToken(false)
  }

  const clearAccountAction = () => {
    const next = new URLSearchParams(searchParams)
    next.delete('action')
    setSearchParams(next, { replace: true })
  }

  useEffect(() => {
    if (!session || searchParams.get('action') !== 'create-token') return
    openCreateToken()
    clearAccountAction()
  }, [session?.user_id, searchParams, setSearchParams])

  const copyToken = async () => {
    if (!newToken) return
    await window.navigator.clipboard?.writeText(newToken.token)
    setTokenCopied(true)
    if (tokenCopiedTimerRef.current !== null) window.clearTimeout(tokenCopiedTimerRef.current)
    tokenCopiedTimerRef.current = window.setTimeout(() => {
      setTokenCopied(false)
      tokenCopiedTimerRef.current = null
    }, 2000)
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
          <div className="w-full max-w-md space-y-4">
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

      <Section
        title={t('account.tokens.title')}
        action={
          <button
            type="button"
            onClick={openCreateToken}
            className="inline-flex items-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            <Plus size={15} />
            {t('account.tokens.new')}
          </button>
        }
      >
        <AccountTable
          label={t('account.tokens.title')}
          headers={[
            t('account.tokens.columns.name'),
            t('account.tokens.columns.created'),
            t('account.tokens.columns.lastUsed'),
            t('account.tokens.columns.action'),
          ]}
          colgroup={
            <>
              <col />
              <col className="w-48" />
              <col className="w-40" />
              <col className="w-24" />
            </>
          }
        >
          {tokens.map((token) => (
            <tr key={token.id} className="border-b border-border last:border-0 hover:bg-accent/40">
              <td className="px-4 py-3 text-sm font-medium text-foreground">{token.name}</td>
              <td className="px-4 py-3 text-sm text-muted-foreground">
                {formatAccountDate(token.created_at)}
              </td>
              <td className="px-4 py-3 text-sm text-muted-foreground">
                {formatAccountDate(token.last_used_at)}
              </td>
              <td className="px-4 py-3 text-right">
                <button
                  type="button"
                  onClick={() => setRevokeCandidate(token)}
                  aria-label={t('account.tokens.revokeAria', { name: token.name })}
                  className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 size={15} />
                </button>
              </td>
            </tr>
          ))}
          {tokens.length === 0 && (
            <tr>
              <td colSpan={4} className="px-4 py-8 text-center text-xs text-muted-foreground">
                {t('account.tokens.empty')}
              </td>
            </tr>
          )}
        </AccountTable>
      </Section>

      {isAdmin && <AdminUsersSection />}

      {creatingToken && (
        <AccountModal
          title={t('account.tokens.createTitle')}
          onClose={closeCreateToken}
          footer={
            <>
              <button
                type="button"
                onClick={closeCreateToken}
                className="rounded-md px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {t('common.cancel')}
              </button>
              <button
                type="submit"
                form="account-token-form"
                disabled={createToken.isPending || tokenName.trim().length === 0}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {createToken.isPending ? t('account.tokens.creating') : t('account.tokens.create')}
              </button>
            </>
          }
        >
          <form id="account-token-form" onSubmit={submitToken} className="space-y-4">
            <TokenNameField value={tokenName} onChange={setTokenName} />
            <ErrorText error={createToken.error} />
          </form>
        </AccountModal>
      )}

      {newToken && (
        <TokenRevealModal
          token={newToken}
          copied={tokenCopied}
          onCopy={copyToken}
          onClose={() => setNewToken(null)}
        />
      )}

      {revokeCandidate && (
        <ConfirmModal
          title={t('account.tokens.revokeTitle')}
          message={t('account.tokens.revokeMessage', { name: revokeCandidate.name })}
          confirmLabel={t('account.tokens.revoke')}
          variant="destructive"
          onCancel={() => setRevokeCandidate(null)}
          onConfirm={() => {
            revokeToken.mutate(revokeCandidate.id)
            setRevokeCandidate(null)
          }}
        />
      )}
    </div>
  )
}

function TokenNameField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useI18n()

  return (
    <div>
      <TextField label={t('account.tokens.name')} value={value} onChange={onChange} />
      <p className="mt-2 text-xs font-medium text-muted-foreground">
        {t('account.tokens.nameHint')}
      </p>
    </div>
  )
}

function TokenRevealModal({
  token,
  copied,
  onCopy,
  onClose,
}: {
  token: { name: string; token: string }
  copied: boolean
  onCopy: () => void
  onClose: () => void
}) {
  const { t } = useI18n()
  const { handlers, tip } = useCopyTooltip(
    copied ? t('account.tokens.copied') : t('account.tokens.copy'),
  )

  return (
    <AccountModal
      title={token.name}
      onClose={onClose}
      footer={
        <button
          type="button"
          onClick={onClose}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {t('common.close')}
        </button>
      }
    >
      <p className="text-xs text-muted-foreground">{t('account.tokens.copyOnce')}</p>
      <div className="flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
        <code className="min-w-0 flex-1 break-all font-mono text-sm text-foreground">
          {token.token}
        </code>
        <button
          type="button"
          onClick={onCopy}
          aria-label={t('account.tokens.copy')}
          className="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          {...handlers}
        >
          {copied ? <Check size={15} /> : <Copy size={15} />}
        </button>
        {tip}
      </div>
    </AccountModal>
  )
}

function AdminUsersSection() {
  const { t } = useI18n()
  const [searchParams, setSearchParams] = useSearchParams()
  const { data: session } = useSession()
  const { data: users = [] } = useAdminUsers(true)
  const createUser = useCreateUser()
  const updateUser = useUpdateAdminUser()
  const deleteUser = useDeleteAdminUser()
  const setupToken = useCreateSetupToken()
  const [creatingUser, setCreatingUser] = useState(false)
  const [editingUser, setEditingUser] = useState<AccountUser | null>(null)
  const [deleteCandidate, setDeleteCandidate] = useState<AccountUser | null>(null)
  const [copiedSetupUserID, setCopiedSetupUserID] = useState<string | null>(null)
  const [createdInvite, setCreatedInvite] = useState<{ email: string; link: string } | null>(null)
  const [createdInviteCopied, setCreatedInviteCopied] = useState(false)
  const copiedSetupTimerRef = useRef<number | null>(null)
  const createdInviteCopiedTimerRef = useRef<number | null>(null)

  useEffect(
    () => () => {
      if (copiedSetupTimerRef.current !== null) window.clearTimeout(copiedSetupTimerRef.current)
      if (createdInviteCopiedTimerRef.current !== null)
        window.clearTimeout(createdInviteCopiedTimerRef.current)
    },
    [],
  )

  const setupLinkFromToken = (token: string) =>
    new URL(`/account-setup?token=${encodeURIComponent(token)}`, window.location.origin).toString()

  const generateSetupToken = (userID: string) => {
    setupToken.mutate(userID, {
      onSuccess: async (token) => {
        const link = setupLinkFromToken(token.token)
        await window.navigator.clipboard?.writeText(link)
        setCopiedSetupUserID(userID)
        if (copiedSetupTimerRef.current !== null) window.clearTimeout(copiedSetupTimerRef.current)
        copiedSetupTimerRef.current = window.setTimeout(() => {
          setCopiedSetupUserID(null)
          copiedSetupTimerRef.current = null
        }, 2000)
      },
    })
  }

  const openCreateUser = () => {
    createUser.reset()
    setupToken.reset()
    setCreatedInvite(null)
    setCreatedInviteCopied(false)
    setCreatingUser(true)
  }

  const closeCreateUser = () => {
    createUser.reset()
    setupToken.reset()
    setCreatedInvite(null)
    setCreatedInviteCopied(false)
    setCreatingUser(false)
  }

  const clearAccountAction = () => {
    const next = new URLSearchParams(searchParams)
    next.delete('action')
    setSearchParams(next, { replace: true })
  }

  useEffect(() => {
    if (searchParams.get('action') !== 'create-user') return
    openCreateUser()
    clearAccountAction()
  }, [searchParams, setSearchParams])

  const copyCreatedInvite = async () => {
    if (!createdInvite) return
    await window.navigator.clipboard?.writeText(createdInvite.link)
    setCreatedInviteCopied(true)
    if (createdInviteCopiedTimerRef.current !== null)
      window.clearTimeout(createdInviteCopiedTimerRef.current)
    createdInviteCopiedTimerRef.current = window.setTimeout(() => {
      setCreatedInviteCopied(false)
      createdInviteCopiedTimerRef.current = null
    }, 2000)
  }

  const openEditUser = (account: AccountUser) => {
    updateUser.reset()
    setEditingUser(account)
  }

  const closeEditUser = () => {
    updateUser.reset()
    setEditingUser(null)
  }

  return (
    <Section
      title={t('account.users.title')}
      action={
        <button
          type="button"
          onClick={openCreateUser}
          className="inline-flex items-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          <Plus size={15} />
          {t('account.users.new')}
        </button>
      }
    >
      <AccountTable
        label={t('account.users.title')}
        headers={[
          t('account.users.columns.user'),
          t('account.users.columns.role'),
          t('account.users.columns.status'),
          t('account.users.columns.actions'),
        ]}
        colgroup={
          <>
            <col />
            <col className="w-36" />
            <col className="w-36" />
            <col className="w-32" />
          </>
        }
      >
        {users.map((account) => (
          <AdminUserRow
            key={account.user_id}
            account={account}
            canEdit={account.user_id !== session?.user_id}
            setupCopied={copiedSetupUserID === account.user_id}
            setupPending={setupToken.isPending}
            onEdit={() => openEditUser(account)}
            onSetupToken={() => generateSetupToken(account.user_id)}
          />
        ))}
        {users.length === 0 && (
          <tr>
            <td colSpan={4} className="px-4 py-8 text-center text-xs text-muted-foreground">
              {t('account.users.empty')}
            </td>
          </tr>
        )}
      </AccountTable>
      <ErrorText error={setupToken.error} />

      {creatingUser && !createdInvite && (
        <UserFormModal
          mode="create"
          pending={createUser.isPending || setupToken.isPending}
          error={createUser.error ?? setupToken.error}
          onClose={closeCreateUser}
          onSubmit={(input) =>
            createUser.mutate(
              {
                email: input.email,
                role: input.role,
              },
              {
                onSuccess: (created) => {
                  setupToken.mutate(created.user_id, {
                    onSuccess: (token) =>
                      setCreatedInvite({
                        email: created.email,
                        link: setupLinkFromToken(token.token),
                      }),
                  })
                },
              },
            )
          }
        />
      )}

      {createdInvite && (
        <SetupLinkRevealModal
          email={createdInvite.email}
          link={createdInvite.link}
          copied={createdInviteCopied}
          onCopy={copyCreatedInvite}
          onClose={closeCreateUser}
        />
      )}

      {editingUser && (
        <UserFormModal
          mode="edit"
          user={editingUser}
          pending={updateUser.isPending}
          error={updateUser.error}
          onClose={closeEditUser}
          onDelete={() => setDeleteCandidate(editingUser)}
          onSubmit={(input) =>
            updateUser.mutate(
              {
                id: editingUser.user_id,
                patch: { role: input.role, disabled: input.disabled },
              },
              { onSuccess: closeEditUser },
            )
          }
        />
      )}

      {deleteCandidate && (
        <ConfirmModal
          title={t('account.users.deleteTitle')}
          message={
            <div className="space-y-2">
              <p>
                {t('account.users.deleteMessage', { name: displayNameForUser(deleteCandidate) })}
              </p>
              <ErrorText error={deleteUser.error} />
            </div>
          }
          confirmLabel={deleteUser.isPending ? t('account.users.deleting') : t('common.delete')}
          variant="destructive"
          confirmDisabled={deleteUser.isPending}
          onCancel={() => setDeleteCandidate(null)}
          onConfirm={() => {
            deleteUser.mutate(deleteCandidate.user_id, {
              onSuccess: () => {
                setDeleteCandidate(null)
                setEditingUser(null)
              },
            })
          }}
        />
      )}
    </Section>
  )
}

function UserFormModal({
  mode,
  user,
  pending,
  error,
  onClose,
  onSubmit,
  onDelete,
}: {
  mode: 'create' | 'edit'
  user?: AccountUser
  pending: boolean
  error?: unknown
  onClose: () => void
  onSubmit: (input: { email: string; role: UserRole; disabled: boolean }) => void
  onDelete?: () => void
}) {
  const { t } = useI18n()
  const [email, setEmail] = useState(user?.email ?? '')
  const [emailTouched, setEmailTouched] = useState(false)
  const [role, setRole] = useState<UserRole>(user?.role ?? 'user')
  const [disabledAccount, setDisabledAccount] = useState(!!user?.disabled_at)
  const title =
    mode === 'create'
      ? t('account.users.createTitle')
      : t('account.users.edit', { name: displayNameForUser(user) })
  const disabled = pending || (mode === 'create' && !email.trim())
  const showEmailWarning =
    mode === 'create' && emailTouched && email.trim().length > 0 && !isValidEmail(email)
  const emailConflictMessage = mode === 'create' ? userEmailConflictMessage(error, email) : ''
  const emailMessage = showEmailWarning
    ? t('auth.validation.email')
    : emailConflictMessage || undefined
  const emailTone = showEmailWarning ? 'warning' : emailConflictMessage ? 'destructive' : undefined

  const submit = (event: FormEvent) => {
    event.preventDefault()
    setEmailTouched(true)
    if (mode === 'create' && !isValidEmail(email)) return
    onSubmit({
      email: email.trim(),
      role,
      disabled: disabledAccount,
    })
  }

  return (
    <AccountModal
      title={title}
      onClose={onClose}
      footer={
        <>
          {mode === 'edit' && user && onDelete && (
            <button
              type="button"
              onClick={onDelete}
              className="mr-auto rounded-md px-4 py-2 text-sm text-destructive transition-colors hover:bg-destructive/10"
            >
              {t('account.users.deleteUser')}
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {t('common.cancel')}
          </button>
          <button
            type="submit"
            form="account-user-form"
            disabled={disabled}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {mode === 'create'
              ? pending
                ? t('account.users.creating')
                : t('account.users.create')
              : t('account.users.saveChanges')}
          </button>
        </>
      }
    >
      <form id="account-user-form" onSubmit={submit} className="space-y-4">
        <TextField
          label={t('account.users.email')}
          type="text"
          inputMode="email"
          value={email}
          disabled={mode === 'edit'}
          message={emailMessage}
          tone={emailTone}
          onChange={setEmail}
        />
        {mode === 'edit' && (
          <TextField
            label={t('account.users.displayName')}
            value={user?.display_name ?? ''}
            disabled
            onChange={() => {}}
          />
        )}
        <div>
          <span className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
            {t('account.users.columns.role')}
          </span>
          <RoleChoice value={role} onChange={setRole} />
        </div>
        {mode === 'edit' && (
          <div>
            <span className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
              {t('account.users.columns.status')}
            </span>
            <StatusChoice disabled={disabledAccount} onChange={setDisabledAccount} />
          </div>
        )}
        {!emailConflictMessage && <ErrorText error={error} />}
      </form>
    </AccountModal>
  )
}

function SetupLinkRevealModal({
  email,
  link,
  copied,
  onCopy,
  onClose,
}: {
  email: string
  link: string
  copied: boolean
  onCopy: () => void
  onClose: () => void
}) {
  const { t } = useI18n()
  const { handlers, tip } = useCopyTooltip(
    copied ? t('account.users.copied') : t('account.users.copySetupLink'),
  )

  return (
    <AccountModal
      title={t('account.users.inviteTitle', { email })}
      onClose={onClose}
      footer={
        <button
          type="button"
          onClick={onClose}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {t('common.close')}
        </button>
      }
    >
      <p className="text-xs text-muted-foreground">{t('account.users.inviteSummary')}</p>
      <div className="flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
        <code className="min-w-0 flex-1 break-all font-mono text-sm text-foreground">{link}</code>
        <button
          type="button"
          onClick={onCopy}
          aria-label={t('account.users.copySetupLink')}
          className="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          {...handlers}
        >
          {copied ? <Check size={15} /> : <Copy size={15} />}
        </button>
        {tip}
      </div>
    </AccountModal>
  )
}

function displayNameForUser(user?: AccountUser | null) {
  return user?.display_name?.trim() || user?.email || ''
}

function AdminUserRow({
  account,
  canEdit,
  setupCopied,
  setupPending,
  onEdit,
  onSetupToken,
}: {
  account: AccountUser
  canEdit: boolean
  setupCopied: boolean
  setupPending: boolean
  onEdit: () => void
  onSetupToken: () => void
}) {
  const { t } = useI18n()
  const { handlers, tip } = useCopyTooltip(
    setupCopied ? t('account.users.copied') : t('account.users.copySetupLink'),
  )
  const disabled = !!account.disabled_at
  const displayName = displayNameForUser(account)
  return (
    <tr className="border-b border-border last:border-0 hover:bg-accent/40">
      <td className="px-4 py-3">
        <div className="text-foreground">{displayName}</div>
        <div className="text-xs text-muted-foreground">{account.email}</div>
      </td>
      <td className="px-4 py-3">
        <RoleChip role={account.role} />
      </td>
      <td className="px-4 py-3">
        <StatusChip disabled={disabled} />
      </td>
      <td className="px-4 py-3 text-right">
        {account.setup_pending && (
          <>
            <button
              type="button"
              disabled={setupPending}
              onClick={onSetupToken}
              aria-label={t('account.users.copySetupLink')}
              className="mr-2 inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
              {...handlers}
            >
              <Copy size={15} />
            </button>
            {tip}
          </>
        )}
        {canEdit && (
          <button
            type="button"
            onClick={onEdit}
            aria-label={t('account.users.editAria', { name: displayName })}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <Pencil size={15} />
          </button>
        )}
      </td>
    </tr>
  )
}
