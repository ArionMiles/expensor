import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { AppLayout } from '@/components/AppLayout'
import { DisplayProvider } from '@/contexts/DisplayContext'
import { I18nProvider } from '@/i18n/I18nProvider'
import { useSetupStatus } from '@/api/queries'
import { AuthGate } from '@/pages/auth/AuthPages'

const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Transactions = lazy(() => import('@/pages/Transactions'))
const Wizard = lazy(() => import('@/pages/setup/Wizard').then((m) => ({ default: m.Wizard })))
const Settings = lazy(() => import('@/pages/Settings'))
const Rules = lazy(() => import('@/pages/Rules'))
const RuleForm = lazy(() => import('@/pages/rules/RuleForm').then((m) => ({ default: m.RuleForm })))
const RuleEmailSearch = lazy(() =>
  import('@/pages/rules/RuleEmailSearch').then((m) => ({ default: m.RuleEmailSearch })),
)
const Diagnostics = lazy(() => import('@/pages/Diagnostics'))
const ExpenseGroupsPage = lazy(() => import('@/pages/ExpenseGroupsPage'))
const IgnoredPage = lazy(() => import('@/pages/IgnoredPage'))
const LoginPage = lazy(() =>
  import('@/pages/auth/AuthPages').then((m) => ({ default: m.LoginPage })),
)
const BootstrapPage = lazy(() =>
  import('@/pages/auth/AuthPages').then((m) => ({ default: m.BootstrapPage })),
)
const AccountSetupPage = lazy(() =>
  import('@/pages/auth/AuthPages').then((m) => ({ default: m.AccountSetupPage })),
)

export const queryClient = new QueryClient({
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
              <AuthGate>
                <Routes>
                  <Route
                    path="/login"
                    element={
                      <PageSuspense>
                        <LoginPage />
                      </PageSuspense>
                    }
                  />
                  <Route
                    path="/bootstrap"
                    element={
                      <PageSuspense>
                        <BootstrapPage />
                      </PageSuspense>
                    }
                  />
                  <Route
                    path="/account-setup"
                    element={
                      <PageSuspense>
                        <AccountSetupPage />
                      </PageSuspense>
                    }
                  />
                  <Route element={<AppLayout />}>
                    <Route
                      path="/"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Dashboard />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/transactions"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Transactions />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/setup"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Wizard />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/settings"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Settings />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/rules"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Rules />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/rules/new/search"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <RuleEmailSearch />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/rules/new"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <RuleForm />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/rules/:id"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <RuleForm />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/diagnostics"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <Diagnostics />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/expense-groups"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <ExpenseGroupsPage />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route
                      path="/ignored"
                      element={
                        <FirstRunGate>
                          <PageSuspense>
                            <IgnoredPage />
                          </PageSuspense>
                        </FirstRunGate>
                      }
                    />
                    <Route path="/muted" element={<RedirectToIgnored />} />
                  </Route>
                </Routes>
              </AuthGate>
            </ErrorBoundary>
          </BrowserRouter>
        </DisplayProvider>
      </I18nProvider>
    </QueryClientProvider>
  )
}
