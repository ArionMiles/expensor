import { Check, Copy, Pencil, Plus, Trash2 } from 'lucide-react'
import {
  FormEvent,
  useEffect,
  useId,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
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
import type { AccessToken, AccountUser, AvatarKey, UserRole } from '@/api/types'
import { useDisplay } from '@/contexts/DisplayContext'
import { useI18n } from '@/i18n/I18nProvider'
import { AvatarPicker } from '@/pages/auth/AuthPages'
import { cn, formatDate } from '@/lib/utils'
import { ConfirmModal } from '@/components/ConfirmModal'

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

function useCopyTooltip(initialLabel: string) {
  const [tooltip, setTooltip] = useState<{ label: string; x: number; y: number } | null>(null)
  const labelRef = useRef(initialLabel)

  useEffect(() => {
    labelRef.current = initialLabel
    setTooltip((current) => (current ? { ...current, label: initialLabel } : current))
  }, [initialLabel])

  const handlers = {
    onMouseEnter: (event: MouseEvent<Element>) => {
      const rect = event.currentTarget.getBoundingClientRect()
      setTooltip({
        label: labelRef.current,
        x: rect.left + rect.width / 2,
        y: rect.bottom + 6,
      })
    },
    onMouseLeave: () => setTooltip(null),
  }

  const tip =
    tooltip &&
    createPortal(
      <div
        className="pointer-events-none fixed z-50 -translate-x-1/2 rounded bg-foreground px-2 py-1 text-xs text-background shadow"
        style={{ left: tooltip.x, top: tooltip.y }}
      >
        {tooltip.label}
      </div>,
      document.body,
    )

  return { handlers, tip }
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
        setTokenCopied(false)
      },
    })
  }

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
          <div className="space-y-4">
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

      {newToken && (
        <AccountModal
          title={newToken.name}
          onClose={() => setNewToken(null)}
          footer={
            <button
              type="button"
              onClick={() => setNewToken(null)}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
            >
              {t('common.close')}
            </button>
          }
        >
          <p className="text-xs text-muted-foreground">{t('account.tokens.copyOnce')}</p>
          <div className="flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
            <code className="min-w-0 flex-1 break-all font-mono text-sm text-foreground">
              {newToken.token}
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
          <div className="min-h-4">
            {tokenCopied && <p className="text-xs text-success">{t('account.tokens.copied')}</p>}
          </div>
        </AccountModal>
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

function AdminUsersSection() {
  const { t } = useI18n()
  const { data: session } = useSession()
  const { data: users = [] } = useAdminUsers(true)
  const createUser = useCreateUser()
  const updateUser = useUpdateAdminUser()
  const setupToken = useCreateSetupToken()
  const [creatingUser, setCreatingUser] = useState(false)
  const [editingUser, setEditingUser] = useState<AccountUser | null>(null)
  const [disableCandidate, setDisableCandidate] = useState<AccountUser | null>(null)
  const [copiedSetupUserID, setCopiedSetupUserID] = useState<string | null>(null)
  const copiedSetupTimerRef = useRef<number | null>(null)

  useEffect(
    () => () => {
      if (copiedSetupTimerRef.current !== null) window.clearTimeout(copiedSetupTimerRef.current)
    },
    [],
  )

  const generateSetupToken = (userID: string) => {
    setupToken.mutate(userID, {
      onSuccess: async (token) => {
        const link = `/account-setup?token=${token.token}`
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

  return (
    <Section
      title={t('account.users.title')}
      action={
        <button
          type="button"
          onClick={() => setCreatingUser(true)}
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
            onEdit={() => setEditingUser(account)}
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
      <ErrorText error={updateUser.error ?? setupToken.error ?? createUser.error} />

      {creatingUser && (
        <UserFormModal
          mode="create"
          pending={createUser.isPending}
          onClose={() => setCreatingUser(false)}
          onSubmit={(input) =>
            createUser.mutate(input, {
              onSuccess: () => setCreatingUser(false),
            })
          }
        />
      )}

      {editingUser && (
        <UserFormModal
          mode="edit"
          user={editingUser}
          pending={updateUser.isPending}
          onClose={() => setEditingUser(null)}
          onDisable={() => setDisableCandidate(editingUser)}
          onEnable={() =>
            updateUser.mutate(
              { id: editingUser.user_id, patch: { disabled: false } },
              { onSuccess: () => setEditingUser(null) },
            )
          }
          onSubmit={(input) =>
            updateUser.mutate(
              { id: editingUser.user_id, patch: { role: input.role } },
              { onSuccess: () => setEditingUser(null) },
            )
          }
        />
      )}

      {disableCandidate && (
        <ConfirmModal
          title={t('account.users.disableTitle')}
          message={t('account.users.disableMessage', { name: disableCandidate.display_name })}
          confirmLabel={t('account.users.disable')}
          variant="destructive"
          onCancel={() => setDisableCandidate(null)}
          onConfirm={() => {
            updateUser.mutate(
              { id: disableCandidate.user_id, patch: { disabled: true } },
              {
                onSuccess: () => {
                  setDisableCandidate(null)
                  setEditingUser(null)
                },
              },
            )
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
  onClose,
  onSubmit,
  onDisable,
  onEnable,
}: {
  mode: 'create' | 'edit'
  user?: AccountUser
  pending: boolean
  onClose: () => void
  onSubmit: (input: {
    email: string
    display_name: string
    role: UserRole
    avatar_key: AvatarKey
  }) => void
  onDisable?: () => void
  onEnable?: () => void
}) {
  const { t } = useI18n()
  const [email, setEmail] = useState(user?.email ?? '')
  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [role, setRole] = useState<UserRole>(user?.role ?? 'user')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>(user?.avatar_key ?? 'default')
  const title =
    mode === 'create'
      ? t('account.users.new')
      : t('account.users.edit', { name: user?.display_name ?? '' })
  const disabled = pending || (mode === 'create' && (!email.trim() || !displayName.trim()))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    onSubmit({
      email: email.trim(),
      display_name: displayName.trim(),
      role,
      avatar_key: avatarKey,
    })
  }

  return (
    <AccountModal
      title={title}
      onClose={onClose}
      footer={
        <>
          {mode === 'edit' && user && !user.disabled_at && onDisable && (
            <button
              type="button"
              onClick={onDisable}
              className="mr-auto rounded-md px-4 py-2 text-sm text-destructive transition-colors hover:bg-destructive/10"
            >
              {t('account.users.disableAccount')}
            </button>
          )}
          {mode === 'edit' && user?.disabled_at && onEnable && (
            <button
              type="button"
              onClick={onEnable}
              className="mr-auto rounded-md px-4 py-2 text-sm text-success transition-colors hover:bg-success/10"
            >
              {t('account.users.enableAccount')}
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
          type="email"
          value={email}
          disabled={mode === 'edit'}
          onChange={setEmail}
        />
        <TextField
          label={t('account.users.displayName')}
          value={displayName}
          disabled={mode === 'edit'}
          onChange={setDisplayName}
        />
        <div>
          <span className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
            {t('account.users.columns.role')}
          </span>
          <RoleChoice value={role} onChange={setRole} />
        </div>
        {mode === 'create' && (
          <div className="flex justify-center pt-2">
            <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
          </div>
        )}
      </form>
    </AccountModal>
  )
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
  return (
    <tr className="border-b border-border last:border-0 hover:bg-accent/40">
      <td className="px-4 py-3">
        <div className="text-foreground">{account.display_name}</div>
        <div className="text-xs text-muted-foreground">{account.email}</div>
      </td>
      <td className="px-4 py-3">
        <RoleChip role={account.role} />
      </td>
      <td className="px-4 py-3">
        <StatusChip disabled={disabled} />
      </td>
      <td className="px-4 py-3 text-right">
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
        {canEdit && (
          <button
            type="button"
            onClick={onEdit}
            aria-label={t('account.users.editAria', { name: account.display_name })}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <Pencil size={15} />
          </button>
        )}
      </td>
    </tr>
  )
}
