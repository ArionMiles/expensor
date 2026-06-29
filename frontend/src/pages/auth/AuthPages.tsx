import {
  type FormEvent,
  type InputHTMLAttributes,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react'
import { Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import {
  useBootstrapAdmin,
  useBootstrapStatus,
  useCompleteAccountSetup,
  useLogin,
  useSession,
} from '@/api/queries'
import type { AvatarKey } from '@/api/types'
import { avatarByKey, avatarCatalog } from '@/assets/avatars'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'

function AuthSurface({ children }: { children: React.ReactNode }) {
  const { t } = useI18n()

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto grid min-h-screen w-full max-w-5xl items-center gap-8 px-6 py-8 lg:grid-cols-[1fr_1px_0.9fr] lg:gap-12 lg:px-10">
        <section className="hidden max-w-md lg:block">
          <div>
            <div className="mb-10 flex items-center gap-3">
              <span className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-card">
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
            <h2 className="text-4xl font-semibold leading-tight text-foreground">
              {t('auth.surface.title')}
            </h2>
            <p className="mt-4 text-sm leading-6 text-muted-foreground">
              {t('auth.surface.summary')}
            </p>
          </div>

          <div className="mt-10 space-y-6">
            <div className="rounded-lg border border-border bg-card/70 p-5 shadow-sm">
              <div className="mb-4 flex items-center justify-between">
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
                  [t('auth.surface.previewLocalData'), t('auth.surface.previewLocalDataStatus')],
                ].map(([label, status]) => (
                  <div
                    key={label}
                    className="flex items-center justify-between gap-4 rounded-md border border-border bg-background px-3 py-3"
                  >
                    <p className="text-sm font-medium text-foreground">{label}</p>
                    <p className="text-right text-xs text-muted-foreground">{status}</p>
                  </div>
                ))}
              </div>
            </div>

            <p className="text-sm leading-6 text-muted-foreground">
              {t('auth.surface.readerReady')}
            </p>
          </div>
        </section>

        <div
          aria-hidden="true"
          className="hidden h-[560px] w-px bg-gradient-to-b from-transparent via-border to-transparent lg:block"
        />

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
  inputMode,
  message,
  tone,
  feedbackTestId,
  labelTestId,
  onBlur,
  onChange,
}: {
  label: string
  type?: string
  value: string
  autoComplete?: string
  inputMode?: InputHTMLAttributes<HTMLInputElement>['inputMode']
  message?: string
  tone?: 'warning'
  feedbackTestId?: string
  labelTestId?: string
  onBlur?: () => void
  onChange: (value: string) => void
}) {
  const id = useId()
  const messageId = `${id}-message`
  const [focused, setFocused] = useState(false)
  const active = focused || value.length > 0

  return (
    <label className="block">
      <div className="relative">
        <input
          id={id}
          type={type}
          value={value}
          autoComplete={autoComplete}
          inputMode={inputMode}
          aria-invalid={tone === 'warning'}
          aria-describedby={message ? messageId : undefined}
          onFocus={() => setFocused(true)}
          onBlur={() => {
            setFocused(false)
            onBlur?.()
          }}
          onChange={(event) => onChange(event.currentTarget.value)}
          className={cn(
            'h-14 w-full rounded-md border bg-background px-3 pb-1.5 pt-4 text-sm text-foreground outline-none transition-colors focus:ring-1',
            tone === 'warning'
              ? 'border-warning focus:border-warning focus:ring-warning/40'
              : 'border-border focus:border-primary focus:ring-ring',
          )}
        />
        <span
          data-testid={labelTestId}
          className={cn(
            'pointer-events-none absolute left-3 uppercase tracking-wider transition-all duration-200 ease-out',
            active ? 'top-1.5 translate-y-0 text-[10px]' : 'top-1/2 -translate-y-1/2 text-sm',
            tone === 'warning' ? 'text-warning' : 'text-muted-foreground',
          )}
        >
          {label}
        </span>
      </div>
      <p
        id={messageId}
        data-testid={feedbackTestId}
        aria-hidden={!message}
        className={cn(
          'mt-1.5 min-h-5 text-xs transition-all duration-200',
          tone === 'warning' ? 'text-warning opacity-100' : 'text-muted-foreground opacity-0',
        )}
      >
        {message || ' '}
      </p>
    </label>
  )
}

