import {
  ArrowLeftRight,
  ChevronLeft,
  ChevronRight,
  Download,
  FileBarChart,
  LayoutDashboard,
  type LucideIcon,
  Plug,
  ScrollText,
  Settings2,
} from 'lucide-react'
import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { ThemeToggle } from './ThemeToggle'

interface NavItemDef {
  label: string
  icon: LucideIcon
  href: string
  soon?: boolean
}

const PRIMARY_NAV: NavItemDef[] = [
  { label: 'Dashboard', icon: LayoutDashboard, href: '/' },
  { label: 'Transactions', icon: ArrowLeftRight, href: '/transactions' },
]

const SECONDARY_NAV: NavItemDef[] = [
  { label: 'Onboarding', icon: Plug, href: '/setup' },
  { label: 'Rules', icon: ScrollText, href: '/rules' },
  { label: 'Settings', icon: Settings2, href: '/settings' },
]

const FUTURE_NAV: NavItemDef[] = [
  { label: 'Reports', icon: FileBarChart, href: '/reports', soon: true },
  { label: 'Exports', icon: Download, href: '/exports', soon: true },
]

function getStoredCollapsed(): boolean {
  try {
    return localStorage.getItem('sidebar_collapsed') === 'true'
  } catch {
    return false
  }
}

export function Sidebar() {
  const [collapsed, setCollapsed] = useState<boolean>(getStoredCollapsed)

  const toggle = () => {
    setCollapsed((prev) => {
      const next = !prev
      try {
        localStorage.setItem('sidebar_collapsed', String(next))
      } catch {
        // ignore
      }
      return next
    })
  }

  return (
    <div
      style={{ width: collapsed ? '56px' : '240px' }}
      className="flex h-screen flex-shrink-0 flex-col overflow-hidden border-r border-border bg-card transition-[width] duration-200 ease-in-out"
    >
      {/* Logo */}
      <div
        className={cn(
          'flex h-12 flex-shrink-0 items-center border-b border-border px-3',
          collapsed ? 'justify-center' : 'justify-between',
        )}
      >
        <span
          className={cn(
            'font-semibold tracking-wide text-primary transition-opacity duration-200',
            collapsed ? 'w-0 overflow-hidden opacity-0' : 'text-sm opacity-100',
          )}
          aria-hidden={collapsed}
        >
          Expensor
        </span>
        {collapsed && (
          <span className="text-xs font-bold text-primary" aria-label="Expensor">
            EX
          </span>
        )}
      </div>

      {/* Nav */}
      <nav
        className="flex-1 space-y-4 overflow-y-auto overflow-x-hidden py-3"
        aria-label="Main navigation"
      >
        {/* Primary */}
        <div className="space-y-0.5 px-2">
          {PRIMARY_NAV.map((item) => (
            <NavLink
              key={item.href}
              to={item.href}
              end={item.href === '/'}
              title={collapsed ? item.label : undefined}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                  collapsed ? 'justify-center px-2' : '',
                  isActive
                    ? 'bg-accent font-medium text-accent-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )
              }
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'whitespace-nowrap transition-opacity duration-200',
                  collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
                )}
              >
                {item.label}
              </span>
            </NavLink>
          ))}
        </div>

        {/* Divider */}
        <div className="px-2">
          <div className="border-t border-border" />
        </div>

        {/* Secondary */}
        <div className="space-y-0.5 px-2">
          {SECONDARY_NAV.map((item) => (
            <NavLink
              key={item.href}
              to={item.href}
              end={item.href === '/'}
              title={collapsed ? item.label : undefined}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                  collapsed ? 'justify-center px-2' : '',
                  isActive
                    ? 'bg-accent font-medium text-accent-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )
              }
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'whitespace-nowrap transition-opacity duration-200',
                  collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
                )}
              >
                {item.label}
              </span>
            </NavLink>
          ))}
        </div>

        {/* Future nav */}
        <div className="space-y-0.5 px-2">
          {FUTURE_NAV.map((item) => (
            <div
              key={item.href}
              title={collapsed ? `${item.label} (coming soon)` : undefined}
              className={cn(
                'flex cursor-not-allowed select-none items-center gap-3 rounded-md px-3 py-2 text-sm opacity-50',
                collapsed ? 'justify-center px-2' : '',
                'text-muted-foreground',
              )}
              aria-disabled="true"
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'flex items-center gap-2 whitespace-nowrap transition-opacity duration-200',
                  collapsed ? 'w-0 overflow-hidden opacity-0' : 'opacity-100',
                )}
              >
                {item.label}
                <span className="rounded border border-border px-1 py-0.5 text-[9px] leading-none">
                  Soon
                </span>
              </span>
            </div>
          ))}
        </div>
      </nav>

      {/* Bottom controls */}
      <div className="flex-shrink-0 space-y-0.5 border-t border-border px-2 py-2">
        <ThemeToggle collapsed={collapsed} />
        <button
          onClick={toggle}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className={cn(
            'flex w-full items-center gap-3 rounded-md px-3 py-2 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
            collapsed && 'justify-center px-2',
          )}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? (
            <ChevronRight size={16} className="flex-shrink-0" />
          ) : (
            <>
              <ChevronLeft size={16} className="flex-shrink-0" />
              <span className="whitespace-nowrap">Collapse</span>
            </>
          )}
        </button>
      </div>
    </div>
  )
}
