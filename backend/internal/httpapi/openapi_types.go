package httpapi

import "time"

// ErrorResponse is the standard JSON error payload for OpenAPI generation.
type ErrorResponse struct {
	Message          string            `json:"message" example:"Something went wrong." validate:"required"`
	RequestID        string            `json:"request_id" example:"7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3" validate:"required"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

// ValidationError describes one invalid request field.
type ValidationError struct {
	Field    string `json:"field" example:"page_size"`
	Location string `json:"location" example:"query"`
	Message  string `json:"message" example:"must be at most 100"`
}

type diagnosticListQuery struct {
	Status string `form:"status" validate:"omitempty,oneof=open resolved ignored all"`
	Limit  *int   `form:"limit" validate:"omitempty,min=1"`
}

type heatmapQuery struct {
	From *time.Time `form:"from"`
	To   *time.Time `form:"to"`
	Year *int       `form:"year" validate:"omitempty,min=1"`
}

type mailboxDiscoveryQuery struct {
	Profile string `form:"profile" validate:"required,no_control_chars"`
}

type providerMessagesQuery struct {
	Subject string `form:"subject" validate:"required,no_control_chars"`
	Limit   int    `form:"limit" validate:"omitempty,min=1,max=50"`
}

type monthlyBreakdownQuery struct {
	Dimension string `form:"dimension" validate:"omitempty,oneof=labels categories buckets"`
}

type taxonomyCleanupQuery struct {
	RemoveFromTransactions bool `form:"remove_from_transactions"`
}

type deleteMutedMerchantQuery struct {
	Unmute bool `form:"unmute"`
}

type transactionListQuery struct {
	// Page intentionally uses zero for both omission and page=0; both normalize to page 1.
	Page               int        `form:"page" validate:"min=0"`
	PageSize           *int       `form:"page_size" validate:"omitempty,min=1,max=100"`
	Merchant           string     `form:"merchant" validate:"no_control_chars"`
	Category           string     `form:"category" validate:"no_control_chars"`
	CategoryMissing    string     `form:"category_missing" validate:"omitempty,oneof=1"`
	ExcludeCategories  string     `form:"exclude_categories" validate:"no_control_chars"`
	Currency           string     `form:"currency" validate:"no_control_chars"`
	Source             string     `form:"source" validate:"no_control_chars"`
	ExcludeSources     string     `form:"exclude_sources" validate:"no_control_chars"`
	SourceType         string     `form:"source_type" validate:"no_control_chars"`
	ExcludeSourceTypes string     `form:"exclude_source_types" validate:"no_control_chars"`
	Bank               string     `form:"bank" validate:"no_control_chars"`
	ExcludeBanks       string     `form:"exclude_banks" validate:"no_control_chars"`
	Label              string     `form:"label" validate:"no_control_chars"`
	LabelMissing       string     `form:"label_missing" validate:"omitempty,oneof=1"`
	ExcludeLabels      string     `form:"exclude_labels" validate:"no_control_chars"`
	Bucket             string     `form:"bucket" validate:"no_control_chars"`
	BucketMissing      string     `form:"bucket_missing" validate:"omitempty,oneof=1"`
	ExcludeBuckets     string     `form:"exclude_buckets" validate:"no_control_chars"`
	DateFrom           *time.Time `form:"date_from"`
	DateTo             *time.Time `form:"date_to"`
	ShowMuted          string     `form:"show_muted" validate:"omitempty,oneof=1"`
	MutedOnly          string     `form:"muted_only" validate:"omitempty,oneof=1"`
	IndividualOnly     string     `form:"individual_only" validate:"omitempty,oneof=1"`
	Weekday            *int       `form:"weekday" validate:"omitempty,min=0,max=6"`
	HourFrom           *int       `form:"hour_from" validate:"omitempty,min=0,max=23"`
	HourTo             *int       `form:"hour_to" validate:"omitempty,min=0,max=23"`
	Timezone           string     `form:"tz" validate:"omitempty,iana_timezone"`
	Query              string     `form:"q" validate:"no_control_chars"`
	SortBy             string     `form:"sort_by" validate:"omitempty,oneof=timestamp"`
	SortDir            string     `form:"sort_dir" validate:"omitempty,oneof=asc desc"`
}

// HealthResponse is the health check payload.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// VersionResponse is the version payload.
type VersionResponse struct {
	Version string `json:"version" example:"dev"`
}

// StatsResponse documents the stats payload embedded in status responses.
type StatsResponse struct {
	TotalCount         int                `json:"total_count"`
	TotalBase          float64            `json:"total_base"`
	BaseCurrency       string             `json:"base_currency" example:"INR"`
	TotalByCategory    map[string]float64 `json:"total_by_category"`
	TotalCategoryCount map[string]int     `json:"total_category_count"`
}

// StatusResponse documents the combined daemon and stats status payload.
type StatusResponse struct {
	Daemon DaemonStatus   `json:"daemon"`
	Stats  *StatsResponse `json:"stats,omitempty"`
}

// DaemonReaderRequest is the daemon start/rescan request body.
type DaemonReaderRequest struct {
	Reader string `json:"reader" validate:"required,no_control_chars" example:"thunderbird"`
}

// StatusOnlyResponse is a simple status message payload.
type StatusOnlyResponse struct {
	Status string `json:"status" example:"ok"`
}

type ScanningSettingsResponse struct {
	ActiveReader string `json:"active_reader" example:"gmail"`
	Enabled      bool   `json:"enabled" example:"true"`
}

type ScanningSettingsPatchRequest struct {
	ActiveReader *string `json:"active_reader" validate:"omitempty,no_control_chars" example:"gmail"`
	Enabled      *bool   `json:"enabled"`
}

type ScanningStatusResponse struct {
	TenantID      string     `json:"tenant_id,omitempty" example:"11111111-1111-1111-1111-111111111111"`
	ActiveReader  string     `json:"active_reader" example:"gmail"`
	Enabled       bool       `json:"enabled" example:"true"`
	State         string     `json:"state" example:"running"`
	ReasonCode    string     `json:"reason_code,omitempty" example:"needs_auth_invalid_grant"`
	PublicMessage string     `json:"public_message,omitempty" example:"Reconnect your reader account to continue scanning."`
	LastStartedAt *time.Time `json:"last_started_at,omitempty"`
	LastStoppedAt *time.Time `json:"last_stopped_at,omitempty"`
	LastFailedAt  *time.Time `json:"last_failed_at,omitempty"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	RetryCount    int        `json:"retry_count"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AdminScanningSettingsResponse struct {
	MaxConcurrentScans int       `json:"max_concurrent_scans" example:"4"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AdminScanningSettingsPatchRequest struct {
	MaxConcurrentScans *int `json:"max_concurrent_scans" validate:"omitempty,min=1,max=64" example:"4"`
}

type AdminLoggingSettingsResponse struct {
	Level string `json:"level" example:"info" enums:"debug,info,warn,error"`
}

type AdminLoggingSettingsPatchRequest struct {
	Level string `json:"level" validate:"required,oneof=debug info warn error" example:"debug" enums:"debug,info,warn,error"`
}

// RuleSourceResponse documents a structured rule source.
type RuleSourceResponse struct {
	Type  string `json:"type" validate:"required,no_control_chars" example:"Email"`
	Label string `json:"label" validate:"no_control_chars" example:"Contract"`
	Bank  string `json:"bank" validate:"required,no_control_chars" example:"Contract Bank"`
}

// RuleResponse documents a rule returned by the API.
type RuleResponse struct {
	ID                string             `json:"id,omitempty" example:"11111111-1111-1111-1111-111111111111"`
	Name              string             `json:"name" example:"Contract Rule"`
	SenderEmail       string             `json:"sender_email,omitempty" example:"contract@example.com"`
	SenderEmails      []string           `json:"sender_emails"`
	SubjectContains   string             `json:"subject_contains" example:"Contract transaction"`
	AmountRegex       string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex     string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex     string             `json:"currency_regex" example:"(INR)"`
	TransactionSource string             `json:"transaction_source,omitempty" example:"Email - Contract Bank"`
	SourceType        string             `json:"source_type,omitempty" example:"Email"`
	SourceLabel       string             `json:"source_label,omitempty" example:"Contract"`
	Bank              string             `json:"bank,omitempty" example:"Contract Bank"`
	Source            RuleSourceResponse `json:"source"`
	Predefined        bool               `json:"predefined"`
	CreatedAt         time.Time          `json:"created_at,omitempty"`
	UpdatedAt         time.Time          `json:"updated_at,omitempty"`
}

// RuleMutationRequest documents the create/update rule payload.
type RuleMutationRequest struct {
	Name            string             `json:"name" validate:"required,no_control_chars" example:"Contract Rule"`
	SenderEmails    []string           `json:"sender_emails" validate:"required,min=1,dive,required,email"`
	SubjectContains string             `json:"subject_contains" validate:"no_control_chars" example:"Contract transaction"`
	AmountRegex     string             `json:"amount_regex" validate:"required,regexp" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" validate:"required,regexp" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" validate:"omitempty,regexp" example:"(INR)"`
	Source          RuleSourceResponse `json:"source" validate:"required"`
}

// RulePresetValueResponse documents a rule preset taxonomy value.
type RulePresetValueResponse struct {
	Value  string `json:"value" example:"Email"`
	Origin string `json:"origin" example:"custom"`
}

// RulePresetsResponse documents rule document taxonomy presets.
type RulePresetsResponse struct {
	SourceTypes []RulePresetValueResponse `json:"source_types"`
	Banks       []RulePresetValueResponse `json:"banks"`
}

// RuleDocumentEntryResponse documents a rule entry in an import/export document.
type RuleDocumentEntryResponse struct {
	Name            string             `json:"name" validate:"required,no_control_chars" example:"Contract Import Rule"`
	SenderEmails    []string           `json:"sender_emails" validate:"required,min=1,dive,required,email"`
	SubjectContains string             `json:"subject_contains" validate:"no_control_chars" example:"Contract transaction"`
	AmountRegex     string             `json:"amount_regex" validate:"required,regexp" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" validate:"required,regexp" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" validate:"omitempty,regexp" example:"(INR)"`
	Source          RuleSourceResponse `json:"source" validate:"required"`
}

// RuleDocumentResponse documents a versioned rules import/export document.
type RuleDocumentResponse struct {
	Version int                         `json:"version" validate:"required,oneof=2" example:"2"`
	Presets RulePresetsResponse         `json:"presets"`
	Rules   []RuleDocumentEntryResponse `json:"rules" validate:"required,dive"`
}

// RuleImportResponse documents a rules import result.
type RuleImportResponse struct {
	Imported int `json:"imported" example:"1"`
}

type LLMProviderInfoResponse struct {
	Name           string                   `json:"name" example:"openai"`
	DisplayName    string                   `json:"display_name" example:"OpenAI"`
	APIKeyURL      string                   `json:"api_key_url,omitempty" example:"https://platform.openai.com/api-keys"`
	APIKeyLinkText string                   `json:"api_key_link_text,omitempty" example:"OpenAI dashboard"`
	DataUse        LLMProviderDataUse       `json:"data_use"`
	AuthType       string                   `json:"auth_type" example:"api_key"`
	Capabilities   []string                 `json:"capabilities" example:"text_generation,json_schema"`
	ConfigSchema   map[string]any           `json:"config_schema,omitempty"`
	ModelOptions   []LLMProviderModelOption `json:"model_options,omitempty"`
}

type LLMProviderDataUse struct {
	Mode      string `json:"mode" example:"no_training_by_default"`
	PolicyURL string `json:"policy_url" example:"https://platform.openai.com/docs/models/default-usage-policies-by-endpoint"`
}

type LLMProviderModelOption struct {
	ID          string `json:"id" example:"gpt-5.4-mini"`
	DisplayName string `json:"display_name" example:"GPT-5.4 mini"`
	Quality     string `json:"quality" example:"Balanced"`
	Cost        string `json:"cost" example:"Lower"`
	Description string `json:"description,omitempty" example:"Recommended for rule drafting: strong quality with lower per-use cost."`
	Recommended bool   `json:"recommended,omitempty" example:"true"`
}

type LLMProviderStatusResponse struct {
	Name              string                   `json:"name" example:"openai"`
	Config            LLMProviderConfigRequest `json:"config"`
	ConfigPresent     bool                     `json:"config_present"`
	CredentialsStored bool                     `json:"credentials_stored"`
	Active            bool                     `json:"active"`
	Ready             bool                     `json:"ready"`
}

type LLMProviderConfigSaveRequest struct {
	Config LLMProviderConfigRequest `json:"config" validate:"required"`
}

type LLMProviderConfigRequest struct {
	Model   string `json:"model" example:"gpt-5.4-mini"`
	BaseURL string `json:"base_url" example:"https://api.openai.com/v1"`
}

type LLMProviderCredentialsRequest struct {
	APIKey string `json:"api_key" validate:"required" example:"sk-..."`
}

type LLMProviderHealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Message string `json:"message,omitempty" example:"OpenAI connection is healthy."`
}

