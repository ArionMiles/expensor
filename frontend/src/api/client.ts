import axios from 'axios'
import type {
  AnnualHeatmapData,
  AuthStartResponse,
  AuthStatus,
  BankColor,
  Bucket,
  Category,
  ChartData,
  DashboardData,
  CredentialsStatus,
  ExtractionDiagnostic,
  ExtractionDiagnosticListStatus,
  ExtractionDiagnosticStatus,
  Facets,
  HeatmapData,
  HealthResponse,
  Label,
  MonthlyBreakdownData,
  MutedMerchantWithCount,
  PluginInfo,
  ReaderConfig,
  ReaderGuide,
  ReaderStatus,
  Rule,
  RuleImport,
  SetupStatus,
  StatusResponse,
  SyncStatus,
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

  version: {
    get: () => apiClient.get<{ version: string }>('/version'),
  },

  stats: {
    charts: () => apiClient.get<ChartData>('/stats/charts'),
    dashboard: () => apiClient.get<DashboardData>('/stats/dashboard'),
    heatmap: (from?: string, to?: string) => {
      const params = new URLSearchParams()
      if (from) params.set('from', from)
      if (to) params.set('to', to)
      const qs = params.toString()
      return apiClient.get<HeatmapData>(qs ? `/stats/heatmap?${qs}` : '/stats/heatmap')
    },
    annualHeatmap: (year: number) =>
      apiClient.get<AnnualHeatmapData>(`/stats/heatmap/annual?year=${year}`),
    monthlyBreakdown: (dimension: 'labels' | 'categories' | 'buckets') =>
      apiClient.get<MonthlyBreakdownData>(
        `/stats/labels/monthly?dimension=${encodeURIComponent(dimension)}`,
      ),
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
      exchange: (readerName: string, callbackUrl: string) =>
        apiClient.post(`/readers/${readerName}/auth/exchange`, { url: callbackUrl }),
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

    guide: (name: string) => apiClient.get<ReaderGuide>(`/readers/${name}/guide`),
  },

  thunderbird: {
    discoverProfiles: () =>
      apiClient.get<{ profiles: string[] }>('/readers/thunderbird/discover/profiles'),
    discoverMailboxes: (profile: string) =>
      apiClient.get<{ mailboxes: string[] }>(
        `/readers/thunderbird/discover/mailboxes?profile=${encodeURIComponent(profile)}`,
      ),
  },

  banks: {
    list: () => apiClient.get<Array<BankColor>>('/config/banks'),
  },

  config: {
    getActiveReader: () => apiClient.get<{ reader: string }>('/config/active-reader'),
    getSetupStatus: () => apiClient.get<SetupStatus>('/config/setup-status'),
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
    getCheckpoint: (reader: string) =>
      apiClient.get<{ last_scan_at: string | null }>(`/config/readers/${reader}/checkpoint`),
    clearCheckpoint: (reader: string) => apiClient.delete(`/config/readers/${reader}/checkpoint`),
    getTimezone: () => apiClient.get<{ timezone: string }>('/config/timezone'),
    setTimezone: (timezone: string) =>
      apiClient.put<{ timezone: string }>('/config/timezone', { timezone }),
    getTimeFormat: () => apiClient.get<{ time_format: string }>('/config/time-format'),
    setTimeFormat: (time_format: string) =>
      apiClient.put<{ time_format: string }>('/config/time-format', { time_format }),

    labels: {
      list: () => apiClient.get<Label[]>('/config/labels'),
      create: (name: string, color: string) =>
        apiClient.post<{ name: string; color: string }>('/config/labels', { name, color }),
      update: (name: string, color: string) =>
        apiClient.put<{ name: string; color: string }>(
          `/config/labels/${encodeURIComponent(name)}`,
          { color },
        ),
      delete: (name: string, removeFromTransactions = false) =>
        apiClient.delete(`/config/labels/${encodeURIComponent(name)}`, {
          params: { remove_from_transactions: removeFromTransactions },
          data: { remove_from_transactions: removeFromTransactions },
        }),
      apply: (name: string, merchantPattern: string) =>
        apiClient.post<{ applied: number }>(`/config/labels/${encodeURIComponent(name)}/apply`, {
          merchant_pattern: merchantPattern,
        }),
      removeMerchant: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/labels/${encodeURIComponent(name)}/merchant`,
          {
            data: { merchant_pattern: merchantPattern },
          },
        ),
      mappings: () => apiClient.get<Record<string, string[]>>('/config/labels/mappings'),
      export: () => apiClient.get('/config/labels/export', { responseType: 'blob' }),
    },

    categories: {
      list: () => apiClient.get<Category[]>('/config/categories'),
      create: (name: string, description?: string) =>
        apiClient.post<{ name: string }>('/config/categories', { name, description }),
      delete: (name: string, removeFromTransactions = false) =>
        apiClient.delete(`/config/categories/${encodeURIComponent(name)}`, {
          params: { remove_from_transactions: removeFromTransactions },
          data: { remove_from_transactions: removeFromTransactions },
        }),
      apply: (name: string, merchantPattern: string) =>
        apiClient.post<{ applied: number }>(
          `/config/categories/${encodeURIComponent(name)}/apply`,
          { merchant_pattern: merchantPattern },
        ),
      removeMerchant: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/categories/${encodeURIComponent(name)}/merchant`,
          {
            data: { merchant_pattern: merchantPattern },
          },
        ),
      mappings: () => apiClient.get<Record<string, string[]>>('/config/categories/mappings'),
      export: () => apiClient.get('/config/categories/export', { responseType: 'blob' }),
    },

    buckets: {
      list: () => apiClient.get<Bucket[]>('/config/buckets'),
      create: (name: string, description?: string) =>
        apiClient.post<{ name: string }>('/config/buckets', { name, description }),
      delete: (name: string, removeFromTransactions = false) =>
        apiClient.delete(`/config/buckets/${encodeURIComponent(name)}`, {
          params: { remove_from_transactions: removeFromTransactions },
          data: { remove_from_transactions: removeFromTransactions },
        }),
      apply: (name: string, merchantPattern: string) =>
        apiClient.post<{ applied: number }>(`/config/buckets/${encodeURIComponent(name)}/apply`, {
          merchant_pattern: merchantPattern,
        }),
      removeMerchant: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/buckets/${encodeURIComponent(name)}/merchant`,
          {
            data: { merchant_pattern: merchantPattern },
          },
        ),
      mappings: () => apiClient.get<Record<string, string[]>>('/config/buckets/mappings'),
      export: () => apiClient.get('/config/buckets/export', { responseType: 'blob' }),
    },
  },

  sync: {
    trigger: () => apiClient.post<{ status: string }>('/config/sync'),
    status: () => apiClient.get<SyncStatus>('/config/sync/status'),
  },

  rules: {
    list: () => apiClient.get<Rule[]>('/rules'),
    create: (body: Omit<Rule, 'id' | 'predefined' | 'created_at' | 'updated_at'>) =>
      apiClient.post<Rule>('/rules', body),
    update: (
      id: string,
      body: Partial<Omit<Rule, 'id' | 'predefined' | 'created_at' | 'updated_at'>>,
    ) => apiClient.put<Rule>(`/rules/${id}`, body),
    delete: (id: string) => apiClient.delete(`/rules/${id}`),
    export: () => apiClient.get<RuleImport[]>('/rules/export'),
    import: (rules: RuleImport[]) => apiClient.post<{ imported: number }>('/rules/import', rules),
  },

  extractionDiagnostics: {
    list: (status: ExtractionDiagnosticListStatus = 'open') =>
      apiClient.get<ExtractionDiagnostic[]>(
        `/extraction-diagnostics?status=${encodeURIComponent(status)}`,
      ),
    get: (id: string) => apiClient.get<ExtractionDiagnostic>(`/extraction-diagnostics/${id}`),
    updateStatus: (id: string, status: ExtractionDiagnosticStatus) =>
      apiClient.put<ExtractionDiagnostic>(`/extraction-diagnostics/${id}/status`, { status }),
  },

  transactions: {
    list: (filters: TransactionFilters = {}) => {
      const params = new URLSearchParams()
      if (filters.page) params.set('page', String(filters.page))
      if (filters.page_size) params.set('page_size', String(filters.page_size))
      if (filters.category) params.set('category', filters.category)
      if (filters.category_missing) params.set('category_missing', '1')
      if (filters.exclude_categories?.length)
        params.set('exclude_categories', filters.exclude_categories.join(','))
      if (filters.currency) params.set('currency', filters.currency)
      if (filters.source) params.set('source', filters.source)
      if (filters.exclude_sources?.length)
        params.set('exclude_sources', filters.exclude_sources.join(','))
      if (filters.bucket) params.set('bucket', filters.bucket)
      if (filters.bucket_missing) params.set('bucket_missing', '1')
      if (filters.exclude_buckets?.length)
        params.set('exclude_buckets', filters.exclude_buckets.join(','))
      if (filters.merchant) params.set('merchant', filters.merchant)
      if (filters.label) params.set('label', filters.label)
      if (filters.label_missing) params.set('label_missing', '1')
      if (filters.exclude_labels?.length)
        params.set('exclude_labels', filters.exclude_labels.join(','))
      if (filters.date_from) params.set('date_from', filters.date_from)
      if (filters.date_to) params.set('date_to', filters.date_to)
      if (filters.sort_by) params.set('sort_by', filters.sort_by)
      if (filters.sort_dir) params.set('sort_dir', filters.sort_dir)
      if (filters.show_muted) params.set('show_muted', '1')
      if (filters.muted_only) params.set('muted_only', '1')
      if (filters.individual_only) params.set('individual_only', '1')
      if (filters.hour_from !== undefined && filters.hour_from >= 0)
        params.set('hour_from', String(filters.hour_from))
      if (filters.hour_to !== undefined && filters.hour_to >= 0)
        params.set('hour_to', String(filters.hour_to))
      if (filters.weekday !== undefined) params.set('weekday', String(filters.weekday))
      if (filters.tz) params.set('tz', filters.tz)
      return apiClient.get<TransactionsResponse>(`/transactions?${params.toString()}`)
    },

    search: (q: string, page = 1, pageSize = 20, showMuted = false) => {
      const params = new URLSearchParams({ q, page: String(page), page_size: String(pageSize) })
      if (showMuted) params.set('show_muted', '1')
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

    mute: (id: string, muted: boolean, reason?: string) =>
      apiClient.put<{ muted: boolean }>(`/transactions/${id}/mute`, { muted, reason }),

    updateMuteReason: (id: string, reason: string) =>
      apiClient.put<{ reason: string }>(`/transactions/${id}/mute-reason`, { reason }),
  },

  mutedMerchants: {
    list: () => apiClient.get<MutedMerchantWithCount[]>('/muted-merchants'),
    add: (pattern: string, reason?: string) =>
      apiClient.post<{ pattern: string }>('/muted-merchants', { pattern, reason }),
    updateReason: (id: string, reason: string) =>
      apiClient.put<{ reason: string }>(`/muted-merchants/${id}/reason`, { reason }),
    delete: (id: string, unmute = false) =>
      apiClient.delete(`/muted-merchants/${id}${unmute ? '?unmute=true' : ''}`),
  },

  merchants: {
    categorize: (merchant: string, category: string, bucket: string) =>
      apiClient.post<{ updated: number }>('/merchants/categorize', { merchant, category, bucket }),
  },
}
