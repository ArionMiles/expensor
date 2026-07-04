import axios from 'axios'
import type {
  AccessToken,
  AccountSetupMetadata,
  AnnualHeatmapData,
  AccountUser,
  AdminScanningSettings,
  AdminScanningSettingsPatch,
  AdminUserPatch,
  AuthStartResponse,
  AuthStatus,
  BankColor,
  BootstrapRequest,
  BootstrapStatus,
  Bucket,
  Category,
  ChartData,
  CompleteAccountSetupRequest,
  CreateUserRequest,
  CommunitySyncSettings,
  CommunitySyncSettingsPatch,
  DashboardData,
  CredentialsStatus,
  ExtractionDiagnostic,
  ExtractionDiagnosticListStatus,
  ExtractionDiagnosticStatus,
  Facets,
  HeatmapData,
  HealthResponse,
  Label,
  LoginRequest,
  MonthlyBreakdownData,
  MutedMerchantWithCount,
  PluginInfo,
  Preferences,
  PreferencesPatch,
  Principal,
  ProfilePatch,
  ReaderConfig,
  ReaderGuide,
  ReaderStatus,
  Rule,
  RuleDocument,
  RulePayload,
  ScanningSettings,
  ScanningSettingsPatch,
  ScanningStatus,
  SetupToken,
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

function transactionFilterParams(filters: TransactionFilters): URLSearchParams {
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
  if (filters.source_type) params.set('source_type', filters.source_type)
  if (filters.exclude_source_types?.length)
    params.set('exclude_source_types', filters.exclude_source_types.join(','))
  if (filters.bank) params.set('bank', filters.bank)
  if (filters.exclude_banks?.length) params.set('exclude_banks', filters.exclude_banks.join(','))
  if (filters.bucket) params.set('bucket', filters.bucket)
  if (filters.bucket_missing) params.set('bucket_missing', '1')
  if (filters.exclude_buckets?.length)
    params.set('exclude_buckets', filters.exclude_buckets.join(','))
  if (filters.merchant) params.set('merchant', filters.merchant)
  if (filters.label) params.set('label', filters.label)
  if (filters.label_missing) params.set('label_missing', '1')
  if (filters.exclude_labels?.length) params.set('exclude_labels', filters.exclude_labels.join(','))
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
  return params
}

export const api = {
  auth: {
    bootstrapStatus: () => apiClient.get<BootstrapStatus>('/bootstrap'),
    bootstrap: (body: BootstrapRequest) => apiClient.post<Principal>('/bootstrap', body),
    login: (body: LoginRequest) => apiClient.post<Principal>('/session', body),
    session: () => apiClient.get<Principal>('/session'),
    logout: () => apiClient.delete('/session'),
    updateProfile: (patch: ProfilePatch) => apiClient.patch<Principal>('/profile', patch),
    accountSetup: (token: string) =>
      apiClient.get<AccountSetupMetadata>(`/account-setup?token=${encodeURIComponent(token)}`),
    completeAccountSetup: (body: CompleteAccountSetupRequest) =>
      apiClient.post<Principal>('/account-setup', body),
    tokens: {
      list: () => apiClient.get<AccessToken[]>('/tokens'),
      create: (name: string) => apiClient.post<AccessToken>('/tokens', { name }),
      revoke: (id: string) => apiClient.delete(`/tokens/${encodeURIComponent(id)}`),
    },
    admin: {
      users: () => apiClient.get<AccountUser[]>('/admin/users'),
      createUser: (body: CreateUserRequest) => apiClient.post<AccountUser>('/admin/users', body),
      updateUser: (id: string, patch: AdminUserPatch) =>
        apiClient.patch<AccountUser>(`/admin/users/${encodeURIComponent(id)}`, patch),
      deleteUser: (id: string) => apiClient.delete(`/admin/users/${encodeURIComponent(id)}`),
      createSetupToken: (id: string) =>
        apiClient.post<SetupToken>(`/admin/users/${encodeURIComponent(id)}/setup-tokens`),
      scanningSettings: () => apiClient.get<AdminScanningSettings>('/admin/scanning/settings'),
      updateScanningSettings: (patch: AdminScanningSettingsPatch) =>
        apiClient.patch<AdminScanningSettings>('/admin/scanning/settings', patch),
    },
  },

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
      apiClient.get<AnnualHeatmapData>(`/stats/heatmap?year=${year}`),
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

  scanning: {
    status: () => apiClient.get<ScanningStatus>('/scanning/status'),
    settings: () => apiClient.get<ScanningSettings>('/scanning/settings'),
    updateSettings: (patch: ScanningSettingsPatch) =>
      apiClient.patch<ScanningSettings>('/scanning/settings', patch),
    rescan: (reader: string) =>
      apiClient.post<{ status: 'rescanning' | 'queued' }>('/scanning/rescans', { reader }),
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
        apiClient.put(`/readers/${readerName}/config`, { config }),
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
    getPreferences: () => apiClient.get<Preferences>('/config/preferences'),
    updatePreferences: (patch: PreferencesPatch) =>
      apiClient.patch<Preferences>('/config/preferences', patch),
    getCheckpoint: (reader: string) =>
      apiClient.get<{ last_scan_at: string | null }>(`/config/readers/${reader}/checkpoint`),
    clearCheckpoint: (reader: string) => apiClient.delete(`/config/readers/${reader}/checkpoint`),

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
      putMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.put<{ applied: number }>(
          `/config/labels/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
        ),
      deleteMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/labels/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
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
      putMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.put<{ applied: number }>(
          `/config/categories/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
        ),
      deleteMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/categories/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
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
      putMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.put<{ applied: number }>(
          `/config/buckets/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
        ),
      deleteMerchantMapping: (name: string, merchantPattern: string) =>
        apiClient.delete<{ removed: number }>(
          `/config/buckets/${encodeURIComponent(name)}/merchant-mappings/${encodeURIComponent(merchantPattern)}`,
        ),
      mappings: () => apiClient.get<Record<string, string[]>>('/config/buckets/mappings'),
      export: () => apiClient.get('/config/buckets/export', { responseType: 'blob' }),
    },
  },

  sync: {
    trigger: () => apiClient.post<{ status: string }>('/config/sync'),
    status: () => apiClient.get<SyncStatus>('/config/sync/status'),
    settings: () => apiClient.get<CommunitySyncSettings>('/config/sync/settings'),
    updateSettings: (patch: CommunitySyncSettingsPatch) =>
      apiClient.patch<CommunitySyncSettings>('/config/sync/settings', patch),
  },

  rules: {
    list: () => apiClient.get<Rule[]>('/rules'),
    create: (body: RulePayload) => apiClient.post<Rule>('/rules', body),
    update: (id: string, body: Partial<RulePayload>) => apiClient.put<Rule>(`/rules/${id}`, body),
    delete: (id: string) => apiClient.delete(`/rules/${id}`),
    export: () => apiClient.get<RuleDocument>('/rules/export'),
    import: (rules: RuleDocument) => apiClient.post<{ imported: number }>('/rules/import', rules),
  },

  extractionDiagnostics: {
    list: (status: ExtractionDiagnosticListStatus = 'open') =>
      apiClient.get<ExtractionDiagnostic[]>(
        `/extraction-diagnostics?status=${encodeURIComponent(status)}`,
      ),
    get: (id: string) => apiClient.get<ExtractionDiagnostic>(`/extraction-diagnostics/${id}`),
    updateStatus: (id: string, status: ExtractionDiagnosticStatus) =>
      apiClient.patch<ExtractionDiagnostic>(`/extraction-diagnostics/${id}`, { status }),
  },

  transactions: {
    list: (filters: TransactionFilters = {}) => {
      const params = transactionFilterParams(filters)
      return apiClient.get<TransactionsResponse>(`/transactions?${params.toString()}`)
    },

    search: (q: string, filters: TransactionFilters = {}) => {
      const params = transactionFilterParams(filters)
      params.set('q', q)
      return apiClient.get<TransactionsResponse>(`/transactions?${params.toString()}`)
    },

    facets: () => apiClient.get<Facets>('/transactions/facets'),

    get: (id: string) => apiClient.get<Transaction>(`/transactions/${id}`),

    update: (id: string, patch: TransactionPatch) =>
      apiClient.patch<Transaction>(`/transactions/${id}`, patch),

    addLabels: (id: string, labels: string[]) =>
      apiClient.post<Transaction>(`/transactions/${id}/labels`, { labels }),

    removeLabel: (id: string, label: string) =>
      apiClient.delete<Transaction>(`/transactions/${id}/labels/${encodeURIComponent(label)}`),

    mute: (id: string, muted: boolean, reason?: string) =>
      apiClient.patch<Transaction>(`/transactions/${id}`, { muted, mute_reason: reason }),

    updateMuteReason: (id: string, reason: string) =>
      apiClient.patch<Transaction>(`/transactions/${id}`, { mute_reason: reason }),
  },

  mutedMerchants: {
    list: () => apiClient.get<MutedMerchantWithCount[]>('/muted-merchants'),
    add: (pattern: string, reason?: string) =>
      apiClient.post<{ pattern: string }>('/muted-merchants', { pattern, reason }),
    updateReason: (id: string, reason: string) =>
      apiClient.patch<{ reason: string }>(`/muted-merchants/${id}`, { reason }),
    delete: (id: string, unmute = false) =>
      apiClient.delete(`/muted-merchants/${id}${unmute ? '?unmute=true' : ''}`),
  },

  merchants: {
    categorize: (merchant: string, category: string, bucket: string) =>
      apiClient.post<{ updated: number }>('/merchants/categorize', { merchant, category, bucket }),
  },
}