type RuleDraftExpectedRequest struct {
	Amount   string `json:"amount" example:"123.45"`
	Merchant string `json:"merchant" example:"Example Store"`
	Currency string `json:"currency" example:"INR"`
}

type RuleDraftSampleRequest struct {
	Name     string                   `json:"name" example:"Sample 1"`
	Sender   string                   `json:"sender" example:"alerts@example.com"`
	Subject  string                   `json:"subject" example:"Card spend alert"`
	Body     string                   `json:"body" example:"You spent INR 123.45 at Example Store."`
	Expected RuleDraftExpectedRequest `json:"expected"`
}

type RuleDraftRequest struct {
	Name            string                   `json:"name" example:"Example Bank Card"`
	SenderEmails    []string                 `json:"sender_emails"`
	SubjectContains string                   `json:"subject_contains" example:"Card spend"`
	AmountRegex     string                   `json:"amount_regex" example:""`
	MerchantRegex   string                   `json:"merchant_regex" example:""`
	CurrencyRegex   string                   `json:"currency_regex" example:""`
	Source          RuleSourceResponse       `json:"source"`
	Samples         []RuleDraftSampleRequest `json:"samples"`
}

type RuleDraftRuleResponse struct {
	Name            string             `json:"name" example:"Example Bank Card"`
	SenderEmails    []string           `json:"sender_emails"`
	SubjectContains string             `json:"subject_contains" example:"Card spend"`
	AmountRegex     string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" example:"(INR)"`
	Source          RuleSourceResponse `json:"source"`
	Notes           string             `json:"notes"`
}

