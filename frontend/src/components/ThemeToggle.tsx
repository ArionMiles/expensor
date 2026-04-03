import { type LucideIcon, Monitor, Moon, Sun } from 'lucide-react'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'

type Theme = 'system' | 'light' | 'dark'

function applyTheme(theme: Theme) {
  const root = document.documentElement
  if (theme === 'system') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    root.setAttribute('data-theme', prefersDark ? 'dark' : 'light')
  } else {
    root.setAttribute('data-theme', theme)
  }
}

function getStoredTheme(): Theme {
  try {
    const stored = localStorage.getItem('theme')
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored
  } catch {
    // ignore
  }
  return 'system'
}

const NEXT: Record<Theme, Theme> = { system: 'light', light: 'dark', dark: 'system' }
const ICON: Record<Theme, LucideIcon> = {
  system: Monitor,
  light: Sun,
  dark: Moon,
}
const LABEL: Record<Theme, string> = {
  system: 'System theme',
  light: 'Light theme',
  dark: 'Dark theme',
}

export function ThemeToggle({ collapsed }: { collapsed?: boolean }) {
  const [theme, setTheme] = useState<Theme>(getStoredTheme)

  useEffect(() => {
    applyTheme(theme)
    try {
      localStorage.setItem('theme', theme)
    } catch {
      // ignore
    }
  }, [theme])

  const Icon = ICON[theme]

  return (
    <button
      onClick={() => setTheme(NEXT[theme])}
      title={LABEL[theme]}
      className={cn(
        'flex w-full items-center gap-3 rounded-md px-3 py-2 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
        collapsed && 'justify-center px-2',
      )}
    >
      <Icon size={16} className="flex-shrink-0" />
      {!collapsed && <span>{LABEL[theme]}</span>}
    </button>
  )
}
