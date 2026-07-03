import { useEffect, useMemo, useState } from 'react'
import { KeyRound, LogOut, RefreshCw, RotateCcw, ScrollText, UserPlus } from 'lucide-react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { CommandPalette, type CommandPaletteAction } from './CommandPalette'
import { DaemonStatusBar } from './DaemonStatusBar'
import { Sidebar } from './Sidebar'
import {
  useActiveReader,
  useClearReaderCheckpoint,
  useLogout,
  useRescan,
  useSession,
} from '@/api/queries'
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
  const { data: session } = useSession()
  const { data: activeReader } = useActiveReader()
  const logout = useLogout()
  const rescan = useRescan()
  const clearCheckpoint = useClearReaderCheckpoint()
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
  const commandActions = useMemo(() => {
    const reader = activeReader ?? ''
    const actions: CommandPaletteAction[] = [
      {
        id: 'create-rule',
        titleKey: 'command.actions.createRule' as const,
        descriptionKey: 'command.actions.createRule.description' as const,
        icon: ScrollText,
        keywords: ['new rule', 'rule'],
      },
      {
        id: 'create-token',
        titleKey: 'command.actions.createToken' as const,
        descriptionKey: 'command.actions.createToken.description' as const,
        icon: KeyRound,
        keywords: ['new token', 'access token', 'personal access token'],
      },
      {
        id: 'force-rescan',
        titleKey: 'settings.daemon.forceRescan' as const,
        descriptionKey: 'command.actions.forceRescan.description' as const,
        icon: RefreshCw,
        keywords: ['rescan', 'scan', 'daemon'],
        disabled: !reader || rescan.isPending,
      },
      {
        id: 'clear-checkpoint',
        titleKey: 'settings.daemon.clearCheckpoint' as const,
        descriptionKey: 'command.actions.clearCheckpoint.description' as const,
        icon: RotateCcw,
        keywords: ['checkpoint', 'clear checkpoint', 'daemon'],
        disabled: !reader || clearCheckpoint.isPending,
      },
    ]

    if (session?.role === 'admin') {
      actions.splice(2, 0, {
        id: 'create-user',
        titleKey: 'command.actions.createUser' as const,
        descriptionKey: 'command.actions.createUser.description' as const,
        icon: UserPlus,
        keywords: ['new user', 'invite user', 'create user'],
      })
    }

    actions.push({
      id: 'logout',
      titleKey: 'sidebar.signOut' as const,
      descriptionKey: 'command.actions.signOut.description' as const,
      icon: LogOut,
      keywords: ['logout', 'log out', 'sign out'],
      variant: 'destructive' as const,
      disabled: logout.isPending,
    })

    return actions
  }, [activeReader, clearCheckpoint.isPending, logout.isPending, rescan.isPending, session?.role])

  const closePaletteAndNavigate = (path: string) => {
    navigate(path)
    setPaletteOpen(false)
  }

  const runCommandAction = (id: string) => {
    const reader = activeReader ?? ''
    if (id === 'create-rule') {
      closePaletteAndNavigate('/rules/new')
      return
    }
    if (id === 'create-token') {
      closePaletteAndNavigate('/settings?tab=account&action=create-token')
      return
    }
    if (id === 'create-user') {
      closePaletteAndNavigate('/settings?tab=account&action=create-user')
      return
    }
    if (id === 'force-rescan' && reader) {
      rescan.mutate(reader)
      setPaletteOpen(false)
      return
    }
    if (id === 'clear-checkpoint' && reader) {
      clearCheckpoint.mutate(reader)
      setPaletteOpen(false)
      return
    }
    if (id === 'logout') {
      logout.mutate(undefined, {
        onSuccess: () => {
          setPaletteOpen(false)
          navigate('/login', { replace: true })
        },
      })
    }
  }

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
        actions={commandActions}
        onClose={() => setPaletteOpen(false)}
        onNavigate={(path) => {
          navigate(path)
          setPaletteOpen(false)
        }}
        onAction={runCommandAction}
      />
    </div>
  )
}
