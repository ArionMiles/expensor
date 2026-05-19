import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

type Theme = 'dark' | 'light' | 'system'

interface ThemeContextValue {
  theme: Theme
  setTheme: (t: Theme) => void
}

const ThemeContext = createContext<ThemeContextValue>({ theme: 'system', setTheme: () => {} })

function applyTheme(theme: Theme) {
  const root = document.documentElement
  const resolved =
    theme === 'system'
      ? window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'
      : theme
  root.classList.remove('dark', 'light')
  root.classList.add(resolved)
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => {
    try {
      const stored = localStorage.getItem('theme')
      if (stored === 'dark' || stored === 'light' || stored === 'system') return stored
    } catch {
      // ignore
    }
    return 'system'
  })

  useEffect(() => {
    applyTheme(theme)
    try {
      localStorage.setItem('theme', theme)
    } catch {
      // ignore
    }
  }, [theme])

  // React to OS-level changes when theme is 'system'
  useEffect(() => {
    if (theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => applyTheme('system')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [theme])

  return (
    <ThemeContext.Provider value={{ theme, setTheme: setThemeState }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
