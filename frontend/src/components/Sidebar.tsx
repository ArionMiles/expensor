import {
  ArrowLeftRight,
  ChevronLeft,
  ChevronRight,
  Download,
  FileBarChart,
  LayoutDashboard,
  type LucideIcon,
  Plug,
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
      className="flex flex-col h-screen bg-card border-r border-border transition-[width] duration-200 ease-in-out overflow-hidden flex-shrink-0"
    >
      {/* Logo */}
      <div
        className={cn(
          'flex items-center h-12 flex-shrink-0 px-3 border-b border-border',
          collapsed ? 'justify-center' : 'justify-between',
        )}
      >
        <span
          className={cn(
            'font-semibold text-primary tracking-wide transition-opacity duration-200',
            collapsed ? 'opacity-0 w-0 overflow-hidden' : 'opacity-100 text-sm',
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
      <nav className="flex-1 overflow-y-auto overflow-x-hidden py-3 space-y-4" aria-label="Main navigation">
        {/* Primary */}
        <div className="px-2 space-y-0.5">
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
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )
              }
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'transition-opacity duration-200 whitespace-nowrap',
                  collapsed ? 'opacity-0 w-0 overflow-hidden' : 'opacity-100',
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
        <div className="px-2 space-y-0.5">
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
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )
              }
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'transition-opacity duration-200 whitespace-nowrap',
                  collapsed ? 'opacity-0 w-0 overflow-hidden' : 'opacity-100',
                )}
              >
                {item.label}
              </span>
            </NavLink>
          ))}
        </div>

        {/* Future nav */}
        <div className="px-2 space-y-0.5">
          {FUTURE_NAV.map((item) => (
            <div
              key={item.href}
              title={collapsed ? `${item.label} (coming soon)` : undefined}
              className={cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm opacity-50 cursor-not-allowed select-none',
                collapsed ? 'justify-center px-2' : '',
                'text-muted-foreground',
              )}
              aria-disabled="true"
            >
              <item.icon size={16} className="flex-shrink-0" />
              <span
                className={cn(
                  'transition-opacity duration-200 whitespace-nowrap flex items-center gap-2',
                  collapsed ? 'opacity-0 w-0 overflow-hidden' : 'opacity-100',
                )}
              >
                {item.label}
                <span className="text-[9px] border border-border rounded px-1 py-0.5 leading-none">
                  Soon
                </span>
              </span>
            </div>
          ))}
        </div>
      </nav>

      {/* Bottom controls */}
      <div className="flex-shrink-0 border-t border-border py-2 px-2 space-y-0.5">
        <ThemeToggle collapsed={collapsed} />
        <button
          onClick={toggle}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className={cn(
            'flex items-center gap-3 w-full rounded-md px-3 py-2 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors',
            collapsed && 'justify-center px-2',
          )}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <ChevronRight size={16} className="flex-shrink-0" /> : (
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
