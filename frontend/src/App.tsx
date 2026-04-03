import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { AppLayout } from '@/components/AppLayout'

const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Transactions = lazy(() => import('@/pages/Transactions'))
const Wizard = lazy(() => import('@/pages/setup/Wizard').then((m) => ({ default: m.Wizard })))
const Settings = lazy(() => import('@/pages/Settings'))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      staleTime: 30_000,
    },
  },
})

function PageSuspense({ children }: { children: React.ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center">
          <span className="font-mono text-xs text-muted-foreground">loading...</span>
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ErrorBoundary>
          <Routes>
            <Route element={<AppLayout />}>
              <Route
                path="/"
                element={
                  <PageSuspense>
                    <Dashboard />
                  </PageSuspense>
                }
              />
              <Route
                path="/transactions"
                element={
                  <PageSuspense>
                    <Transactions />
                  </PageSuspense>
                }
              />
              <Route
                path="/setup"
                element={
                  <PageSuspense>
                    <Wizard />
                  </PageSuspense>
                }
              />
              <Route
                path="/settings"
                element={
                  <PageSuspense>
                    <Settings />
                  </PageSuspense>
                }
              />
            </Route>
          </Routes>
        </ErrorBoundary>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
