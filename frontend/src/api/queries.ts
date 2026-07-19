import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type {
  AdminUserPatch,
  AdminLoggingSettingsPatch,
  BootstrapRequest,
  CompleteAccountSetupRequest,
  BankColor,
  AdminScanningSettingsPatch,
  CommunitySyncSettings,
  CommunitySyncSettingsPatch,
  CreateUserRequest,
  DashboardData,
  ExtractionDiagnosticListStatus,
  ExtractionDiagnosticStatus,
  LoginRequest,
  LLMProviderConfig,
  LLMProviderStatus,
  MonthlyBreakdownData,
  PasswordPatch,
  PreferencesPatch,
  ProfilePatch,
  RuleDocument,
  RuleDraftRequest,
  RulePayload,
  ScanningSettingsPatch,
  SyncStatus,
  TransactionFilters,
  TransactionPatch,
} from './types'

export const queryKeys = {
  bootstrap: ['auth', 'bootstrap'] as const,
  session: ['auth', 'session'] as const,
  accessTokens: ['auth', 'tokens'] as const,
  adminUsers: ['auth', 'admin', 'users'] as const,
  adminScanningSettings: ['auth', 'admin', 'scanning-settings'] as const,
  adminLoggingSettings: ['auth', 'admin', 'logging-settings'] as const,
  communitySyncSettings: ['config', 'sync', 'settings'] as const,
  health: ['health'] as const,
  status: ['status'] as const,
  scanningStatus: ['scanning', 'status'] as const,
  scanningSettings: ['scanning', 'settings'] as const,
  chartData: ['stats', 'charts'] as const,
  dashboardData: ['stats', 'dashboard'] as const,
  preferences: ['config', 'preferences'] as const,
  heatmap: (from?: string, to?: string) => ['stats', 'heatmap', from ?? null, to ?? null] as const,
  annualHeatmap: (year: number) => ['stats', 'heatmap', 'annual', year] as const,
  readers: ['plugins', 'readers'] as const,
  readerCredentialsStatus: (name: string) => ['readers', name, 'credentials', 'status'] as const,
  readerAuthStatus: (name: string) => ['readers', name, 'auth', 'status'] as const,
  readerConfig: (name: string) => ['readers', name, 'config'] as const,
  readerStatus: (name: string) => ['readers', name, 'status'] as const,
  facets: ['transactions', 'facets'] as const,
  transactions: (filters: TransactionFilters) => ['transactions', filters] as const,
  transactionSearch: (q: string, filters: TransactionFilters) =>
    ['transactions', 'search', q, filters] as const,
  transaction: (id: string) => ['transactions', id] as const,
  extractionDiagnostics: (status: ExtractionDiagnosticListStatus) =>
    ['extraction-diagnostics', status] as const,
  extractionDiagnostic: (id: string | null) => ['extraction-diagnostics', id] as const,
  labels: ['config', 'labels'] as const,
  categories: ['config', 'categories'] as const,
  buckets: ['config', 'buckets'] as const,
  setupStatus: ['config', 'setup-status'] as const,
  activeReader: ['scanning', 'settings', 'active-reader'] as const,
  llmProviders: ['llm', 'providers'] as const,
  llmProviderStatus: (name: string) => ['llm', 'providers', name, 'status'] as const,
}

export function useBootstrapStatus() {
  return useQuery({
    queryKey: queryKeys.bootstrap,
    queryFn: () => api.auth.bootstrapStatus().then((r) => r.data),
    retry: false,
    staleTime: 0,
  })
}

export function useSession(enabled = true) {
  return useQuery({
    queryKey: queryKeys.session,
    queryFn: () => api.auth.session().then((r) => r.data),
    retry: false,
    enabled,
    staleTime: 0,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: LoginRequest) => api.auth.login(body).then((r) => r.data),
    onSuccess: (principal) => {
      qc.setQueryData(queryKeys.session, principal)
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: queryKeys.setupStatus })
    },
  })
}

