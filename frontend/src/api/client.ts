import axios from 'axios'
import type {
  AnnualHeatmapData,
  AuthStartResponse,
  AuthStatus,
  Bucket,
  Category,
  ChartData,
  CredentialsStatus,
  Facets,
  HeatmapData,
  HealthResponse,
  Label,
  PluginInfo,
  ReaderConfig,
  ReaderStatus,
  Rule,
  RuleImport,
  StatusResponse,
  Transaction,
  TransactionFilters,
  TransactionPatch,
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
    heatmap: (from?: string, to?: string) => {
      const params = new URLSearchParams()
      if (from) params.set('from', from)
      if (to) params.set('to', to)
      const qs = params.toString()
      return apiClient.get<HeatmapData>(qs ? `/stats/heatmap?${qs}` : '/stats/heatmap')
    },
    annualHeatmap: (year: number) =>
      apiClient.get<AnnualHeatmapData>(`/stats/heatmap/annual?year=${year}`),
  },

  daemon: {
    start: (reader: string) => apiClient.post<{ status: string }>('/daemon/start', { reader }),
    rescan: (reader: string) =>
      apiClient.post<{ status: 'rescanning' | 'queued' }>('/daemon/rescan', { reader }),
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
    getActiveReader: () => apiClient.get<{ reader: string }>('/config/active-reader'),
    getBaseCurrency: () => apiClient.get<{ base_currency: string }>('/config/base-currency'),
    setBaseCurrency: (currency: string) =>
      apiClient.put<{ base_currency: string }>('/config/base-currency', {
        base_currency: currency,
      }),
    getScanInterval: () => apiClient.get<{ scan_interval: string }>('/config/scan-interval'),
    setScanInterval: (seconds: number) =>
      apiClient.put<{ scan_interval: string }>('/config/scan-interval', {
        scan_interval: String(seconds),
      }),
    getLookbackDays: () => apiClient.get<{ lookback_days: string }>('/config/lookback-days'),
    setLookbackDays: (days: number) =>
      apiClient.put<{ lookback_days: string }>('/config/lookback-days', {
        lookback_days: String(days),
      }),

    labels: {
      list: () => apiClient.get<Label[]>('/config/labels'),
      create: (name: string, color: string) =>
        apiClient.post<{ name: string; color: string }>('/config/labels', { name, color }),
      update: (name: string, color: string) =>
        apiClient.put<{ name: string; color: string }>(
          `/config/labels/${encodeURIComponent(name)}`,
          { color },
        ),
      delete: (name: string) => apiClient.delete(`/config/labels/${encodeURIComponent(name)}`),
      apply: (name: string, merchantPattern: string) =>
        apiClient.post<{ applied: number }>(`/config/labels/${encodeURIComponent(name)}/apply`, {
          merchant_pattern: merchantPattern,
        }),
    },

    categories: {
      list: () => apiClient.get<Category[]>('/config/categories'),
      create: (name: string, description?: string) =>
        apiClient.post<{ name: string }>('/config/categories', { name, description }),
      delete: (name: string) => apiClient.delete(`/config/categories/${encodeURIComponent(name)}`),
    },

    buckets: {
      list: () => apiClient.get<Bucket[]>('/config/buckets'),
      create: (name: string, description?: string) =>
        apiClient.post<{ name: string }>('/config/buckets', { name, description }),
      delete: (name: string) => apiClient.delete(`/config/buckets/${encodeURIComponent(name)}`),
    },
  },

  rules: {
    list: () => apiClient.get<Rule[]>('/rules'),
    create: (body: Omit<Rule, 'id' | 'source' | 'created_at' | 'updated_at'>) =>
      apiClient.post<Rule>('/rules', body),
    update: (
      id: string,
      body: Partial<Omit<Rule, 'id' | 'source' | 'created_at' | 'updated_at'>>,
    ) => apiClient.put<Rule>(`/rules/${id}`, body),
    delete: (id: string) => apiClient.delete(`/rules/${id}`),
    export: () => apiClient.get<RuleImport[]>('/rules/export'),
    import: (rules: RuleImport[]) => apiClient.post<{ imported: number }>('/rules/import', rules),
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
      if (filters.sort_by) params.set('sort_by', filters.sort_by)
      if (filters.sort_dir) params.set('sort_dir', filters.sort_dir)
      return apiClient.get<TransactionsResponse>(`/transactions?${params.toString()}`)
    },

    search: (q: string, page = 1, pageSize = 20) => {
      const params = new URLSearchParams({ q, page: String(page), page_size: String(pageSize) })
      return apiClient.get<TransactionsResponse>(`/transactions/search?${params.toString()}`)
    },

    facets: () => apiClient.get<Facets>('/transactions/facets'),

    get: (id: string) => apiClient.get<Transaction>(`/transactions/${id}`),

    update: (id: string, patch: TransactionPatch) =>
      apiClient.put<Transaction>(`/transactions/${id}`, patch),

    addLabels: (id: string, labels: string[]) =>
      apiClient.post<Transaction>(`/transactions/${id}/labels`, { labels }),

    removeLabel: (id: string, label: string) =>
      apiClient.delete<Transaction>(`/transactions/${id}/labels/${encodeURIComponent(label)}`),
  },
}