function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

function passwordStrength(value: string): 'weak' | 'good' | 'strong' {
  if (
    value.length >= 12 &&
    /[A-Z]/.test(value) &&
    /[a-z]/.test(value) &&
    /\d/.test(value) &&
    /[^\w\s]/.test(value)
  ) {
    return 'strong'
  }
  if (value.length >= 12) return 'good'
  return 'weak'
}

function PasswordStrength({ password }: { password: string }) {
  const { t } = useI18n()
  const strength = passwordStrength(password)
  const width = strength === 'strong' ? 'w-full' : strength === 'good' ? 'w-2/3' : 'w-1/3'
  const tone =
    strength === 'strong' ? 'bg-success' : strength === 'good' ? 'bg-primary' : 'bg-warning'
  const label = {
    weak: t('auth.passwordStrength.weak'),
    good: t('auth.passwordStrength.good'),
    strong: t('auth.passwordStrength.strong'),
  }[strength]
  const missingHints = [
    { met: password.length >= 12, label: t('auth.validation.passwordLength') },
    { met: /[A-Z]/.test(password), label: t('auth.passwordHint.uppercase') },
    { met: /[a-z]/.test(password), label: t('auth.passwordHint.lowercase') },
    { met: /\d/.test(password), label: t('auth.passwordHint.number') },
    { met: /[^\w\s]/.test(password), label: t('auth.passwordHint.symbol') },
  ].filter((hint) => password.length > 0 && !hint.met)

  return (
    <div data-testid="password-strength-feedback" className="min-h-16 space-y-2">
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          data-testid="password-strength-meter"
          className={cn(
            'h-full rounded-full transition-all duration-300 ease-out',
            password ? width : 'w-0',
            tone,
          )}
        />
      </div>
      <div className="flex items-center justify-between gap-3 text-xs">
        <p
          className={cn(
            'transition-all duration-200',
            password ? 'opacity-100' : 'opacity-0',
            strength === 'weak' ? 'text-warning' : 'text-muted-foreground',
          )}
          aria-hidden={!password}
        >
          {t('auth.passwordStrength.label')}: {label}
        </p>
      </div>
      <div className="flex min-h-6 flex-wrap gap-1.5" aria-hidden={!password}>
        {missingHints.map((hint) => (
          <span
            key={hint.label}
            className="rounded-full border border-warning/30 bg-warning/10 px-2 py-0.5 text-xs text-warning transition-opacity duration-200"
          >
            {hint.label}
          </span>
        ))}
      </div>
    </div>
  )
}

