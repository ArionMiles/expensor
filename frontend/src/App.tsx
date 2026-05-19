import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { AppLayout } from '@/components/AppLayout'
import { DisplayProvider } from '@/contexts/DisplayContext'
import { I18nProvider } from '@/i18n/I18nProvider'
import { useSetupStatus } from '@/api/queries'

const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Transactions = lazy(() => import('@/pages/Transactions'))
const Wizard = lazy(() => import('@/pages/setup/Wizard').then((m) => ({ default: m.Wizard })))
const Settings = lazy(() => import('@/pages/Settings'))
const Rules = lazy(() => import('@/pages/Rules'))
const RuleForm = lazy(() => import('@/pages/rules/RuleForm').then((m) => ({ default: m.RuleForm })))
const Diagnostics = lazy(() => import('@/pages/Diagnostics'))
const ExpenseGroupsPage = lazy(() => import('@/pages/ExpenseGroupsPage'))
const IgnoredPage = lazy(() => import('@/pages/IgnoredPage'))

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

function RedirectToIgnored() {
  const location = useLocation()
  return <Navigate to={`/ignored${location.search}`} replace />
}

function FirstRunGate({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { data: setupStatus, isLoading } = useSetupStatus()

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="font-mono text-xs text-muted-foreground">loading...</span>
      </div>
    )
  }

  if (setupStatus?.required && location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />
  }

  return <>{children}</>
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <DisplayProvider>
          <BrowserRouter>
            <ErrorBoundary>
              <FirstRunGate>
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
                    <Route
                      path="/rules"
                      element={
                        <PageSuspense>
                          <Rules />
                        </PageSuspense>
                      }
                    />
                    <Route
                      path="/rules/new"
                      element={
                        <PageSuspense>
                          <RuleForm />
                        </PageSuspense>
                      }
                    />
                    <Route
                      path="/rules/:id"
                      element={
                        <PageSuspense>
                          <RuleForm />
                        </PageSuspense>
                      }
                    />
                    <Route
                      path="/diagnostics"
                      element={
                        <PageSuspense>
                          <Diagnostics />
                        </PageSuspense>
                      }
                    />
                    <Route
                      path="/expense-groups"
                      element={
                        <PageSuspense>
                          <ExpenseGroupsPage />
                        </PageSuspense>
                      }
                    />
                    <Route
                      path="/ignored"
                      element={
                        <PageSuspense>
                          <IgnoredPage />
                        </PageSuspense>
                      }
                    />
                    <Route path="/muted" element={<RedirectToIgnored />} />
                  </Route>
                </Routes>
              </FirstRunGate>
            </ErrorBoundary>
          </BrowserRouter>
        </DisplayProvider>
      </I18nProvider>
    </QueryClientProvider>
  )
}