export function useBootstrapAdmin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: BootstrapRequest) => api.auth.bootstrap(body).then((r) => r.data),
    onSuccess: (principal) => {
      qc.setQueryData(queryKeys.session, principal)
      qc.setQueryData(queryKeys.bootstrap, { required: false })
      qc.invalidateQueries({ queryKey: queryKeys.setupStatus })
    },
  })
}

export function useAccountSetup(token: string) {
  return useQuery({
    queryKey: ['auth', 'account-setup', token] as const,
    queryFn: () => api.auth.accountSetup(token).then((r) => r.data),
    enabled: token.length > 0,
    retry: false,
  })
}

export function useCompleteAccountSetup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CompleteAccountSetupRequest) =>
      api.auth.completeAccountSetup(body).then((r) => r.data),
    onSuccess: (principal) => {
      qc.setQueryData(queryKeys.session, principal)
      qc.invalidateQueries({ queryKey: queryKeys.setupStatus })
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.auth.logout(),
    onSuccess: () => {
      qc.removeQueries()
    },
  })
}

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: ProfilePatch) => api.auth.updateProfile(patch).then((r) => r.data),
    onSuccess: (principal) => {
      qc.setQueryData(queryKeys.session, principal)
    },
  })
}

export function useUpdatePassword() {
  return useMutation({
    mutationFn: (patch: PasswordPatch) => api.auth.updatePassword(patch),
  })
}

export function useAccessTokens() {
  return useQuery({
    queryKey: queryKeys.accessTokens,
    queryFn: () => api.auth.tokens.list().then((r) => r.data),
  })
}

export function useCreateAccessToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.auth.tokens.create(name).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.accessTokens })
    },
  })
}

export function useRevokeAccessToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.auth.tokens.revoke(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.accessTokens })
    },
  })
}

export function useAdminUsers(enabled = true) {
  return useQuery({
    queryKey: queryKeys.adminUsers,
    queryFn: () => api.auth.admin.users().then((r) => r.data),
    enabled,
  })
}

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateUserRequest) => api.auth.admin.createUser(body).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.adminUsers })
    },
  })
}

export function useUpdateAdminUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: AdminUserPatch }) =>
      api.auth.admin.updateUser(id, patch).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.adminUsers })
      qc.invalidateQueries({ queryKey: queryKeys.session })
    },
  })
}

export function useDeleteAdminUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.auth.admin.deleteUser(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.adminUsers })
    },
  })
}

export function useCreateSetupToken() {
  return useMutation({
    mutationFn: (id: string) => api.auth.admin.createSetupToken(id).then((r) => r.data),
  })
}

export function useVersion() {
  return useQuery({
    queryKey: ['version'] as const,
    queryFn: () => api.version.get().then((r) => r.data.version),
    staleTime: Infinity,
  })
}

export function useStatus(enabled = true) {
  return useQuery({
    queryKey: queryKeys.status,
    queryFn: () => api.status.get().then((r) => r.data),
    refetchInterval: 10_000,
    staleTime: 5_000,
    enabled,
  })
}

export function useSetupStatus() {
  return useQuery({
    queryKey: queryKeys.setupStatus,
    queryFn: () => api.config.getSetupStatus().then((r) => r.data),
    staleTime: 0,
  })
}

export function useChartData(enabled = true) {
  return useQuery({
    queryKey: queryKeys.chartData,
    queryFn: () => api.stats.charts().then((r) => r.data),
    staleTime: 300_000,
    refetchInterval: 300_000,
    enabled,
  })
}

export function useDashboardData(enabled = true) {
  return useQuery<DashboardData>({
    queryKey: queryKeys.dashboardData,
    queryFn: () => api.stats.dashboard().then((r) => r.data),
    staleTime: 300_000,
    refetchInterval: 300_000,
    enabled,
  })
}

