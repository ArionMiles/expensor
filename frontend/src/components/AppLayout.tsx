import { Outlet } from 'react-router-dom'
import { DaemonStatusBar } from './DaemonStatusBar'
import { Sidebar } from './Sidebar'

export function AppLayout() {
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <DaemonStatusBar />
        <main className="flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
