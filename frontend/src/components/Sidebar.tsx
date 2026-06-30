import {
  ArrowLeftRight,
  CircleAlert,
  EyeOff,
  LayoutDashboard,
  LogOut,
  type LucideIcon,
  PanelLeft,
  Plug,
  ScrollText,
  Settings2,
  Layers,
} from 'lucide-react'
import { NavLink, useNavigate } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { useTooltip } from '@/hooks/useTooltip'
import { ThemeToggle } from './ThemeToggle'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'
import { avatarByKey, isAvatarKey } from '@/assets/avatars'
import { shortcutLabel } from '@/lib/shortcuts'
import { useExtractionDiagnostics, useLogout, useReaderStatus, useSession } from '@/api/queries'

interface NavItemDef {
  labelKey: MessageKey
  icon: LucideIcon
  href: string
}

const PRIMARY_NAV: NavItemDef[] = [
  { labelKey: 'nav.dashboard', icon: LayoutDashboard, href: '/' },
  { labelKey: 'nav.transactions', icon: ArrowLeftRight, href: '/transactions' },
]

const SECONDARY_NAV: NavItemDef[] = [
  { labelKey: 'nav.setup', icon: Plug, href: '/setup' },
  { labelKey: 'nav.rules', icon: ScrollText, href: '/rules' },
  { labelKey: 'nav.diagnostics', icon: CircleAlert, href: '/diagnostics' },
  { labelKey: 'nav.expenseGroups', icon: Layers, href: '/expense-groups?tab=categories' },
  { labelKey: 'nav.ignored', icon: EyeOff, href: '/ignored' },
  { labelKey: 'nav.settings', icon: Settings2, href: '/settings' },
]

function GithubIcon({ size = 14 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="flex-shrink-0"
      aria-hidden="true"
    >
      <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
    </svg>
  )
}

// Reusable nav item class — single source of truth for alignment
function navItemCls(collapsed: boolean, active = false) {
  return cn(
    'relative flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
    collapsed && 'justify-center gap-0 px-2',
    active
      ? 'bg-accent font-medium text-accent-foreground'
      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
  )
}