export function useHeatmapData(from?: string, to?: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.heatmap(from, to),
    queryFn: () => api.stats.heatmap(from, to).then((r) => r.data),
    staleTime: 5 * 60 * 1000,
    placeholderData: (prev) => prev,
    enabled,
  })
}

export function useAnnualHeatmapData(year: number, enabled = true) {
  return useQuery({
    queryKey: queryKeys.annualHeatmap(year),
    queryFn: () => api.stats.annualHeatmap(year).then((r) => r.data),
    staleTime: 5 * 60 * 1000,
    placeholderData: (prev) => prev,
    enabled,
  })
}

export function useMonthlyBreakdownSpend(dimension: 'labels' | 'categories' | 'buckets') {
  return useQuery<MonthlyBreakdownData>({
    queryKey: ['monthly-breakdown-spend', dimension] as const,
    queryFn: () => api.stats.monthlyBreakdown(dimension).then((r) => r.data),
    staleTime: 300_000,
    placeholderData: (prev) => prev,
  })
}

export function useReaders() {
  return useQuery({
    queryKey: queryKeys.readers,
    queryFn: () => api.plugins.readers().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useReaderCredentialsStatus(name: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.readerCredentialsStatus(name),
    queryFn: () => api.readers.credentials.status(name).then((r) => r.data),
    enabled: enabled && name.length > 0,
  })
}

export function useReaderAuthStatus(name: string, pollInterval?: number, enabled = true) {
  return useQuery({
    queryKey: queryKeys.readerAuthStatus(name),
    queryFn: () => api.readers.auth.status(name).then((r) => r.data),
    enabled: enabled && name.length > 0,
    refetchInterval: pollInterval,
  })
}

export function useRevokeToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (readerName: string) => api.readers.auth.revoke(readerName),
    onSuccess: (_, readerName) => {
      qc.invalidateQueries({ queryKey: queryKeys.readerStatus(readerName) })
      qc.invalidateQueries({ queryKey: queryKeys.readerAuthStatus(readerName) })
    },
  })
}

export function useReaderConfig(name: string) {
  return useQuery({
    queryKey: queryKeys.readerConfig(name),
    queryFn: () => api.readers.config.get(name).then((r) => r.data),
    enabled: name.length > 0,
  })
}

export function useReaderStatus(name: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.readerStatus(name),
    queryFn: () => api.readers.status(name).then((r) => r.data),
    enabled: enabled && name.length > 0,
  })
}

export function useFacets() {
  return useQuery({
    queryKey: queryKeys.facets,
    queryFn: () => api.transactions.facets().then((r) => r.data),
    staleTime: 300_000,
  })
}

export function useTransactions(filters: TransactionFilters, searchQuery: string) {
  const isSearch = searchQuery.trim().length > 0

  return useQuery({
    queryKey: isSearch
      ? queryKeys.transactionSearch(searchQuery, filters)
      : queryKeys.transactions(filters),
    queryFn: isSearch
      ? () => api.transactions.search(searchQuery, filters).then((r) => r.data)
      : () => api.transactions.list(filters).then((r) => r.data),
    placeholderData: (prev) => prev,
  })
}

export function useUpdateTransactionDescription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, description }: { id: string; description: string }) =>
      api.transactions.update(id, { description }).then((r) => r.data),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: queryKeys.transaction(data.id) })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useAddLabels() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, labels }: { id: string; labels: string[] }) =>
      api.transactions.addLabels(id, labels).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useRemoveLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, label }: { id: string; label: string }) =>
      api.transactions.removeLabel(id, label).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useUploadCredentials() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ readerName, file }: { readerName: string; file: File }) =>
      api.readers.credentials.upload(readerName, file),
    onSuccess: (_, { readerName }) => {
      qc.invalidateQueries({ queryKey: queryKeys.readerCredentialsStatus(readerName) })
    },
  })
}

