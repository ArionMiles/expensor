import { useEffect, useMemo, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { CommandPalette } from './CommandPalette'
import { DaemonStatusBar } from './DaemonStatusBar'
import { Sidebar } from './Sidebar'
import { DocumentTitle } from '@/lib/documentTitle'
import { NAVIGATION_TARGETS } from '@/lib/navigation'

function getStoredCollapsed(): boolean {
  try {
    return localStorage.getItem('sidebar_collapsed') === 'true'
  } catch {
    return false
  }
}

export function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState<boolean>(getStoredCollapsed)
  const [paletteOpen, setPaletteOpen] = useState(false)

  const toggleSidebar = () => {
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

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!event.metaKey && !event.ctrlKey) return

      if (event.key === '.') {
        event.preventDefault()
        toggleSidebar()
        return
      }

      if (event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setPaletteOpen(true)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  useEffect(() => {
    setPaletteOpen(false)
  }, [location.pathname, location.search])

  const navigationTargets = useMemo(() => NAVIGATION_TARGETS, [])

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <DocumentTitle />
      <Sidebar collapsed={collapsed} onToggle={toggleSidebar} />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <DaemonStatusBar />
        <main className="flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
      <CommandPalette
        open={paletteOpen}
        targets={navigationTargets}
        onClose={() => setPaletteOpen(false)}
        onNavigate={(path) => {
          navigate(path)
          setPaletteOpen(false)
        }}
      />
    </div>
  )
}
