import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type { TransactionFilters, TransactionPatch } from './types'

export const queryKeys = {
  health: ['health'] as const,
  status: ['status'] as const,
  chartData: ['stats', 'charts'] as const,
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
  labels: ['config', 'labels'] as const,
  categories: ['config', 'categories'] as const,
  buckets: ['config', 'buckets'] as const,
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

export function useChartData(enabled = true) {
  return useQuery({
    queryKey: queryKeys.chartData,
    queryFn: () => api.stats.charts().then((r) => r.data),
    staleTime: 300_000,
    refetchInterval: 300_000,
    enabled,
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
      ? () => api.transactions.search(searchQuery, page, pageSize).then((r) => r.data)
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
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.labels }),
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
    mutationFn: (name: string) => api.config.labels.delete(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.labels }),
  })
}

export function useApplyLabel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, pattern }: { name: string; pattern: string }) =>
      api.config.labels.apply(name, pattern),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }),
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
    mutationFn: (name: string) => api.config.categories.delete(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.categories })
      qc.invalidateQueries({ queryKey: queryKeys.facets })
    },
  })
}

export function useCreateBucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, description }: { name: string; description?: string }) =>
      api.config.buckets.create(name, description),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.buckets }),
  })
}

export function useDeleteBucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.config.buckets.delete(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.buckets }),
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
