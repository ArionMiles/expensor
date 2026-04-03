export interface Transaction {
  id: string
  message_id: string
  amount: number
  currency: string
  original_amount?: number
  original_currency?: string
  exchange_rate?: number
  timestamp: string // RFC3339
  merchant_info: string
  category: string
  bucket: string
  source: string
  description: string
  labels: string[]
  created_at: string
  updated_at: string
}

export interface DaemonStatus {
  running: boolean
  started_at?: string
  last_error?: string
}

export interface CategoryStat {
  category: string
  amount: number
}

export interface TopMerchant {
  merchant: string
  amount: number
  count: number
}

export interface Stats {
  total_count: number
  total_inr: number
  total_by_category: Record<string, number> | null
  top_merchants: TopMerchant[] | null
}

export interface TimeBucket {
  period: string
  amount: number
  count: number
}

export interface ChartData {
  monthly_spend: TimeBucket[]
  daily_spend: TimeBucket[]
  by_category: Record<string, number>
  by_bucket: Record<string, number>
  by_label: Record<string, number>
}

export interface StatusResponse {
  daemon: DaemonStatus
  stats?: Stats
}

export interface ConfigField {
  name: string
  type: string
  label: string
  required: boolean
  placeholder?: string
}

export interface PluginInfo {
  name: string
  description: string
  auth_type: 'oauth' | 'config'
  requires_credentials_upload: boolean
  config_schema: ConfigField[]
}

export interface CredentialsStatus {
  exists: boolean
}

export interface AuthStartResponse {
  url: string
}

export interface AuthStatus {
  authenticated: boolean
  expiry?: string // RFC3339 — present when a token exists
}

export interface ReaderStatus {
  credentials_uploaded: boolean
  authenticated: boolean
  config_present: boolean
  auth_type: 'oauth' | 'config'
  ready: boolean
}

export interface ReaderConfig {
  config: Record<string, string>
}

export interface TransactionsResponse {
  transactions: Transaction[]
  total: number
}

export interface TransactionFilters {
  page?: number
  page_size?: number
  category?: string
  currency?: string
  label?: string
  date_from?: string
  date_to?: string
}

export interface HealthResponse {
  status: string
}
