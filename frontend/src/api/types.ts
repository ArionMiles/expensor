export interface Source {
  type: string
  label: string
  bank: string
}

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
  source: Source
  description: string
  labels: string[]
  muted: boolean
  muted_by_merchant: boolean
  mute_reason?: string
  created_at: string
  updated_at: string
}

export interface BankColor {
  fragment: string
  color: string
  name: string
}

export interface SyncStatus {
  last_synced_at: string | null
  error: string | null
  entries_updated: number
}

export interface CommunitySyncSettings {
  automatic_sync_enabled: boolean
}

export interface CommunitySyncSettingsPatch {
  automatic_sync_enabled?: boolean
}

export interface SetupStatus {
  required: boolean
  missing: Array<'base_currency' | 'timezone' | 'time_format'>
}

export type ScanningState =
  | 'queued'
  | 'starting'
  | 'running'
  | 'backing_off'
  | 'needs_auth'
  | 'reader_not_configured'
  | 'paused'
  | 'stopped'

export interface ScanningStatus {
  tenant_id?: string
  active_reader: string
  enabled: boolean
  state: ScanningState
  reason_code?: string
  public_message?: string
  last_started_at?: string
  last_stopped_at?: string
  last_failed_at?: string
  next_retry_at?: string
  retry_count: number
  updated_at: string
}

export interface ScanningSettings {
  active_reader: string
  enabled: boolean
}

export interface ScanningSettingsPatch {
  active_reader?: string
  enabled?: boolean
}

export interface AdminScanningSettings {
  max_concurrent_scans: number
  updated_at: string
}