export function useDisconnectReader() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (readerName: string) => api.readers.disconnect(readerName),
    onSuccess: (_, readerName) => {
      qc.invalidateQueries({ queryKey: queryKeys.readerStatus(readerName) })
      qc.invalidateQueries({ queryKey: queryKeys.readerCredentialsStatus(readerName) })
      qc.invalidateQueries({ queryKey: queryKeys.readerAuthStatus(readerName) })
      qc.invalidateQueries({ queryKey: queryKeys.status })
      qc.invalidateQueries({ queryKey: queryKeys.activeReader })
    },
  })
}

export function useSaveReaderConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ readerName, config }: { readerName: string; config: Record<string, string> }) =>
      api.readers.config.save(readerName, config),
    onSuccess: (_, { readerName }) => {
      qc.invalidateQueries({ queryKey: queryKeys.readerConfig(readerName) })
      qc.invalidateQueries({ queryKey: queryKeys.readerStatus(readerName) })
    },
  })
}

export function useLLMProviders() {
  return useQuery({
    queryKey: queryKeys.llmProviders,
    queryFn: () => api.llm.providers().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useLLMProviderStatus(name: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.llmProviderStatus(name),
    queryFn: () => api.llm.status(name).then((r) => r.data),
    enabled: enabled && name.length > 0,
  })
}

export function useLLMProviderStatuses(names: string[]) {
  return useQueries({
    queries: names.map((name) => ({
      queryKey: queryKeys.llmProviderStatus(name),
      queryFn: () => api.llm.status(name).then((response) => response.data),
      enabled: name.length > 0,
    })),
  })
}

export function useActiveLLMProviderStatus() {
  const providersQuery = useLLMProviders()
  const providerNames = providersQuery.data?.map((provider) => provider.name) ?? []
  const statusQueries = useLLMProviderStatuses(providerNames)
  const statuses = statusQueries
    .map((query) => query.data)
    .filter((status): status is LLMProviderStatus => Boolean(status))
  const activeStatus = statuses.find((status) => status.active)

  return {
    data: activeStatus,
    provider: providersQuery.data?.find((provider) => provider.name === activeStatus?.name),
    isLoading: providersQuery.isLoading || statusQueries.some((query) => query.isLoading),
  }
}

export function useSaveLLMProviderConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, config }: { name: string; config: LLMProviderConfig }) =>
      api.llm.saveConfig(name, config),
    onSuccess: (_, { name }) => {
      qc.invalidateQueries({ queryKey: queryKeys.llmProviderStatus(name) })
    },
  })
}

export function useSaveLLMProviderCredentials() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, apiKey }: { name: string; apiKey: string }) =>
      api.llm.saveCredentials(name, apiKey),
    onSuccess: (_, { name }) => {
      qc.invalidateQueries({ queryKey: queryKeys.llmProviderStatus(name) })
    },
  })
}

export function useHealthCheckLLMProvider() {
  return useMutation({
    mutationFn: (name: string) => api.llm.healthcheck(name).then((r) => r.data),
  })
}

export function useActivateLLMProvider() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.llm.activate(name).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.llmProviders })
    },
  })
}

export function useDisconnectLLMProvider() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.llm.disconnect(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.llmProviders })
    },
  })
}

export function useDraftRule() {
  return useMutation({
    mutationFn: (body: RuleDraftRequest) => api.ruleDrafts.create(body).then((r) => r.data),
  })
}

