package api

import "time"

// DocErrorResponse is the standard JSON error payload for OpenAPI generation.
type DocErrorResponse struct {
	Error string `json:"error" example:"database not connected"`
}

// DocHealthResponse is the health check payload.
type DocHealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// DocVersionResponse is the version payload.
type DocVersionResponse struct {
	Version string `json:"version" example:"dev"`
}

// DocStatsResponse documents the stats payload embedded in status responses.
type DocStatsResponse struct {
	TotalCount         int                `json:"total_count"`
	TotalBase          float64            `json:"total_base"`
	BaseCurrency       string             `json:"base_currency" example:"INR"`
	TotalByCategory    map[string]float64 `json:"total_by_category"`
	TotalCategoryCount map[string]int     `json:"total_category_count"`
}

// DocStatusResponse documents the combined daemon and stats status payload.
type DocStatusResponse struct {
	Daemon DaemonStatus      `json:"daemon"`
	Stats  *DocStatsResponse `json:"stats,omitempty"`
}

// DocDaemonReaderRequest is the daemon start/rescan request body.
type DocDaemonReaderRequest struct {
	Reader string `json:"reader" example:"gmail"`
}

// DocStatusOnlyResponse is a simple status message payload.
type DocStatusOnlyResponse struct {
	Status string `json:"status" example:"ok"`
}

// DocActiveReaderResponse is the active reader config payload.
type DocActiveReaderResponse struct {
	Reader string `json:"reader" example:"gmail"`
}

// DocBaseCurrencyRequest is the base currency update payload.
type DocBaseCurrencyRequest struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// DocBaseCurrencyResponse is the base currency payload.
type DocBaseCurrencyResponse struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// DocScanIntervalRequest is the scan interval update payload.
type DocScanIntervalRequest struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// DocScanIntervalResponse is the scan interval payload.
type DocScanIntervalResponse struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// DocLookbackDaysRequest is the lookback days update payload.
type DocLookbackDaysRequest struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// DocLookbackDaysResponse is the lookback days payload.
type DocLookbackDaysResponse struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// DocTimezoneRequest is the timezone update payload.
type DocTimezoneRequest struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// DocTimezoneResponse is the timezone payload.
type DocTimezoneResponse struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// DocTimeFormatRequest is the time format update payload.
type DocTimeFormatRequest struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
}

// DocTimeFormatResponse is the time format payload.
type DocTimeFormatResponse struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
}

// DocSetupStatusResponse is the first-run setup status payload.
type DocSetupStatusResponse struct {
	Required bool     `json:"required" example:"true"`
	Missing  []string `json:"missing" example:"base_currency,timezone,time_format"`
}

// DocReaderCheckpointResponse is the reader checkpoint payload.
type DocReaderCheckpointResponse struct {
	LastScanAt *string `json:"last_scan_at" example:"2026-04-14T09:00:00Z" extensions:"x-nullable"`
}

// DocSyncStatusResponse is the community sync status payload.
type DocSyncStatusResponse struct {
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty" extensions:"x-nullable"`
	Error          *string    `json:"error,omitempty" extensions:"x-nullable"`
	EntriesUpdated int        `json:"entries_updated"`
}

// DocLabelResponse documents a managed label.
type DocLabelResponse struct {
	Name      string    `json:"name" example:"food"`
	Color     string    `json:"color" example:"#f59e0b"`
	CreatedAt time.Time `json:"created_at"`
}

// DocCreateLabelRequest is the label creation payload.
type DocCreateLabelRequest struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// DocUpdateLabelRequest is the label update payload.
type DocUpdateLabelRequest struct {
	Color string `json:"color" example:"#f59e0b"`
}

// DocLabelMutationResponse is the label create/update response payload.
type DocLabelMutationResponse struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// DocApplyLabelRequest is the label-by-merchant apply payload.
type DocApplyLabelRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"swiggy"`
}

// DocAppliedCountResponse is the count payload for apply actions.
type DocAppliedCountResponse struct {
	Applied int64 `json:"applied"`
}

// DocLabelMappingsResponse documents label-to-merchant mappings.
type DocLabelMappingsResponse map[string][]string

// DocCategoryResponse documents a managed category.
type DocCategoryResponse struct {
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
	IsDefault   bool   `json:"is_default"`
}

// DocCreateCategoryRequest is the category creation payload.
type DocCreateCategoryRequest struct {
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
}

// DocBucketResponse documents a managed bucket.
type DocBucketResponse struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
	IsDefault   bool   `json:"is_default"`
}

// DocCreateBucketRequest is the bucket creation payload.
type DocCreateBucketRequest struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
}

// DocNameResponse is a simple named resource payload.
type DocNameResponse struct {
	Name string `json:"name" example:"Food"`
}

// DocBankColorResponse documents an embedded bank color mapping.
type DocBankColorResponse struct {
	Fragment string `json:"fragment" example:"hdfc"`
	Color    string `json:"color" example:"#2563eb"`
	Name     string `json:"name" example:"HDFC Bank"`
}

// DocTransactionResponse documents a transaction payload.
type DocTransactionResponse struct {
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

// DocTransactionsListResponse documents the paginated list payload.
type DocTransactionsListResponse struct {
	Transactions []DocTransactionResponse `json:"transactions"`
	Total        int                      `json:"total"`
	TotalAmount  float64                  `json:"total_amount"`
	BaseCurrency string                   `json:"base_currency" example:"INR"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"page_size"`
}

// DocTransactionsSearchResponse documents the paginated search payload.
type DocTransactionsSearchResponse struct {
	Transactions []DocTransactionResponse `json:"transactions"`
	Total        int                      `json:"total"`
	TotalAmount  float64                  `json:"total_amount"`
	BaseCurrency string                   `json:"base_currency" example:"INR"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"page_size"`
	Query        string                   `json:"query" example:"coffee"`
}

// DocExtractionDiagnosticResponse documents an extraction diagnostic payload.
type DocExtractionDiagnosticResponse struct {
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

// DocExtractionDiagnosticStatusRequest is the diagnostic status update payload.
type DocExtractionDiagnosticStatusRequest struct {
	Status string `json:"status" example:"resolved"`
}

// DocFacetsResponse documents the distinct transaction filter values.
type DocFacetsResponse struct {
	Sources    []string `json:"sources"`
	Categories []string `json:"categories"`
	Currencies []string `json:"currencies"`
	Merchants  []string `json:"merchants"`
	Labels     []string `json:"labels"`
	Buckets    []string `json:"buckets"`
}

// DocTransactionUpdateRequest is the transaction patch payload.
type DocTransactionUpdateRequest struct {
	Description *string `json:"description,omitempty" example:"Dinner order"`
	Category    *string `json:"category,omitempty" example:"Food"`
	Bucket      *string `json:"bucket,omitempty" example:"Needs"`
}

// DocTransactionLabelsRequest is the transaction labels mutation payload.
type DocTransactionLabelsRequest struct {
	Labels []string `json:"labels"`
}

// DocMuteTransactionRequest is the transaction mute payload.
type DocMuteTransactionRequest struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// DocMuteTransactionResponse is the transaction mute response payload.
type DocMuteTransactionResponse struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// DocUpdateMuteReasonRequest is the mute reason update payload.
type DocUpdateMuteReasonRequest struct {
	Reason string `json:"reason" example:"Internal transfer"`
}

// DocMuteReasonResponse is the mute reason response payload.
type DocMuteReasonResponse struct {
	Reason string `json:"reason" example:"Internal transfer"`
}