function isBootstrapFormValid(email: string, displayName: string, password: string) {
  return isValidEmail(email) && displayName.trim().length > 0 && password.length >= 12
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
  const [emailTouched, setEmailTouched] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [avatarKey, setAvatarKey] = useState<AvatarKey>('default')
  const bootstrap = useBootstrapAdmin()
  const showEmailWarning = emailTouched && email.length > 0 && !isValidEmail(email)
  const formValid = isBootstrapFormValid(email, displayName, password)

  const submit = (event: FormEvent) => {
    event.preventDefault()
    setEmailTouched(true)
    if (!formValid) return
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
      <form onSubmit={submit} className="space-y-5" noValidate>
        <div>
          <h1 className="text-xl font-semibold text-foreground">{t('auth.bootstrap.title')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t('auth.bootstrap.summary')}</p>
        </div>
        <AvatarPicker value={avatarKey} onChange={setAvatarKey} />
        <Field
          label={t('account.email')}
          type="text"
          autoComplete="email"
          inputMode="email"
          value={email}
          message={showEmailWarning ? t('auth.validation.email') : undefined}
          tone={showEmailWarning ? 'warning' : undefined}
          feedbackTestId="email-feedback"
          labelTestId="email-floating-label"
          onBlur={() => setEmailTouched(true)}
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
        <PasswordStrength password={password} />
        <ErrorText error={bootstrap.error} />
        <button
          type="submit"
          disabled={bootstrap.isPending || !formValid}
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
  const passwordValid = password.length >= 12

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!passwordValid) return
    setup.mutate(
      { token, password },
      {
        onSuccess: () => navigate('/', { replace: true }),
      },
    )
  }

  return (
    <AuthSurface>
      <form onSubmit={submit} className="space-y-5" noValidate>
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
        <PasswordStrength password={password} />
        {!token && (
          <p className="text-sm text-destructive">{t('auth.accountSetup.missingToken')}</p>
        )}
        <ErrorText error={setup.error} />
        <button
          type="submit"
          disabled={setup.isPending || token.length === 0 || !passwordValid}
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
  const selectedAvatar = avatarCatalog.find((avatar) => avatar.key === value) ?? avatarCatalog[0]
  const [expanded, setExpanded] = useState(false)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const buttonRefs = useRef<Record<AvatarKey, HTMLButtonElement | null>>({
    default: null,
    ledger: null,
    wallet: null,
  })

  useEffect(() => {
    if (!expanded) return
    const selectedButton = buttonRefs.current[value]
    selectedButton?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest', inline: 'center' })
  }, [expanded, value])

  useEffect(() => {
    if (!expanded) return undefined
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setExpanded(false)
    }
    document.addEventListener('pointerdown', closeOnOutsidePointer)
    return () => document.removeEventListener('pointerdown', closeOnOutsidePointer)
  }, [expanded])

  const visibleAvatars = expanded ? avatarCatalog : [selectedAvatar]

  return (
    <div
      ref={rootRef}
      className="flex h-24 items-center justify-center"
      aria-label={t('account.avatar')}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setExpanded(false)
      }}
    >
      <div
        className={cn(
          'group relative flex h-24 items-center justify-center gap-3 overflow-hidden rounded-lg border border-border bg-background px-4 transition-all duration-200 ease-out',
          expanded ? 'w-full' : 'w-28 focus-within:border-primary/60 hover:border-primary/60',
        )}
      >
        {!expanded && (
          <>
            <span className="absolute left-3 h-2 w-2 rounded-full bg-primary/25 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100" />
            <span className="absolute right-3 h-2 w-2 rounded-full bg-primary/25 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100" />
          </>
        )}
        {visibleAvatars.map((avatar) => {
          const selected = avatar.key === value
          return (
            <button
              key={avatar.key}
              ref={(button) => {
                buttonRefs.current[avatar.key] = button
              }}
              type="button"
              data-testid={`avatar-option-${avatar.key}`}
              aria-label={t('auth.avatar.option', { label: avatar.label })}
              aria-pressed={selected}
              onClick={() => {
                if (selected && !expanded) {
                  setExpanded(true)
                  return
                }
                onChange(avatar.key)
                setExpanded(false)
              }}
              className={cn(
                'flex shrink-0 items-center justify-center rounded-full border bg-background p-2 shadow-sm transition-all duration-200 ease-out focus:outline-none focus:ring-2 focus:ring-ring',
                selected
                  ? 'h-20 w-20 scale-100 border-primary ring-2 ring-primary/25'
                  : 'h-16 w-16 scale-95 border-border opacity-80 hover:scale-100 hover:border-primary hover:opacity-100',
              )}
            >
              <span
                aria-hidden="true"
                className="block h-full w-full [&_svg]:h-full [&_svg]:w-full"
                dangerouslySetInnerHTML={{ __html: avatarByKey[avatar.key] }}
              />
            </button>
          )
        })}
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