export function useLabels() {
  return useQuery({
    queryKey: queryKeys.labels,
    queryFn: () => api.config.labels.list().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useLabelMappings() {
  return useQuery({
    queryKey: ['label-mappings'] as const,
    queryFn: () => api.config.labels.mappings().then((r) => r.data as Record<string, string[]>),
    staleTime: 60_000,
  })
}

export function useCategoryMappings() {
  return useQuery({
    queryKey: ['category-mappings'] as const,
    queryFn: () => api.config.categories.mappings().then((r) => r.data as Record<string, string[]>),
    staleTime: 60_000,
  })
}

export function useBucketMappings() {
  return useQuery({
    queryKey: ['bucket-mappings'] as const,
    queryFn: () => api.config.buckets.mappings().then((r) => r.data as Record<string, string[]>),
    staleTime: 60_000,
  })
}

export function useCategories() {
  return useQuery({
    queryKey: queryKeys.categories,
    queryFn: () => api.config.categories.list().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useBuckets() {
  return useQuery({
    queryKey: queryKeys.buckets,
    queryFn: () => api.config.buckets.list().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useCreateLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, color }: { name: string; color: string }) =>
      api.config.labels.create(name, color),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.labels })
      qc.invalidateQueries({ queryKey: ['label-mappings'] })
    },
  })
}

export function useUpdateLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, color }: { name: string; color: string }) =>
      api.config.labels.update(name, color),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.labels }),
  })
}

export function useDeleteLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      name,
      removeFromTransactions,
    }: {
      name: string
      removeFromTransactions?: boolean
    }) => api.config.labels.delete(name, !!removeFromTransactions),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.labels })
      qc.invalidateQueries({ queryKey: ['label-mappings'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useApplyLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.labels.putMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['label-mappings'] })
    },
  })
}

export function useRemoveLabelByMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.labels.deleteMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['label-mappings'] })
    },
  })
}

export function useCreateCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, description }: { name: string; description?: string }) =>
      api.config.categories.create(name, description),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.categories })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
    },
  })
}

export function useDeleteCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      name,
      removeFromTransactions,
    }: {
      name: string
      removeFromTransactions?: boolean
    }) => api.config.categories.delete(name, !!removeFromTransactions),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.categories })
      qc.invalidateQueries({ queryKey: ['category-mappings'] })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useApplyCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.categories.putMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['category-mappings'] })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
    },
  })
}

export function useRemoveCategoryByMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.categories.deleteMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['category-mappings'] })
    },
  })
}

export function useCreateBucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, description }: { name: string; description?: string }) =>
      api.config.buckets.create(name, description),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.buckets })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
    },
  })
}

export function useDeleteBucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      name,
      removeFromTransactions,
    }: {
      name: string
      removeFromTransactions?: boolean
    }) => api.config.buckets.delete(name, !!removeFromTransactions),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.buckets })
      qc.invalidateQueries({ queryKey: ['bucket-mappings'] })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useApplyBucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.buckets.putMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['bucket-mappings'] })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
    },
  })
}

export function useRemoveBucketByMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.buckets.deleteMerchantMapping(name, pattern),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: ['bucket-mappings'] })
    },
  })
}

export function useUpdateTransactionFields() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: TransactionPatch }) =>
      api.transactions.update(id, patch).then((r) => r.data),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
      qc.invalidateQueries({ queryKey: queryKeys.transaction(data.id) })
    },
  })
}

export function useRules() {
  return useQuery({
    queryKey: ['rules'] as const,
    queryFn: () => api.rules.list().then((r) => r.data),
    staleTime: 30_000,
  })
}

export function useCreateRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: RulePayload) => api.rules.create(body).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rules'] }),
  })
}

export function useUpdateRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Partial<RulePayload> }) =>
      api.rules.update(id, body).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rules'] }),
  })
}

export function useDeleteRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.rules.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rules'] }),
  })
}

export function useSearchReaderMessages() {
  return useMutation({
    mutationFn: ({
      reader,
      subject,
      limit = 10,
    }: {
      reader: string
      subject: string
      limit?: number
    }) => api.readers.searchMessages(reader, subject, limit).then((r) => r.data),
  })
}

export function useExtractionDiagnostics(status: ExtractionDiagnosticListStatus = 'open') {
  return useQuery({
    queryKey: queryKeys.extractionDiagnostics(status),
    queryFn: () => api.extractionDiagnostics.list(status).then((r) => r.data),
  })
}

