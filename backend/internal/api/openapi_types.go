package api

import "time"

// ErrorResponse is the standard JSON error payload for OpenAPI generation.
type ErrorResponse struct {
	Error string `json:"error" example:"database not connected"`
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
	Reader string `json:"reader" example:"gmail"`
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
	Type  string `json:"type" example:"Email"`
	Label string `json:"label" example:"Contract"`
	Bank  string `json:"bank" example:"Contract Bank"`
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
	Name            string             `json:"name" example:"Contract Rule"`
	SenderEmails    []string           `json:"sender_emails"`
	SubjectContains string             `json:"subject_contains" example:"Contract transaction"`
	AmountRegex     string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" example:"(INR)"`
	Source          RuleSourceResponse `json:"source"`
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
	Name            string             `json:"name" example:"Contract Rule"`
	SenderEmails    []string           `json:"sender_emails"`
	SubjectContains string             `json:"subject_contains" example:"Contract transaction"`
	AmountRegex     string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" example:"(INR)"`
	Source          RuleSourceResponse `json:"source"`
}

// RuleDocumentResponse documents a versioned rules import/export document.
type RuleDocumentResponse struct {
	Version int                         `json:"version" example:"2"`
	Presets RulePresetsResponse         `json:"presets"`
	Rules   []RuleDocumentEntryResponse `json:"rules"`
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
	URL string `json:"url" example:"http://localhost:8080/api/auth/callback?state=state&code=code" binding:"required"`
}

// AuthExchangeResponse documents a successful manual OAuth exchange.
type AuthExchangeResponse struct {
	Status string `json:"status" example:"authorized"`
}

// AuthStatusResponse documents a reader auth status payload.
type AuthStatusResponse struct {
	Authenticated bool       `json:"authenticated"`
	AuthType      string     `json:"auth_type,omitempty" example:"config"`
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

// HeatmapDataResponse documents heatmap stats data.
type HeatmapDataResponse struct {
	ByWeekdayHour []WeekdayHourBucketResponse `json:"by_weekday_hour"`
	ByDayOfMonth  []DayOfMonthBucketResponse  `json:"by_day_of_month"`
}

// DailyBucketResponse documents spend totals for a calendar date.
type DailyBucketResponse struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount" example:"249.50"`
	Count  int       `json:"count" example:"3"`
}

// AnnualHeatmapResponse documents annual heatmap stats data.
type AnnualHeatmapResponse struct {
	Year    int                   `json:"year" example:"2026"`
	Buckets []DailyBucketResponse `json:"buckets"`
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

// BaseCurrencyRequest is the base currency update payload.
type BaseCurrencyRequest struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// BaseCurrencyResponse is the base currency payload.
type BaseCurrencyResponse struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// ScanIntervalRequest is the scan interval update payload.
type ScanIntervalRequest struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// ScanIntervalResponse is the scan interval payload.
type ScanIntervalResponse struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// LookbackDaysRequest is the lookback days update payload.
type LookbackDaysRequest struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// LookbackDaysResponse is the lookback days payload.
type LookbackDaysResponse struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// TimezoneRequest is the timezone update payload.
type TimezoneRequest struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// TimezoneResponse is the timezone payload.
type TimezoneResponse struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// TimeFormatRequest is the time format update payload.
type TimeFormatRequest struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
}

// TimeFormatResponse is the time format payload.
type TimeFormatResponse struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
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
	EntriesUpdated int        `json:"entries_updated"`
}