type RuleDraftMatchResponse struct {
	SampleIndex int    `json:"sample_index" example:"0"`
	SampleName  string `json:"sample_name" example:"Sample 1"`
	Amount      string `json:"amount" example:"123.45"`
	Merchant    string `json:"merchant" example:"Example Store"`
	Currency    string `json:"currency" example:"INR"`
}

type RuleDraftIssueResponse struct {
	SampleIndex int    `json:"sample_index" example:"0"`
	SampleName  string `json:"sample_name" example:"Sample 1"`
	Field       string `json:"field" example:"amount"`
	Expected    string `json:"expected,omitempty" example:"123.45"`
	Actual      string `json:"actual,omitempty" example:"123.00"`
	Message     string `json:"message" example:"Amount matched \"123.00\", expected \"123.45\"."`
}

type RuleDraftResponse struct {
	Draft            RuleDraftRuleResponse    `json:"draft"`
	Matches          []RuleDraftMatchResponse `json:"matches"`
	ValidationIssues []RuleDraftIssueResponse `json:"validation_issues,omitempty"`
}

// ConfigFieldResponse documents provider configuration field metadata.
type ConfigFieldResponse struct {
	Name      string `json:"name" example:"profilePath"`
	Label     string `json:"label" example:"Profile path"`
	Type      string `json:"type" example:"text"`
	Required  bool   `json:"required"`
	Help      string `json:"help,omitempty" example:"Path to the Thunderbird profile"`
	DependsOn string `json:"depends_on,omitempty" example:"profilePath"`
}

