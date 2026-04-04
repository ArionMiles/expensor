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
  total_base: number
  base_currency: string
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

export interface WeekdayHourBucket {
  weekday: number // 0=Sunday … 6=Saturday
  hour: number // 0–23
  amount: number
  count: number
}

export interface DayOfMonthBucket {
  day: number // 1–31
  amount: number
  count: number
}

export interface HeatmapData {
  by_weekday_hour: WeekdayHourBucket[]
  by_day_of_month: DayOfMonthBucket[]
}

export interface DailyBucket {
  date: string // RFC3339 date string — parse with new Date(b.date)
  amount: number
  count: number
}

export interface AnnualHeatmapData {
  year: number
  buckets: DailyBucket[]
}

export interface Rule {
  id: string
  name: string
  sender_email: string
  subject_contains: string
  amount_regex: string
  merchant_regex: string
  currency_regex: string
  transaction_source: string
  enabled: boolean
  source: 'system' | 'user'
  created_at: string
  updated_at: string
}

export interface RuleImport {
  name: string
  senderEmail: string
  subjectContains: string
  amountRegex: string
  merchantInfoRegex: string
  currencyRegex?: string
  enabled: boolean
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

export interface Facets {
  sources: string[]
  categories: string[]
  currencies: string[]
  labels: string[]
}

export interface TransactionFilters {
  page?: number
  page_size?: number
  category?: string
  currency?: string
  source?: string
  label?: string
  date_from?: string
  date_to?: string
  sort_by?: string
  sort_dir?: 'asc' | 'desc'
}

export interface HealthResponse {
  status: string
}

export interface Label {
  name: string
  color: string
  created_at: string
}

export interface Category {
  name: string
  description?: string
  is_default: boolean
}

export interface Bucket {
  name: string
  description?: string
  is_default: boolean
}

export interface TransactionPatch {
  description?: string
  category?: string
  bucket?: string
}
