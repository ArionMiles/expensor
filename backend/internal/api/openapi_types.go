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

// AppliedCountResponse is the count payload for apply actions.
type AppliedCountResponse struct {
	Applied int64 `json:"applied"`
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