// ProviderInfoResponse documents provider metadata.
type ProviderInfoResponse struct {
	Name                      string                `json:"name" example:"thunderbird"`
	Description               string                `json:"description" example:"Read transaction emails from Thunderbird"`
	AuthType                  string                `json:"auth_type" example:"config"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []ConfigFieldResponse `json:"config_schema"`
}

// ProviderSearchResultResponse documents a provider email search result for rule authoring.
type ProviderSearchResultResponse struct {
	ID          string     `json:"id" example:"1879f6d32a7f3c11"`
	SenderEmail string     `json:"sender_email" example:"alerts@example.com"`
	Subject     string     `json:"subject" example:"Card spend approved"`
	Body        string     `json:"body" example:"INR 42.00 at Coffee"`
	ReceivedAt  *time.Time `json:"received_at,omitempty"`
}

// ProviderSearchResponse documents provider message search results.
type ProviderSearchResponse struct {
	Results []ProviderSearchResultResponse `json:"results"`
}

// UploadCredentialsResponse documents a stored reader credentials location.
type UploadCredentialsResponse struct {
	Path string `json:"path" example:"db://reader_runtime/gmail/client_secret"`
}

// CredentialsStatusResponse documents whether reader credentials are present.
type CredentialsStatusResponse struct {
	Exists bool `json:"exists"`
}

// AuthStartResponse documents an OAuth authorization start response.
type AuthStartResponse struct {
	URL         string `json:"url" example:"https://accounts.google.com/o/oauth2/auth"`
	RedirectURI string `json:"redirect_uri" example:"http://localhost:8080/api/auth/callback"`
}

// AuthExchangeRequest documents a manual OAuth callback exchange request.
type AuthExchangeRequest struct {
	URL string `json:"url" validate:"required,url" example:"http://localhost:8080/api/auth/callback?state=state&code=code"`
}

// AuthExchangeResponse documents a successful manual OAuth exchange.
type AuthExchangeResponse struct {
	Status string `json:"status" example:"authorized"`
}

// AuthStatusResponse documents a reader auth status payload.
type AuthStatusResponse struct {
	Authenticated bool       `json:"authenticated"`
	AuthType      string     `json:"auth_type,omitempty" example:"config"`
	AuthState     string     `json:"auth_state" example:"connected"`
	Expiry        *time.Time `json:"expiry,omitempty" extensions:"x-nullable"`
}

// ProviderDisconnectResponse documents a provider disconnect result.
type ProviderDisconnectResponse struct {
	Status       string   `json:"status" example:"disconnected"`
	FilesRemoved []string `json:"files_removed"`
}

// ProviderTokenRevokeResponse documents an OAuth token revoke result.
type ProviderTokenRevokeResponse struct {
	Status string `json:"status" example:"revoked"`
}

// ProviderConfigRequest documents arbitrary persisted provider configuration JSON.
type ProviderConfigRequest map[string]any

// ProviderConfigResponse documents arbitrary persisted provider configuration JSON.
type ProviderConfigResponse map[string]any

// ProviderConfigSaveResponse documents a provider config save result.
type ProviderConfigSaveResponse struct {
	Status string `json:"status" example:"saved"`
}

// ProviderStatusResponse documents a provider readiness status payload.
type ProviderStatusResponse struct {
	CredentialsUploaded bool   `json:"credentials_uploaded"`
	Authenticated       bool   `json:"authenticated"`
	ConfigPresent       bool   `json:"config_present"`
	AuthType            string `json:"auth_type" example:"config"`
	AuthState           string `json:"auth_state" example:"connected"`
	Ready               bool   `json:"ready"`
}

// ThunderbirdProfilesResponse documents discovered Thunderbird profile paths.
type ThunderbirdProfilesResponse struct {
	Profiles []string `json:"profiles"`
}

// ThunderbirdMailboxesResponse documents discovered Thunderbird mailboxes.
type ThunderbirdMailboxesResponse struct {
	Mailboxes []string `json:"mailboxes"`
}

// ProviderGuideResponse documents a reader setup guide payload.
type ProviderGuideResponse struct {
	Sections []GuideSectionResponse `json:"sections"`
	Notes    []GuideNoteResponse    `json:"notes,omitempty"`
}

// GuideSectionResponse documents a titled group of setup guide steps.
type GuideSectionResponse struct {
	Title string              `json:"title" example:"Connect Thunderbird"`
	Steps []GuideStepResponse `json:"steps"`
	Link  *GuideLinkResponse  `json:"link,omitempty"`
}

// GuideStepResponse documents a single setup guide step.
type GuideStepResponse struct {
	Text     string   `json:"text" example:"Select your Thunderbird profile"`
	SubSteps []string `json:"sub_steps,omitempty"`
}

// GuideLinkResponse documents an optional setup guide link.
type GuideLinkResponse struct {
	Label string `json:"label" example:"Thunderbird profiles"`
	URL   string `json:"url" example:"https://support.mozilla.org/kb/profiles-where-thunderbird-stores-user-data"`
}

// GuideNoteResponse documents a setup guide callout.
type GuideNoteResponse struct {
	Type string `json:"type" example:"info"`
	Text string `json:"text" example:"Docker deployments need the Thunderbird profile mounted into the container."`
}

// TimeBucketResponse documents a single chart time-period data point.
type TimeBucketResponse struct {
	Period string  `json:"period" example:"2026-05"`
	Amount float64 `json:"amount" example:"249.50"`
	Count  int     `json:"count" example:"3"`
}

// CategoryMonthlyEntryResponse documents current/prior month spend for a category.
type CategoryMonthlyEntryResponse struct {
	Current float64 `json:"current" example:"1200.50"`
	Prior   float64 `json:"prior" example:"950.25"`
}

// ChartDataResponse documents chart stats data.
type ChartDataResponse struct {
	MonthlySpend      []TimeBucketResponse                    `json:"monthly_spend"`
	DailySpend        []TimeBucketResponse                    `json:"daily_spend"`
	ByCategory        map[string]float64                      `json:"by_category"`
	ByBucket          map[string]float64                      `json:"by_bucket"`
	ByLabel           map[string]float64                      `json:"by_label"`
	BySource          map[string]float64                      `json:"by_source"`
	BySourceType      map[string]float64                      `json:"by_source_type"`
	ByBank            map[string]float64                      `json:"by_bank"`
	ByCategoryMonthly map[string]CategoryMonthlyEntryResponse `json:"by_category_monthly"`
}

// DashboardSectionResponse documents a dashboard slice.
type DashboardSectionResponse struct {
	Label  string            `json:"label" example:"Current Month"`
	Stats  StatsResponse     `json:"stats"`
	Charts ChartDataResponse `json:"charts"`
}

// DashboardDataResponse documents dashboard stats data.
type DashboardDataResponse struct {
	CurrentMonth DashboardSectionResponse `json:"current_month"`
	AllTime      DashboardSectionResponse `json:"all_time"`
}

// WeekdayHourBucketResponse documents spend totals for a weekday/hour heatmap cell.
type WeekdayHourBucketResponse struct {
	Weekday int     `json:"weekday" example:"1"`
	Hour    int     `json:"hour" example:"14"`
	Amount  float64 `json:"amount" example:"249.50"`
	Count   int     `json:"count" example:"3"`
}

// DayOfMonthBucketResponse documents spend totals for a day-of-month heatmap cell.
type DayOfMonthBucketResponse struct {
	Day    int     `json:"day" example:"15"`
	Amount float64 `json:"amount" example:"249.50"`
	Count  int     `json:"count" example:"3"`
}

// DailyBucketResponse documents spend totals for a calendar date.
type DailyBucketResponse struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount" example:"249.50"`
	Count  int       `json:"count" example:"3"`
}

