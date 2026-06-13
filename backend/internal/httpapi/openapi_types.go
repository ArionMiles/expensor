package httpapi

import "time"

// ErrorResponse is the standard JSON error payload for OpenAPI generation.
type ErrorResponse struct {
	Error string `json:"error" example:"internal server error"`
}

// ValidationErrorDetail describes one invalid request field.
type ValidationErrorDetail struct {
	Field    string `json:"field" example:"page_size"`
	Location string `json:"location" example:"query"`
	Message  string `json:"message" example:"must be at most 100"`
}

// ValidationErrorResponse is returned for semantically invalid requests.
type ValidationErrorResponse struct {
	Error   string                  `json:"error" example:"request validation failed"`
	Details []ValidationErrorDetail `json:"details"`
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

// ActiveReaderResponse is the active reader config payload.
type ActiveReaderResponse struct {
	Reader string `json:"reader" example:"gmail"`
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

// ConfigFieldResponse documents reader plugin configuration field metadata.
type ConfigFieldResponse struct {
	Name      string `json:"name" example:"profilePath"`
	Label     string `json:"label" example:"Profile path"`
	Type      string `json:"type" example:"text"`
	Required  bool   `json:"required"`
	Help      string `json:"help,omitempty" example:"Path to the Thunderbird profile"`
	DependsOn string `json:"depends_on,omitempty" example:"profilePath"`
}

// ReaderInfoResponse documents reader plugin metadata.
type ReaderInfoResponse struct {
	Name                      string                `json:"name" example:"thunderbird"`
	Description               string                `json:"description" example:"Read transaction emails from Thunderbird"`
	AuthType                  string                `json:"auth_type" example:"config"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []ConfigFieldResponse `json:"config_schema"`
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

// ReaderDisconnectResponse documents a reader disconnect result.
type ReaderDisconnectResponse struct {
	Status       string   `json:"status" example:"disconnected"`
	FilesRemoved []string `json:"files_removed"`
}

// ReaderTokenRevokeResponse documents an OAuth token revoke result.
type ReaderTokenRevokeResponse struct {
	Status string `json:"status" example:"revoked"`
}

// ReaderConfigRequest documents arbitrary persisted reader configuration JSON.
type ReaderConfigRequest map[string]any

// ReaderConfigResponse documents arbitrary persisted reader configuration JSON.
type ReaderConfigResponse map[string]any

// ReaderConfigSaveResponse documents a reader config save result.
type ReaderConfigSaveResponse struct {
	Status string `json:"status" example:"saved"`
}

// ReaderStatusResponse documents a reader readiness status payload.
type ReaderStatusResponse struct {
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

// WriterInfoResponse documents writer plugin metadata.
type WriterInfoResponse struct {
	Name        string `json:"name" example:"postgres"`
	Description string `json:"description" example:"Store transactions in PostgreSQL"`
}

// ReaderGuideResponse documents a reader setup guide payload.
type ReaderGuideResponse struct {
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

// ReaderCheckpointResponse is the reader checkpoint payload.
type ReaderCheckpointResponse struct {
	LastScanAt *string `json:"last_scan_at" example:"2026-04-14T09:00:00Z" extensions:"x-nullable"`
}

// SyncStatusResponse is the community sync status payload.
type SyncStatusResponse struct {
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty" extensions:"x-nullable"`
	Error          *string    `json:"error,omitempty" extensions:"x-nullable"`
	EntriesUpdated int64      `json:"entries_updated"`
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