export function Sidebar({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  const { handlers: tipHandlers, tip } = useTooltip('right')
  const { t } = useI18n()
  const { data: session } = useSession()
  const logout = useLogout()
  const navigate = useNavigate()
  const sidebarToggleLabel = `${collapsed ? t('sidebar.open') : t('sidebar.close')} (${shortcutLabel('.')})`
  const { data: gmailStatus, isSuccess: gmailStatusLoaded } = useReaderStatus('gmail')
  const { data: openDiagnostics } = useExtractionDiagnostics('open')
  const setupNeedsAttention =
    gmailStatusLoaded &&
    gmailStatus?.auth_type === 'oauth' &&
    gmailStatus.auth_state === 'reauthorization_required'
  const openDiagnosticsCount = openDiagnostics?.length ?? 0
  const avatarKey = session && isAvatarKey(session.avatar_key) ? session.avatar_key : 'default'

  const diagnosticsCountLabel =
    openDiagnosticsCount === 1
      ? t('sidebar.diagnosticsCount.one')
      : t('sidebar.diagnosticsCount', { count: openDiagnosticsCount })
  const handleLogout = () => {
    logout.mutate(undefined, {
      onSuccess: () => navigate('/login', { replace: true }),
    })
  }

  return (
    <div
      style={{ width: collapsed ? '56px' : '240px' }}
      className="flex h-screen flex-shrink-0 flex-col overflow-hidden border-r border-border bg-card transition-[width] duration-200 ease-in-out"
    >
      {/* Header row — logo + collapse toggle */}
      <div
        className={cn(
          'flex h-12 flex-shrink-0 items-center border-b border-border',
          collapsed ? 'px-3' : 'px-4',
          collapsed ? 'justify-center' : 'justify-between',
        )}
      >
        {!collapsed && (
          <NavLink
            to="/"
            aria-label="Expensor home"
            className="flex min-w-0 flex-1 items-end gap-2"
          >
            <img
              src="/brand/expensor-wallet.svg"
              alt=""
              aria-hidden="true"
              className="h-6 w-6 flex-shrink-0"
            />
            <img
              src="/brand/expensor-wordmark.svg"
              alt=""
              aria-hidden="true"
              className="h-6 w-auto max-w-[132px] translate-y-[5px]"
            />
          </NavLink>
        )}

        {/* Toggle button — tooltip via shared portal */}
        <button
          onClick={onToggle}
          {...tipHandlers(sidebarToggleLabel)}
          aria-label={sidebarToggleLabel}
          className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          {collapsed ? (
            <img src="/brand/expensor-wallet.svg" alt="" aria-hidden="true" className="h-4 w-4" />
          ) : (
            <PanelLeft size={15} />
          )}
        </button>
      </div>

      {/* Nav */}
      <nav
        className="flex-1 space-y-4 overflow-y-auto overflow-x-hidden py-3"
        aria-label="Main navigation"
      >
        <div className="space-y-0.5 px-2">
          {PRIMARY_NAV.map((item) => (
            <NavLink
              key={item.href}
              to={item.href}
              end={item.href === '/'}
              {...(collapsed ? tipHandlers(t(item.labelKey)) : {})}
              className={({ isActive }) => navItemCls(collapsed, isActive)}
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'whitespace-nowrap transition-opacity duration-200',
                  collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
                )}
              >
                {t(item.labelKey)}
              </span>
            </NavLink>
          ))}
        </div>

        <div className="px-2">
          <div className="border-t border-border" />
        </div>

        <div className="space-y-0.5 px-2">
          {SECONDARY_NAV.map((item) => {
            const isSetup = item.href === '/setup'
            const isDiagnostics = item.href === '/diagnostics'
            const showSetupDot = isSetup && setupNeedsAttention
            const diagnosticsCount = isDiagnostics ? openDiagnosticsCount : 0
            const showDiagnosticsCount = diagnosticsCount > 0
            const hasIndicator = showSetupDot || showDiagnosticsCount
            const label = t(item.labelKey)
            const tooltipLabel =
              showSetupDot && collapsed
                ? t('sidebar.setupAttention')
                : showDiagnosticsCount && collapsed
                  ? `${label} · ${diagnosticsCountLabel}`
                  : label

            return (
              <NavLink
                key={item.href}
                to={item.href}
                end={item.href === '/'}
                data-testid={isDiagnostics ? 'nav-link-diagnostics' : undefined}
                {...(collapsed ? tipHandlers(tooltipLabel) : {})}
                className={({ isActive }) =>
                  cn(navItemCls(collapsed, isActive), hasIndicator && 'overflow-visible')
                }
              >
                <item.icon size={16} className="flex-shrink-0" />
                <span
                  className={cn(
                    'whitespace-nowrap transition-opacity duration-200',
                    collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
                  )}
                >
                  {label}
                </span>

                {showSetupDot && (
                  <span
                    data-testid="setup-attention-dot"
                    aria-hidden="true"
                    className={cn(
                      'h-2 w-2 rounded-full bg-amber-400 shadow-[0_0_0_2px_hsl(var(--card))]',
                      collapsed ? 'absolute right-1.5 top-1.5' : 'ml-auto flex-shrink-0',
                    )}
                  />
                )}

                {showDiagnosticsCount && (
                  <span
                    data-testid="diagnostics-count-badge"
                    aria-label={diagnosticsCountLabel}
                    className={cn(
                      'inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-amber-400 px-1.5 text-[11px] font-semibold leading-none text-slate-950 shadow-[0_0_0_2px_hsl(var(--card))]',
                      collapsed ? 'absolute right-0 top-0' : 'ml-auto flex-shrink-0',
                    )}
                  >
                    {diagnosticsCount > 99 ? '99+' : diagnosticsCount}
                  </span>
                )}
              </NavLink>
            )
          })}
        </div>
      </nav>

      {/* Bottom — same px-2 wrapper + navItemCls pattern as nav for consistent alignment */}
      <div className="flex-shrink-0 space-y-0.5 border-t border-border px-2 py-2">
        <ThemeToggle collapsed={collapsed} onTip={collapsed ? tipHandlers : undefined} />

        {/* Report Issues */}
        <a
          href="https://github.com/ArionMiles/expensor/issues"
          target="_blank"
          rel="noopener noreferrer"
          {...(collapsed ? tipHandlers(t('nav.reportIssues')) : {})}
          className={navItemCls(collapsed)}
        >
          <GithubIcon size={16} />
          <span
            className={cn(
              'whitespace-nowrap transition-opacity duration-200',
              collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
            )}
          >
            {t('nav.reportIssues')}
          </span>
        </a>

        <NavLink
          to="/settings?tab=account"
          aria-label={
            session
              ? `${session.display_name} ${session.email}`
              : t('nav.settings.account.subtitle')
          }
          className={({ isActive }) =>
            cn(navItemCls(collapsed, isActive), !collapsed && 'border border-border/50')
          }
          {...(collapsed && session
            ? tipHandlers(
                `${session.display_name} · ${t(session.role === 'admin' ? 'account.role.admin' : 'account.role.user')}`,
              )
            : {})}
        >
          <span
            data-testid="sidebar-user-avatar"
            className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full [&_svg]:h-full [&_svg]:w-full"
            aria-hidden="true"
            dangerouslySetInnerHTML={{ __html: avatarByKey[avatarKey] }}
          />
          <div
            className={cn(
              'min-w-0 transition-opacity duration-200',
              collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
            )}
          >
            <p className="truncate text-xs font-medium text-foreground">
              {session?.display_name ?? t('account.title')}
            </p>
            <p className="truncate text-[10px] text-muted-foreground">{session?.email}</p>
          </div>
        </NavLink>
        <button
          type="button"
          onClick={handleLogout}
          disabled={logout.isPending}
          {...(collapsed ? tipHandlers(t('sidebar.signOut')) : {})}
          className={navItemCls(collapsed)}
        >
          <LogOut size={16} className="flex-shrink-0" />
          <span
            className={cn(
              'whitespace-nowrap transition-opacity duration-200',
              collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
            )}
          >
            {t('sidebar.signOut')}
          </span>
        </button>
      </div>

      {tip}
    </div>
  )
}