// HeatmapResponse documents either range-based or annual heatmap data.
type HeatmapResponse struct {
	ByWeekdayHour []WeekdayHourBucketResponse `json:"by_weekday_hour,omitempty"`
	ByDayOfMonth  []DayOfMonthBucketResponse  `json:"by_day_of_month,omitempty"`
	Year          int                         `json:"year,omitempty" example:"2026"`
	Buckets       []DailyBucketResponse       `json:"buckets,omitempty"`
}

// MonthlyBreakdownSeriesResponse documents a named monthly spend series.
type MonthlyBreakdownSeriesResponse struct {
	Label string    `json:"label" example:"Food"`
	Data  []float64 `json:"data"`
}

// MonthlyBreakdownResponse documents monthly breakdown stats data.
type MonthlyBreakdownResponse struct {
	Labels []string                         `json:"labels"`
	Months []string                         `json:"months"`
	Series []MonthlyBreakdownSeriesResponse `json:"series"`
}

// PreferencesPatchRequest is a partial application preferences update.
type PreferencesPatchRequest struct {
	BaseCurrency *string `json:"base_currency,omitempty" validate:"omitempty,currency_code" example:"USD" minLength:"3" maxLength:"3"`
	ScanInterval *int    `json:"scan_interval,omitempty" validate:"omitempty,min=10,max=3600" example:"120" minimum:"10" maximum:"3600"`
	LookbackDays *int    `json:"lookback_days,omitempty" validate:"omitempty,min=1,max=3650" example:"365" minimum:"1" maximum:"3650"`
	Timezone     *string `json:"timezone,omitempty" validate:"omitempty,iana_timezone" example:"Asia/Kolkata"`
	TimeFormat   *string `json:"time_format,omitempty" validate:"omitempty,time_format" example:"HH:mm" enums:"HH:mm,HH:mm:ss,h:mm a,h:mm:ss a"`
}