// LabelResponse documents a managed label.
type LabelResponse struct {
	Name      string    `json:"name" example:"food"`
	Color     string    `json:"color" example:"#f59e0b"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateLabelRequest is the label creation payload.
type CreateLabelRequest struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// UpdateLabelRequest is the label update payload.
type UpdateLabelRequest struct {
	Color string `json:"color" example:"#f59e0b"`
}

// LabelMutationResponse is the label create/update response payload.
type LabelMutationResponse struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// ApplyLabelRequest is the label-by-merchant apply payload.
type ApplyLabelRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"swiggy"`
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

// TaxonomyMerchantRequest is the merchant-pattern payload for taxonomy apply/remove actions.
type TaxonomyMerchantRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"Swiggy"`
}

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
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
}

// BucketResponse documents a managed bucket.
type BucketResponse struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
	IsDefault   bool   `json:"is_default"`
}

// CreateBucketRequest is the bucket creation payload.
type CreateBucketRequest struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
}

// NameResponse is a simple named resource payload.
type NameResponse struct {
	Name string `json:"name" example:"Food"`
}

// BankColorResponse documents an embedded bank color mapping.
type BankColorResponse struct {
	Fragment string `json:"fragment" example:"hdfc"`
	Color    string `json:"color" example:"#2563eb"`
	Name     string `json:"name" example:"HDFC Bank"`
}

// TransactionResponse documents a transaction payload.
type TransactionResponse struct {
	ID               string    `json:"id" example:"tx_123"`
	MessageID        string    `json:"message_id" example:"gmail-message-id"`
	Amount           float64   `json:"amount" example:"249.50"`
	Currency         string    `json:"currency" example:"INR"`
	OriginalAmount   *float64  `json:"original_amount,omitempty"`
	OriginalCurrency *string   `json:"original_currency,omitempty"`
	ExchangeRate     *float64  `json:"exchange_rate,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	MerchantInfo     string    `json:"merchant_info" example:"Swiggy"`
	Category         string    `json:"category" example:"Food"`
	Bucket           string    `json:"bucket" example:"Needs"`
	Source           string    `json:"source" example:"gmail"`
	Description      string    `json:"description" example:"Dinner order"`
	Labels           []string  `json:"labels"`
	Muted            bool      `json:"muted"`
	MutedByMerchant  bool      `json:"muted_by_merchant"`
	MuteReason       string    `json:"mute_reason,omitempty" example:"Internal transfer"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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

// TransactionsSearchResponse documents the paginated search payload.
type TransactionsSearchResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Total        int                   `json:"total"`
	TotalAmount  float64               `json:"total_amount"`
	BaseCurrency string                `json:"base_currency" example:"INR"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"page_size"`
	Query        string                `json:"query" example:"coffee"`
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
	Status string `json:"status" example:"resolved"`
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
	Description *string `json:"description,omitempty" example:"Dinner order"`
	Category    *string `json:"category,omitempty" example:"Food"`
	Bucket      *string `json:"bucket,omitempty" example:"Needs"`
}

// TransactionLabelsRequest is the transaction labels mutation payload.
type TransactionLabelsRequest struct {
	Labels []string `json:"labels"`
}

// MuteTransactionRequest is the transaction mute payload.
type MuteTransactionRequest struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// MuteTransactionResponse is the transaction mute response payload.
type MuteTransactionResponse struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// UpdateMuteReasonRequest is the mute reason update payload.
type UpdateMuteReasonRequest struct {
	Reason string `json:"reason" example:"Internal transfer"`
}

// MuteReasonResponse is the mute reason response payload.
type MuteReasonResponse struct {
	Reason string `json:"reason" example:"Internal transfer"`
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
	Pattern string `json:"pattern" example:"Swiggy" binding:"required"`
	Reason  string `json:"reason,omitempty" example:"contract check"`
}

// MuteMerchantResponse is the merchant mute response payload.
type MuteMerchantResponse struct {
	Pattern string `json:"pattern" example:"Swiggy"`
}

// MerchantReasonRequest is the muted merchant reason update payload.
type MerchantReasonRequest struct {
	Reason string `json:"reason" example:"contract check"`
}

// CategorizeMerchantRequest is the merchant-wide categorization payload.
type CategorizeMerchantRequest struct {
	Merchant string `json:"merchant" example:"Swiggy" binding:"required"`
	Category string `json:"category" example:"Food & Dining"`
	Bucket   string `json:"bucket" example:"Wants"`
}

// CategorizeMerchantResponse is the merchant-wide categorization response payload.
type CategorizeMerchantResponse struct {
	Updated int `json:"updated"`
}