export function useExtractionDiagnostic(id: string | null) {
  return useQuery({
    queryKey: queryKeys.extractionDiagnostic(id),
    queryFn: () => api.extractionDiagnostics.get(id!).then((r) => r.data),
    enabled: Boolean(id),
  })
}

export function useUpdateExtractionDiagnosticStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: ExtractionDiagnosticStatus }) =>
      api.extractionDiagnostics.updateStatus(id, status).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['extraction-diagnostics'] }),
  })
}

export function useImportRules() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (rules: RuleDocument) => api.rules.import(rules).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rules'] }),
  })
}

export function useRescan() {
  return useMutation({
    mutationFn: (reader: string) => api.scanning.rescan(reader).then((r) => r.data),
  })
}

export function useScanningStatus() {
  return useQuery({
    queryKey: queryKeys.scanningStatus,
    queryFn: () => api.scanning.status().then((r) => r.data),
    refetchInterval: 30_000,
    staleTime: 15_000,
  })
}

export function useScanningSettings() {
  return useQuery({
    queryKey: queryKeys.scanningSettings,
    queryFn: () => api.scanning.settings().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useUpdateScanningSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: ScanningSettingsPatch) =>
      api.scanning.updateSettings(patch).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.scanningSettings })
      void qc.invalidateQueries({ queryKey: queryKeys.scanningStatus })
      void qc.invalidateQueries({ queryKey: queryKeys.activeReader })
    },
  })
}

export function usePreferences() {
  return useQuery({
    queryKey: queryKeys.preferences,
    queryFn: () => api.config.getPreferences().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useReaderCheckpoint(reader: string) {
  return useQuery({
    queryKey: ['config', 'checkpoint', reader] as const,
    queryFn: () => api.config.getCheckpoint(reader).then((r) => r.data.last_scan_at),
    enabled: reader.length > 0,
    staleTime: 30_000,
  })
}

export function useClearReaderCheckpoint() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (reader: string) => api.config.clearCheckpoint(reader),
    onSuccess: (_d, reader) => qc.invalidateQueries({ queryKey: ['config', 'checkpoint', reader] }),
  })
}

export function useTimezone() {
  return useQuery({
    queryKey: queryKeys.preferences,
    queryFn: () => api.config.getPreferences().then((r) => r.data),
    select: (preferences) => preferences.timezone,
    staleTime: Infinity,
  })
}

export function useTimeFormat() {
  return useQuery({
    queryKey: queryKeys.preferences,
    queryFn: () => api.config.getPreferences().then((r) => r.data),
    select: (preferences) => preferences.time_format,
    staleTime: Infinity,
  })
}

export function useUpdatePreferences() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: PreferencesPatch) =>
      api.config.updatePreferences(patch).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.preferences }),
  })
}

export function useActiveReader() {
  return useQuery({
    queryKey: queryKeys.activeReader,
    queryFn: () => api.scanning.settings().then((r) => r.data.active_reader),
    staleTime: 60_000,
  })
}

export function useAdminScanningSettings() {
  return useQuery({
    queryKey: queryKeys.adminScanningSettings,
    queryFn: () => api.auth.admin.scanningSettings().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useUpdateAdminScanningSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: AdminScanningSettingsPatch) =>
      api.auth.admin.updateScanningSettings(patch).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.adminScanningSettings }),
  })
}

export function useAdminLoggingSettings() {
  return useQuery({
    queryKey: queryKeys.adminLoggingSettings,
    queryFn: () => api.auth.admin.loggingSettings().then((r) => r.data),
    staleTime: 60_000,
  })
}

export function useUpdateAdminLoggingSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: AdminLoggingSettingsPatch) =>
      api.auth.admin.updateLoggingSettings(patch).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.adminLoggingSettings }),
  })
}

export function useReaderGuide(name: string) {
  return useQuery({
    queryKey: ['readers', name, 'guide'] as const,
    queryFn: () => api.readers.guide(name).then((r) => r.data),
    staleTime: Infinity,
    enabled: name.length > 0,
  })
}