// PreferencesResponse is the effective application preferences payload.
type PreferencesResponse struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
	ScanInterval int    `json:"scan_interval" example:"120"`
	LookbackDays int    `json:"lookback_days" example:"365"`
	Timezone     string `json:"timezone" example:"Asia/Kolkata"`
	TimeFormat   string `json:"time_format" example:"HH:mm"`
}

// SetupStatusResponse is the first-run setup status payload.
type SetupStatusResponse struct {
	Required bool     `json:"required" example:"true"`
	Missing  []string `json:"missing" example:"base_currency,timezone,time_format"`
}

// ProviderCheckpointResponse is the reader checkpoint payload.
type ProviderCheckpointResponse struct {
	LastScanAt *string `json:"last_scan_at" example:"2026-04-14T09:00:00Z" extensions:"x-nullable"`
}

// SyncStatusResponse is the community sync status payload.
type SyncStatusResponse struct {
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty" extensions:"x-nullable"`
	Error          *string    `json:"error,omitempty" extensions:"x-nullable"`
	EntriesUpdated int64      `json:"entries_updated"`
}

// CommunitySyncSettingsResponse is the community sync settings payload.
type CommunitySyncSettingsResponse struct {
	AutomaticSyncEnabled bool `json:"automatic_sync_enabled" example:"true"`
}

// CommunitySyncSettingsPatchRequest updates community sync settings.
type CommunitySyncSettingsPatchRequest struct {
	AutomaticSyncEnabled *bool `json:"automatic_sync_enabled,omitempty" validate:"omitempty" example:"false"`
}