export interface AdminScanningSettingsPatch {
  max_concurrent_scans?: number
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface AdminLoggingSettings {
  level: LogLevel
}

export interface AdminLoggingSettingsPatch {
  level: LogLevel
}

export interface BootstrapStatus {
  required: boolean
}

export type UserRole = 'admin' | 'user'
export type AvatarKey = 'default' | 'ledger' | 'wallet'

export interface Principal {
  user_id: string
  tenant_id: string
  email: string
  display_name: string
  role: UserRole
  avatar_key: AvatarKey
}

export interface BootstrapRequest {
  email: string
  password: string
  display_name: string
  avatar_key: AvatarKey
}

export interface CreateUserRequest {
  email: string
  role: UserRole
}

export interface LoginRequest {
  email: string
  password: string
}

export interface CompleteAccountSetupRequest {
  token: string
  display_name: string
  password: string
  avatar_key: AvatarKey
}

export interface AccountSetupMetadata {
  email: string
  avatar_key: AvatarKey
}

export interface ProfilePatch {
  display_name?: string
  avatar_key?: AvatarKey
}

export interface PasswordPatch {
  current_password: string
  new_password: string
}

export interface AccessToken {
  id: string
  name: string
  token?: string
  created_at: string
  expires_at?: string | null
  last_used_at?: string | null
}

export interface AccountUser {
  user_id: string
  tenant_id: string
  email: string
  display_name: string
  role: UserRole
  avatar_key: AvatarKey
  setup_pending: boolean
  disabled_at?: string | null
  created_at: string
  updated_at: string
}

export interface AdminUserPatch {
  display_name?: string
  role?: UserRole
  avatar_key?: AvatarKey
  disabled?: boolean
}

export interface SetupToken {
  token: string
  expires_at: string
}

export interface Preferences {
  base_currency: string
  scan_interval: number
  lookback_days: number
  timezone: string
  time_format: string
}

export type PreferencesPatch = Partial<Preferences>

export interface MutedMerchant {
  id: string
  pattern: string
  reason?: string
  created_at: string
}

export interface MutedMerchantWithCount extends MutedMerchant {
  muted_count: number
}

export interface MonthlyBreakdownSeries {
  label: string
  data: number[]
}

export interface MonthlyBreakdownData {
  labels: string[]
  months: string[] // YYYY-MM
  series: MonthlyBreakdownSeries[]
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
  total_category_count: Record<string, number> | null
  top_merchants: TopMerchant[] | null
}

export interface DashboardSection {
  label: string
  stats: Stats
  charts: ChartData
}

export interface DashboardData {
  current_month: DashboardSection
  all_time: DashboardSection
}

export interface TimeBucket {
  period: string
  amount: number
  count: number
}

export interface CategoryMonthlyEntry {
  current: number
  prior: number
}

export interface ChartData {
  monthly_spend: TimeBucket[]
  daily_spend: TimeBucket[]
  by_category: Record<string, number>
  by_bucket: Record<string, number>
  by_label: Record<string, number>
  by_source: Record<string, number>
  by_source_type: Record<string, number>
  by_bank: Record<string, number>
  by_category_monthly: Record<string, CategoryMonthlyEntry>
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
  sender_email?: string
  sender_emails: string[]
  subject_contains: string
  amount_regex: string
  merchant_regex: string
  currency_regex: string
  transaction_source?: string
  source: Source
  predefined: boolean
  created_at: string
  updated_at: string
}

export type RulePayload = Pick<
  Rule,
  | 'name'
  | 'sender_emails'
  | 'subject_contains'
  | 'amount_regex'
  | 'merchant_regex'
  | 'currency_regex'
  | 'source'
>

export interface ReaderMessageSample {
  id: string
  sender_email: string
  subject: string
  body: string
  received_at?: string
}

export interface ReaderMessageSearchResponse {
  results: ReaderMessageSample[]
}

export type ExtractionDiagnosticStatus = 'open' | 'resolved' | 'ignored'
export type ExtractionDiagnosticListStatus = ExtractionDiagnosticStatus | 'all'

export interface ExtractionDiagnostic {
  id: string
  status: ExtractionDiagnosticStatus
  reader: string
  message_id: string
  source: string
  sender: string
  sender_email: string
  subject: string
  email_body: string
  received_at?: string | null
  snippet: string
  rule_id?: string
  rule_name: string
  amount_regex: string
  merchant_regex: string
  currency_regex: string
  failure_reasons: string[]
  created_at: string
  updated_at: string
  resolved_at?: string | null
}

export interface RuleImport {
  name: string
  sender_emails: string[]
  subject_contains: string
  amount_regex: string
  merchant_regex: string
  currency_regex: string
  source: Source
}

export interface PresetValue {
  value: string
  origin: 'predefined' | 'custom'
}

export interface RuleDocument {
  version: 2
  presets: {
    source_types: PresetValue[]
    banks: PresetValue[]
  }
  rules: RuleImport[]
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
  help?: string
  depends_on?: string
  placeholder?: string
}

export interface PluginInfo {
  name: string
  description: string
  auth_type: 'oauth' | 'config'
  requires_credentials_upload: boolean
  config_schema: ConfigField[]
}

export interface GuideLink {
  label: string
  url: string
}

export interface GuideStep {
  text: string
  sub_steps?: string[]
}

export interface GuideSection {
  title: string
  steps: GuideStep[]
  link?: GuideLink
}

export interface GuideNote {
  type: 'info' | 'warning' | 'tip' | 'docker'
  text: string
}

export interface ReaderGuide {
  sections: GuideSection[]
  notes?: GuideNote[]
}

export interface CredentialsStatus {
  exists: boolean
}

export interface AuthStartResponse {
  url: string
  redirect_uri: string
}

export interface AuthStatus {
  authenticated: boolean
  auth_state: 'connected' | 'reauthorization_required' | 'refresh_pending'
  expiry?: string // RFC3339 — present when a token exists
}

export interface ReaderStatus {
  credentials_uploaded: boolean
  authenticated: boolean
  config_present: boolean
  auth_type: 'oauth' | 'config'
  auth_state: 'connected' | 'reauthorization_required' | 'refresh_pending'
  ready: boolean
}

export interface ReaderConfig {
  config: Record<string, string>
}

export interface TransactionsResponse {
  transactions: Transaction[]
  total: number
  total_amount: number
  base_currency: string
}

export interface Facets {
  sources: string[]
  source_types: string[]
  banks: string[]
  categories: string[]
  category_counts?: Record<string, number>
  currencies: string[]
  merchants?: string[]
  labels: string[]
  label_counts?: Record<string, number>
  buckets: string[]
  bucket_counts?: Record<string, number>
}

export interface TransactionFilters {
  page?: number
  page_size?: number
  category?: string
  category_missing?: boolean
  exclude_categories?: string[]
  currency?: string
  source?: string
  exclude_sources?: string[]
  source_type?: string
  exclude_source_types?: string[]
  bank?: string
  exclude_banks?: string[]
  bucket?: string
  bucket_missing?: boolean
  exclude_buckets?: string[]
  label?: string
  label_missing?: boolean
  exclude_labels?: string[]
  merchant?: string
  show_muted?: boolean
  muted_only?: boolean
  individual_only?: boolean
  hour_from?: number
  hour_to?: number
  date_from?: string
  date_to?: string
  sort_by?: string
  sort_dir?: 'asc' | 'desc'
  weekday?: number
  tz?: string
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
  muted?: boolean
  mute_reason?: string
}