export function useThunderbirdProfiles() {
  return useQuery({
    queryKey: ['thunderbird', 'profiles'] as const,
    queryFn: () => api.thunderbird.discoverProfiles().then((r) => r.data.profiles),
    staleTime: 60_000,
  })
}

export function useThunderbirdMailboxes(profilePath: string) {
  return useQuery({
    queryKey: ['thunderbird', 'mailboxes', profilePath] as const,
    queryFn: () => api.thunderbird.discoverMailboxes(profilePath).then((r) => r.data.mailboxes),
    staleTime: 60_000,
    enabled: profilePath.length > 0,
  })
}

export function useBanks() {
  return useQuery<BankColor[]>({
    queryKey: ['config', 'banks'] as const,
    queryFn: () => api.banks.list().then((r) => r.data),
    staleTime: Infinity, // static config — never stale
  })
}

export function useSyncStatus() {
  return useQuery<SyncStatus>({
    queryKey: ['config', 'sync', 'status'] as const,
    queryFn: () => api.sync.status().then((r) => r.data),
    staleTime: 30_000,
  })
}

export function useCommunitySyncSettings() {
  return useQuery<CommunitySyncSettings>({
    queryKey: queryKeys.communitySyncSettings,
    queryFn: () => api.sync.settings().then((r) => r.data),
    staleTime: 30_000,
  })
}

export function useUpdateCommunitySyncSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: CommunitySyncSettingsPatch) =>
      api.sync.updateSettings(patch).then((r) => r.data),
    onSuccess: (settings) => {
      qc.setQueryData(queryKeys.communitySyncSettings, settings)
    },
  })
}

export function useTriggerSync() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.sync.trigger().then((r) => r.data),
    onSuccess: () => {
      // Re-fetch status after a short delay to reflect new sync attempt
      setTimeout(() => {
        void qc.invalidateQueries({ queryKey: ['config', 'sync', 'status'] })
      }, 2000)
    },
  })
}

export function useIgnoredMerchants() {
  return useQuery({
    queryKey: ['muted-merchants'] as const,
    queryFn: () => api.mutedMerchants.list().then((r) => r.data),
  })
}

export function useIgnoreTransaction() {
  return useMutation({
    mutationFn: ({ id, muted, reason }: { id: string; muted: boolean; reason?: string }) =>
      api.transactions.mute(id, muted, reason).then((r) => r.data),
    // No auto-invalidation — callers control when to refresh the list.
  })
}

export function useUpdateIgnoreReason() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      api.transactions.updateMuteReason(id, reason).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }),
  })
}

export function useIgnoreByMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ pattern, reason }: { pattern: string; reason?: string }) =>
      api.mutedMerchants.add(pattern, reason).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['muted-merchants'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useBulkIgnoreTransactions() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ ids, reason }: { ids: string[]; reason?: string }) =>
      Promise.all(ids.map((id) => api.transactions.mute(id, true, reason).then((r) => r.data))),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useBulkIgnoreMerchants() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ patterns, reason }: { patterns: string[]; reason?: string }) =>
      Promise.all(
        [...new Set(patterns.map((pattern) => pattern.trim()).filter(Boolean))].map((pattern) =>
          api.mutedMerchants.add(pattern, reason).then((r) => r.data),
        ),
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['muted-merchants'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useUpdateMerchantReason() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      api.mutedMerchants.updateReason(id, reason).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['muted-merchants'] }),
  })
}

export function useDeleteMutedMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, unmute }: { id: string; unmute: boolean }) =>
      api.mutedMerchants.delete(id, unmute),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['muted-merchants'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
  })
}

export function useCategorizeMerchant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      merchant,
      category,
      bucket,
    }: {
      merchant: string
      category: string
      bucket: string
    }) => api.merchants.categorize(merchant, category, bucket).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }),
  })
}
