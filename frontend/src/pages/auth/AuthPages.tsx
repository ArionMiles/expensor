import { FormEvent, useMemo, useState } from 'react'
import { Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import {
  useBootstrapAdmin,
  useBootstrapStatus,
  useCompleteAccountSetup,
  useLogin,
  useSession,
} from '@/api/queries'
import type { AvatarKey } from '@/api/types'
import { avatarCatalog } from '@/assets/avatars'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'

function AuthSurface({ children }: { children: React.ReactNode }) {
  const { t } = useI18n()

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto grid min-h-screen w-full max-w-6xl items-center gap-10 px-6 py-8 lg:grid-cols-[1.05fr_0.95fr] lg:px-10">
        <section className="hidden min-h-[620px] flex-col justify-between overflow-hidden rounded-lg border border-border bg-card p-8 shadow-sm lg:flex">
          <div>
            <div className="mb-12 flex items-center gap-3">
              <span className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-background">
                <img
                  src="/brand/expensor-wallet.svg"
                  alt=""
                  aria-hidden="true"
                  className="h-6 w-6"
                />
              </span>
              <img src="/brand/expensor-wordmark.svg" alt="Expensor" className="h-8 w-auto" />
            </div>
            <p className="mb-3 text-xs font-medium uppercase tracking-wider text-primary">
              {t('auth.surface.eyebrow')}
            </p>
            <h2 className="max-w-md text-4xl font-semibold leading-tight text-foreground">
              {t('auth.surface.title')}
            </h2>
            <p className="mt-4 max-w-md text-sm leading-6 text-muted-foreground">
              {t('auth.surface.summary')}
            </p>
          </div>

          <div className="space-y-6">
            <div className="rounded-lg border border-border bg-background p-5">
              <div className="mb-5 flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-foreground">
                    {t('auth.surface.previewTitle')}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t('auth.surface.previewStatus')}
                  </p>
                </div>
                <span className="h-2.5 w-2.5 rounded-full bg-primary" />
              </div>
              <div className="grid gap-3">
                {[
                  [t('auth.surface.previewAccount'), t('auth.surface.previewAccountStatus')],
                  [t('auth.surface.previewReaders'), t('auth.surface.previewReadersStatus')],
                  [t('auth.surface.previewTokens'), t('auth.surface.previewTokensStatus')],
                ].map(([label, status]) => (
                  <div
                    key={label}
                    className="flex items-center justify-between gap-4 rounded-md border border-border bg-card px-3 py-3"
                  >
                    <p className="text-sm font-medium text-foreground">{label}</p>
                    <p className="text-right text-xs text-muted-foreground">{status}</p>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid gap-3 text-sm text-muted-foreground">
              {[
                t('auth.surface.localData'),
                t('auth.surface.accountControl'),
                t('auth.surface.readerReady'),
              ].map((item) => (
                <div key={item} className="flex items-center gap-3">
                  <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  <span>{item}</span>
                </div>
              ))}
            </div>
          </div>
        </section>

        <div className="mx-auto w-full max-w-md">
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <span className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-card">
              <img src="/brand/expensor-wallet.svg" alt="" aria-hidden="true" className="h-6 w-6" />
            </span>
            <img src="/brand/expensor-wordmark.svg" alt="Expensor" className="h-8 w-auto" />
          </div>
          <div className="rounded-lg border border-border bg-card px-5 py-6 shadow-sm sm:px-7">
            {children}
          </div>
        </div>
      </div>
    </main>
  )
}

function Field({
  label,
  type = 'text',
  value,
  autoComplete,
  onChange,
}: {
  label: string
  type?: string
  value: string
  autoComplete?: string
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
        autoComplete={autoComplete}
        onChange={(event) => onChange(event.currentTarget.value)}
        className="h-11 w-full rounded-md border border-border bg-background px-3 text-sm text-foreground outline-none transition-colors focus:border-primary focus:ring-1 focus:ring-ring"
      />
    </label>
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

function nextPathFromSearch(search: URLSearchParams) {
  const next = search.get('next')
  return next?.startsWith('/') ? next : '/'
}

export function LoginPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const login = useLogin()

  const submit = (event: FormEvent) => {
    event.preventDefault()
    login.mutate(
      { email, password },
      {
        onSuccess: () => navigate(nextPathFromSearch(searchParams), { replace: true }),
      },
    )
  }

  return (
    <AuthSurface>
      <form onSubmit={submit} className="space-y-5">
        <div>
          <h1 className="text-xl font-semibold text-foreground">{t('auth.login.title')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t('auth.login.summary')}</p>
        </div>
        <Field
          label={t('account.email')}
          type="email"
          autoComplete="email"
          value={email}
          onChange={setEmail}
        />
        <Field
          label={t('account.password')}
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={setPassword}
        />
        <ErrorText error={login.error} />
        <button
          type="submit"
          disabled={login.isPending}
          className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {login.isPending ? t('auth.login.submitting') : t('auth.login.submit')}
        </button>
      </form>
    </AuthSurface>
  )
}

export function BootstrapPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>('default')
  const bootstrap = useBootstrapAdmin()

  const submit = (event: FormEvent) => {
    event.preventDefault()
    bootstrap.mutate(
      {
        email,
        display_name: displayName,
        password,
        avatar_key: avatarKey,
      },
      {
        onSuccess: () => navigate('/setup', { replace: true }),
      },
    )
  }

  return (
    <AuthSurface>
      <form onSubmit={submit} className="space-y-5">
        <div>
          <h1 className="text-xl font-semibold text-foreground">{t('auth.bootstrap.title')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t('auth.bootstrap.summary')}</p>
        </div>
        <Field
          label={t('account.email')}
          type="email"
          autoComplete="email"
          value={email}
          onChange={setEmail}
        />
        <Field
          label={t('account.displayName')}
          autoComplete="name"
          value={displayName}
          onChange={setDisplayName}
        />
        <Field
          label={t('account.password')}
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={setPassword}
        />
        <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
        <ErrorText error={bootstrap.error} />
        <button
          type="submit"
          disabled={bootstrap.isPending}
          className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {bootstrap.isPending ? t('auth.bootstrap.submitting') : t('auth.bootstrap.submit')}
        </button>
      </form>
    </AuthSurface>
  )
}

export function AccountSetupPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const [password, setPassword] = useState('')
  const setup = useCompleteAccountSetup()

  const submit = (event: FormEvent) => {
    event.preventDefault()
    setup.mutate(
      { token, password },
      {
        onSuccess: () => navigate('/', { replace: true }),
      },
    )
  }

  return (
    <AuthSurface>
      <form onSubmit={submit} className="space-y-5">
        <div>
          <h1 className="text-xl font-semibold text-foreground">{t('auth.accountSetup.title')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t('auth.accountSetup.summary')}</p>
        </div>
        <Field
          label={t('account.password')}
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={setPassword}
        />
        {!token && (
          <p className="text-sm text-destructive">{t('auth.accountSetup.missingToken')}</p>
        )}
        <ErrorText error={setup.error} />
        <button
          type="submit"
          disabled={setup.isPending || token.length === 0}
          className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {setup.isPending ? t('auth.accountSetup.submitting') : t('auth.accountSetup.submit')}
        </button>
      </form>
    </AuthSurface>
  )
}

export function AvatarPicker({
  value,
  onChange,
}: {
  value: AvatarKey
  onChange: (value: AvatarKey) => void
}) {
  const { t } = useI18n()

  return (
    <div>
      <p className="mb-1.5 text-xs uppercase tracking-wider text-muted-foreground">
        {t('account.avatar')}
      </p>
      <div className="grid grid-cols-3 gap-2">
        {avatarCatalog.map((avatar) => (
          <button
            key={avatar.key}
            type="button"
            onClick={() => onChange(avatar.key)}
            className={cn(
              'h-10 rounded-md border px-3 text-sm transition-colors',
              value === avatar.key
                ? 'border-primary bg-accent text-accent-foreground'
                : 'border-border text-muted-foreground hover:text-foreground',
            )}
          >
            {avatar.label}
          </button>
        ))}
      </div>
    </div>
  )
}

export function AuthGate({ children }: { children: React.ReactNode }) {
  const { t } = useI18n()
  const location = useLocation()
  const publicPath = useMemo(
    () => ['/login', '/bootstrap', '/account-setup'].includes(location.pathname),
    [location.pathname],
  )
  const bootstrap = useBootstrapStatus()
  const session = useSession(!publicPath && bootstrap.data?.required === false)

  if (
    bootstrap.isLoading ||
    (!publicPath && bootstrap.data?.required === false && session.isLoading)
  ) {
    return (
      <div className="flex h-screen items-center justify-center">
        <span className="font-mono text-xs text-muted-foreground">{t('common.loading')}</span>
      </div>
    )
  }

  if (bootstrap.data?.required && location.pathname !== '/bootstrap') {
    return <Navigate to="/bootstrap" replace />
  }

  if (!publicPath && bootstrap.data?.required === false && session.isError) {
    const next = `${location.pathname}${location.search}`
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />
  }

  return <>{children}</>
}