// LabelResponse documents a managed label.
type LabelResponse struct {
	Name      string    `json:"name" example:"food"`
	Color     string    `json:"color" example:"#f59e0b"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateLabelRequest is the label creation payload.
type CreateLabelRequest struct {
	Name  string `json:"name" validate:"required,no_control_chars" example:"ContractLabel"`
	Color string `json:"color" validate:"omitempty,hexcolor" example:"#f59e0b"`
}

// UpdateLabelRequest is the label update payload.
type UpdateLabelRequest struct {
	Color string `json:"color" validate:"required,hexcolor" example:"#f59e0b"`
}

// TaxonomyCleanupRequest controls whether deleted taxonomy values are cleared from transactions.
type TaxonomyCleanupRequest struct {
	RemoveFromTransactions bool `json:"remove_from_transactions"`
}

// LabelMutationResponse is the label create/update response payload.
type LabelMutationResponse struct {
	Name  string `json:"name" example:"ContractLabel"`
	Color string `json:"color" example:"#f59e0b"`
}

// TaxonomyExportRowResponse documents exported taxonomy rows with merchant mappings.
type TaxonomyExportRowResponse struct {
	Name      string   `json:"name" example:"Food & Dining"`
	Merchants []string `json:"merchants,omitempty"`
}

// LabelTaxonomyExportRowResponse documents exported labels with color and merchant mappings.
type LabelTaxonomyExportRowResponse struct {
	Name      string   `json:"name" example:"Online"`
	Color     string   `json:"color" example:"#f59e0b"`
	Merchants []string `json:"merchants,omitempty"`
}

// TaxonomyMappingsResponse documents taxonomy-to-merchant mappings.
type TaxonomyMappingsResponse map[string][]string

// AppliedCountResponse is the count payload for apply actions.
type AppliedCountResponse struct {
	Applied int64 `json:"applied"`
}

// RemovedCountResponse is the count payload for remove actions.
type RemovedCountResponse struct {
	Removed int64 `json:"removed"`
}

// LabelMappingsResponse documents label-to-merchant mappings.
type LabelMappingsResponse map[string][]string

// CategoryResponse documents a managed category.
type CategoryResponse struct {
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
	IsDefault   bool   `json:"is_default"`
}

// CreateCategoryRequest is the category creation payload.
type CreateCategoryRequest struct {
	Name        string `json:"name" validate:"required,no_control_chars" example:"ContractCategory"`
	Description string `json:"description,omitempty" validate:"no_control_chars" example:"Contract category"`
}

// BucketResponse documents a managed bucket.
type BucketResponse struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
	IsDefault   bool   `json:"is_default"`
}

// CreateBucketRequest is the bucket creation payload.
type CreateBucketRequest struct {
	Name        string `json:"name" validate:"required,no_control_chars" example:"ContractBucket"`
	Description string `json:"description,omitempty" validate:"no_control_chars" example:"Contract bucket"`
}

// NameResponse is a simple named resource payload.
type NameResponse struct {
	Name string `json:"name" example:"ContractCategory"`
}

// BankColorResponse documents an embedded bank color mapping.
type BankColorResponse struct {
	Fragment string `json:"fragment" example:"hdfc"`
	Color    string `json:"color" example:"#2563eb"`
	Name     string `json:"name" example:"HDFC Bank"`
}

// TransactionResponse documents a transaction payload.
type TransactionResponse struct {
	ID               string             `json:"id" example:"00000000-0000-0000-0000-000000000001"`
	MessageID        string             `json:"message_id" example:"gmail-message-id"`
	Amount           float64            `json:"amount" example:"249.50"`
	Currency         string             `json:"currency" example:"INR"`
	OriginalAmount   *float64           `json:"original_amount,omitempty"`
	OriginalCurrency *string            `json:"original_currency,omitempty"`
	ExchangeRate     *float64           `json:"exchange_rate,omitempty"`
	Timestamp        time.Time          `json:"timestamp"`
	MerchantInfo     string             `json:"merchant_info" example:"Swiggy"`
	Category         string             `json:"category" example:"Food & Dining"`
	Bucket           string             `json:"bucket" example:"Needs"`
	Source           RuleSourceResponse `json:"source"`
	Description      string             `json:"description" example:"Dinner order"`
	Labels           []string           `json:"labels"`
	Muted            bool               `json:"muted"`
	MutedByMerchant  bool               `json:"muted_by_merchant"`
	MuteReason       string             `json:"mute_reason,omitempty" example:"Internal transfer"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// TransactionsListResponse documents the paginated list payload.
type TransactionsListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Total        int                   `json:"total"`
	TotalAmount  float64               `json:"total_amount"`
	BaseCurrency string                `json:"base_currency" example:"INR"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"page_size"`
}

