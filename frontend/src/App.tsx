import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { ErrorBoundary } from '@/components/ErrorBoundary'

const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Transactions = lazy(() => import('@/pages/Transactions'))
const Wizard = lazy(() =>
  import('@/pages/setup/Wizard').then((m) => ({ default: m.Wizard })),
)

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
        <div className="min-h-screen bg-[var(--color-bg)] flex items-center justify-center">
          <span className="text-xs font-mono text-[var(--color-text-muted)]">loading...</span>
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
          </Routes>
        </ErrorBoundary>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
