import { type LucideIcon, Monitor, Moon, Sun } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useTheme } from './ThemeProvider'

type Theme = 'dark' | 'light' | 'system'

const NEXT: Record<Theme, Theme> = { system: 'light', light: 'dark', dark: 'system' }
const ICON: Record<Theme, LucideIcon> = { system: Monitor, light: Sun, dark: Moon }
const LABEL: Record<Theme, string> = {
  system: 'System theme',
  light: 'Light theme',
  dark: 'Dark theme',
}

export function ThemeToggle({ collapsed }: { collapsed?: boolean }) {
  const { theme, setTheme } = useTheme()
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