// ExtractionDiagnosticResponse documents an extraction diagnostic payload.
type ExtractionDiagnosticResponse struct {
	ID             string     `json:"id" example:"11111111-1111-1111-1111-111111111111"`
	Status         string     `json:"status" example:"open"`
	Reader         string     `json:"reader" example:"gmail"`
	MessageID      string     `json:"message_id" example:"gmail-message-id"`
	Source         string     `json:"source" example:"HDFC Bank"`
	Sender         string     `json:"sender" example:"HDFC Bank"`
	SenderEmail    string     `json:"sender_email" example:"alerts@hdfcbank.net"`
	Subject        string     `json:"subject" example:"Transaction alert"`
	EmailBody      string     `json:"email_body" example:"Your card was charged INR 249.50 at Swiggy"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	Snippet        string     `json:"snippet" example:"Your card was charged INR 249.50 at Swiggy"`
	RuleID         *string    `json:"rule_id,omitempty" example:"22222222-2222-2222-2222-222222222222"`
	RuleName       string     `json:"rule_name" example:"HDFC credit card"`
	AmountRegex    string     `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex  string     `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex  string     `json:"currency_regex" example:"(INR)"`
	FailureReasons []string   `json:"failure_reasons" example:"amount_not_found"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// ExtractionDiagnosticStatusRequest is the diagnostic status update payload.
type ExtractionDiagnosticStatusRequest struct {
	Status string `json:"status" example:"resolved" validate:"required,oneof=open resolved ignored" enums:"open,resolved,ignored"`
}

// FacetsResponse documents the distinct transaction filter values.
type FacetsResponse struct {
	Sources    []string `json:"sources"`
	Categories []string `json:"categories"`
	Currencies []string `json:"currencies"`
	Merchants  []string `json:"merchants"`
	Labels     []string `json:"labels"`
	Buckets    []string `json:"buckets"`
}

// TransactionUpdateRequest is the transaction patch payload.
type TransactionUpdateRequest struct {
	Description *string `json:"description,omitempty" validate:"omitempty,no_control_chars" example:"Dinner order"`
	Category    *string `json:"category,omitempty" validate:"omitempty,no_control_chars" example:"Food & Dining"`
	Bucket      *string `json:"bucket,omitempty" validate:"omitempty,no_control_chars" example:"Wants"`
	Muted       *bool   `json:"muted,omitempty" example:"true"`
	MuteReason  *string `json:"mute_reason,omitempty" validate:"omitempty,no_control_chars" example:"Duplicate notification"`
}

// TransactionLabelsRequest is the transaction labels mutation payload.
type TransactionLabelsRequest struct {
	Labels []string `json:"labels" validate:"required,min=1,dive,required,no_control_chars"`
}

// MutedMerchantResponse is a muted merchant pattern with the current muted transaction count.
type MutedMerchantResponse struct {
	ID         string    `json:"id" example:"11111111-1111-1111-1111-111111111111"`
	Pattern    string    `json:"pattern" example:"Swiggy"`
	Reason     string    `json:"reason,omitempty" example:"contract check"`
	CreatedAt  time.Time `json:"created_at"`
	MutedCount int       `json:"muted_count"`
}

// MuteMerchantRequest is the merchant mute payload.
type MuteMerchantRequest struct {
	Pattern string `json:"pattern" validate:"required,no_control_chars" example:"Swiggy"`
	Reason  string `json:"reason,omitempty" validate:"no_control_chars" example:"contract check"`
}

// MuteMerchantResponse is the merchant mute response payload.
type MuteMerchantResponse struct {
	Pattern string `json:"pattern" example:"Swiggy"`
}

// MerchantReasonRequest is the muted merchant reason update payload.
type MerchantReasonRequest struct {
	Reason string `json:"reason" validate:"no_control_chars" example:"contract check"`
}

// MerchantReasonResponse is the muted merchant reason response payload.
type MerchantReasonResponse struct {
	Reason string `json:"reason" example:"Internal transfer"`
}

// CategorizeMerchantRequest is the merchant-wide categorization payload.
type CategorizeMerchantRequest struct {
	Merchant string `json:"merchant" validate:"required,no_control_chars" example:"Swiggy"`
	Category string `json:"category" validate:"no_control_chars" example:"Food & Dining"`
	Bucket   string `json:"bucket" validate:"no_control_chars" example:"Wants"`
}

// CategorizeMerchantResponse is the merchant-wide categorization response payload.
type CategorizeMerchantResponse struct {
	Updated int64 `json:"updated"`
}
