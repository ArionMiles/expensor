import axios from 'axios'
import type {
  AuthStartResponse,
  AuthStatus,
  ChartData,
  CredentialsStatus,
  HealthResponse,
  PluginInfo,
  ReaderConfig,
  ReaderStatus,
  StatusResponse,
  Transaction,
  TransactionFilters,
  TransactionsResponse,
} from './types'

const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

apiClient.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error)) {
      if (error.response) {
        const data = error.response.data as { error?: string; message?: string } | undefined
        const message = data?.error ?? data?.message ?? `Server error: ${error.response.status}`
        throw new Error(message)
      } else if (error.request) {
        throw new Error('No response from server. Is the backend running?')
      } else {
        throw new Error(error.message ?? 'Request failed')
      }
    }
    throw error
  },
)

export const api = {
  health: {
    get: () => apiClient.get<HealthResponse>('/health'),
  },

  status: {
    get: () => apiClient.get<StatusResponse>('/status'),
  },

  stats: {
    charts: () => apiClient.get<ChartData>('/stats/charts'),
  },

  daemon: {
    start: (reader: string) => apiClient.post<{ status: string }>('/daemon/start', { reader }),
  },

  plugins: {
    readers: () => apiClient.get<PluginInfo[]>('/plugins/readers'),
  },

  readers: {
    credentials: {
      upload: async (readerName: string, file: File) => {
        const text = await file.text()
        return apiClient.post(`/readers/${readerName}/credentials`, text, {
          headers: { 'Content-Type': 'application/json' },
        })
      },
      status: (readerName: string) =>
        apiClient.get<CredentialsStatus>(`/readers/${readerName}/credentials/status`),
    },

    auth: {
      start: (readerName: string) =>
        apiClient.post<AuthStartResponse>(`/readers/${readerName}/auth/start`),
      status: (readerName: string) =>
        apiClient.get<AuthStatus>(`/readers/${readerName}/auth/status`),
      revoke: (readerName: string) => apiClient.delete(`/readers/${readerName}/auth/token`),
    },

    config: {
      get: (readerName: string) => apiClient.get<ReaderConfig>(`/readers/${readerName}/config`),
      save: (readerName: string, config: Record<string, string>) =>
        apiClient.post(`/readers/${readerName}/config`, { config }),
    },

    status: (readerName: string) => apiClient.get<ReaderStatus>(`/readers/${readerName}/status`),

    disconnect: (readerName: string) => apiClient.delete(`/readers/${readerName}`),
  },

  config: {
    getBaseCurrency: () => apiClient.get<{ base_currency: string }>('/config/base-currency'),
    setBaseCurrency: (currency: string) =>
      apiClient.put<{ base_currency: string }>('/config/base-currency', {
        base_currency: currency,
      }),
  },

  transactions: {
    list: (filters: TransactionFilters = {}) => {
      const params = new URLSearchParams()
      if (filters.page) params.set('page', String(filters.page))
      if (filters.page_size) params.set('page_size', String(filters.page_size))
      if (filters.category) params.set('category', filters.category)
      if (filters.currency) params.set('currency', filters.currency)
      if (filters.source) params.set('source', filters.source)
      if (filters.label) params.set('label', filters.label)
      if (filters.date_from) params.set('date_from', filters.date_from)
      if (filters.date_to) params.set('date_to', filters.date_to)
      return apiClient.get<TransactionsResponse>(`/transactions?${params.toString()}`)
    },

    search: (q: string, page = 1, pageSize = 20) => {
      const params = new URLSearchParams({ q, page: String(page), page_size: String(pageSize) })
      return apiClient.get<TransactionsResponse>(`/transactions/search?${params.toString()}`)
    },

    get: (id: string) => apiClient.get<Transaction>(`/transactions/${id}`),

    update: (id: string, description: string) =>
      apiClient.put<Transaction>(`/transactions/${id}`, { description }),

    addLabels: (id: string, labels: string[]) =>
      apiClient.post<Transaction>(`/transactions/${id}/labels`, { labels }),

    removeLabel: (id: string, label: string) =>
      apiClient.delete<Transaction>(`/transactions/${id}/labels/${encodeURIComponent(label)}`),
  },
}
