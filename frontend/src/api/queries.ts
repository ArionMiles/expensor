import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type {
  BankColor,
  DashboardData,
  ExtractionDiagnosticListStatus,
  ExtractionDiagnosticStatus,
  MonthlyBreakdownData,
  RuleDocument,
  RulePayload,
  SyncStatus,
  TransactionFilters,
  TransactionPatch,
} from './types'

export const queryKeys = {
  health: ['health'] as const,
  status: ['status'] as const,
  chartData: ['stats', 'charts'] as const,
  dashboardData: ['stats', 'dashboard'] as const,
  heatmap: (from?: string, to?: string) => ['stats', 'heatmap', from ?? null, to ?? null] as const,
  annualHeatmap: (year: number) => ['stats', 'heatmap', 'annual', year] as const,
  readers: ['plugins', 'readers'] as const,
  readerCredentialsStatus: (name: string) => ['readers', name, 'credentials', 'status'] as const,
  readerAuthStatus: (name: string) => ['readers', name, 'auth', 'status'] as const,
  readerConfig: (name: string) => ['readers', name, 'config'] as const,
  readerStatus: (name: string) => ['readers', name, 'status'] as const,
  facets: ['transactions', 'facets'] as const,
  transactions: (filters: TransactionFilters) => ['transactions', filters] as const,
  transactionSearch: (q: string, page: number, pageSize: number) =>
    ['transactions', 'search', q, page, pageSize] as const,
  transaction: (id: string) => ['transactions', id] as const,
  extractionDiagnostics: (status: ExtractionDiagnosticListStatus) =>
    ['extraction-diagnostics', status] as const,
  extractionDiagnostic: (id: string | null) => ['extraction-diagnostics', id] as const,
  labels: ['config', 'labels'] as const,
  categories: ['config', 'categories'] as const,
  buckets: ['config', 'buckets'] as const,
  setupStatus: ['config', 'setup-status'] as const,
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
    staleTime: 30_000,
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
  const page = filters.page ?? 1
  const pageSize = filters.page_size ?? 20

  return useQuery({
    queryKey: isSearch
      ? queryKeys.transactionSearch(searchQuery, page, pageSize)
      : queryKeys.transactions(filters),
    queryFn: isSearch
      ? () =>
          api.transactions
            .search(searchQuery, page, pageSize, !!filters.show_muted)
            .then((r) => r.data)
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
      api.config.labels.apply(name, pattern),
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
      api.config.labels.removeMerchant(name, pattern),
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
      api.config.categories.apply(name, pattern),
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
      api.config.categories.removeMerchant(name, pattern),
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
      api.config.buckets.apply(name, pattern),
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
      api.config.buckets.removeMerchant(name, pattern),
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
    mutationFn: ({
      id,
      body,
    }: {
      id: string
      body: Partial<RulePayload>
    }) => api.rules.update(id, body).then((r) => r.data),
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
    mutationFn: (reader: string) => api.daemon.rescan(reader).then((r) => r.data),
  })
}

export function useScanInterval() {
  return useQuery({
    queryKey: ['config', 'scan-interval'] as const,
    queryFn: () => api.config.getScanInterval().then((r) => Number(r.data.scan_interval)),
    staleTime: 60_000,
  })
}

export function useSetScanInterval() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (seconds: number) => api.config.setScanInterval(seconds).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['config', 'scan-interval'] }),
  })
}

export function useLookbackDays() {
  return useQuery({
    queryKey: ['config', 'lookback-days'] as const,
    queryFn: () => api.config.getLookbackDays().then((r) => Number(r.data.lookback_days)),
    staleTime: 60_000,
  })
}

export function useSetLookbackDays() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (days: number) => api.config.setLookbackDays(days).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['config', 'lookback-days'] }),
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
    queryKey: ['config', 'timezone'] as const,
    queryFn: () => api.config.getTimezone().then((r) => r.data.timezone),
    staleTime: Infinity,
  })
}

export function useSetTimezone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (timezone: string) => api.config.setTimezone(timezone).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['config', 'timezone'] }),
  })
}

export function useTimeFormat() {
  return useQuery({
    queryKey: ['config', 'time-format'] as const,
    queryFn: () => api.config.getTimeFormat().then((r) => r.data.time_format),
    staleTime: Infinity,
  })
}

export function useSetTimeFormat() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (timeFormat: string) => api.config.setTimeFormat(timeFormat).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['config', 'time-format'] }),
  })
}

export function useActiveReader() {
  return useQuery({
    queryKey: ['config', 'active-reader'] as const,
    queryFn: () => api.config.getActiveReader().then((r) => r.data.reader),
    staleTime: 60_000,
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
