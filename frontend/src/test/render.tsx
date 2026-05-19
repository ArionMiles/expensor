import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { ReactElement } from 'react'
import { DisplayProvider } from '@/contexts/DisplayContext'
import { I18nProvider } from '@/i18n/I18nProvider'

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 0,
        gcTime: Infinity,
      },
      mutations: {
        retry: false,
      },
    },
  })
}

export function renderWithProviders(ui: ReactElement, { route = '/' }: { route?: string } = {}) {
  const queryClient = createTestQueryClient()

  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <I18nProvider>
          <DisplayProvider>
            <MemoryRouter
              initialEntries={[route]}
              future={{
                v7_startTransition: true,
                v7_relativeSplatPath: true,
              }}
            >
              {ui}
            </MemoryRouter>
          </DisplayProvider>
        </I18nProvider>
      </QueryClientProvider>,
    ),
  }
}
