package store

import (
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// UserRole is an instance-level account role.
type UserRole string

const (
	// UserRoleAdmin can manage instance users.
	UserRoleAdmin UserRole = "admin"
	// UserRoleUser can access their own tenant data.
	UserRoleUser UserRole = "user"
)

// Tenant identifies a tenant boundary for user-owned data. In Phase 1, the tenant ID is the user's ID.
type Tenant struct {
	ID string
}

// User is an Expensor account.
type User struct {
	ID string
	// TenantID is equal to ID in Phase 1.
	TenantID     string
	Email        string
	PasswordHash string
	DisplayName  string
	Role         UserRole
	AvatarKey    string
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateBootstrapAdminInput creates the first admin account.
type CreateBootstrapAdminInput struct {
	Email        string
	DisplayName  string
	PasswordHash string
	AvatarKey    string
}

// CreateUserInput creates an admin-managed account.
type CreateUserInput struct {
	Email        string
	DisplayName  string
	Role         UserRole
	AvatarKey    string
	PasswordHash string
}

// Session is a persisted browser session.
type Session struct {
	ID         string
	UserID     string
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// CreateSessionInput creates a session hash record.
type CreateSessionInput struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// AccessToken is metadata for a programmatic access token.
type AccessToken struct {
	ID         string
	UserID     string
	Name       string
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// CreateAccessTokenInput creates a programmatic access token hash record.
type CreateAccessTokenInput struct {
	UserID    string
	Name      string
	TokenHash string
	ExpiresAt *time.Time
}

// AccountSetupToken is a one-time password setup token.
type AccountSetupToken struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// CreateAccountSetupTokenInput creates a one-time password setup token.
type CreateAccountSetupTokenInput struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// Transaction represents a single expense transaction as returned by the API.
type Transaction struct {
	ID               string     `json:"id"`
	MessageID        string     `json:"message_id"`
	Amount           float64    `json:"amount"`
	Currency         string     `json:"currency"`
	OriginalAmount   *float64   `json:"original_amount,omitempty"`
	OriginalCurrency *string    `json:"original_currency,omitempty"`
	ExchangeRate     *float64   `json:"exchange_rate,omitempty"`
	Timestamp        time.Time  `json:"timestamp"`
	MerchantInfo     string     `json:"merchant_info"`
	Category         string     `json:"category"`
	Bucket           string     `json:"bucket"`
	Source           api.Source `json:"source"`
	Description      string     `json:"description"`
	Labels           []string   `json:"labels"`
	Muted            bool       `json:"muted"`
	MutedByMerchant  bool       `json:"muted_by_merchant"`
	MuteReason       string     `json:"mute_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// MutedMerchant holds a merchant pattern that auto-mutes matching transactions at write time.
type MutedMerchant struct {
	ID        string    `json:"id"`
	Pattern   string    `json:"pattern"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// MutedMerchantWithCount is a MutedMerchant with the count of currently muted transactions.
type MutedMerchantWithCount struct {
	MutedMerchant
	MutedCount int `json:"muted_count"`
}

// Stats holds aggregate statistics about stored transactions.
type Stats struct {
	TotalCount         int                `json:"total_count"`
	TotalBase          float64            `json:"total_base"`
	BaseCurrency       string             `json:"base_currency"`
	TotalByCategory    map[string]float64 `json:"total_by_category"`
	TotalCategoryCount map[string]int     `json:"total_category_count"`
}

// CategoryMonthlyEntry holds spend totals for a category for the current and prior calendar month.
type CategoryMonthlyEntry struct {
	Current float64 `json:"current"`
	Prior   float64 `json:"prior"`
}

// TimeBucket is a single time-period data point used by chart queries.
type TimeBucket struct {
	Period string  `json:"period"` // "2024-01" for monthly, "2024-01-15" for daily
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// ChartData holds all time-series and breakdown data for the dashboard charts.
type ChartData struct {
	MonthlySpend      []TimeBucket                    `json:"monthly_spend"`
	DailySpend        []TimeBucket                    `json:"daily_spend"`
	ByCategory        map[string]float64              `json:"by_category"`
	ByBucket          map[string]float64              `json:"by_bucket"`
	ByLabel           map[string]float64              `json:"by_label"`
	BySource          map[string]float64              `json:"by_source"`
	BySourceType      map[string]float64              `json:"by_source_type"`
	ByBank            map[string]float64              `json:"by_bank"`
	ByCategoryMonthly map[string]CategoryMonthlyEntry `json:"by_category_monthly"`
}

// DashboardSection is one dashboard slice with a label, summary stats, and charts.
type DashboardSection struct {
	Label  string    `json:"label"`
	Stats  Stats     `json:"stats"`
	Charts ChartData `json:"charts"`
}

// DashboardData separates current-month and all-time dashboard data.
type DashboardData struct {
	CurrentMonth DashboardSection `json:"current_month"`
	AllTime      DashboardSection `json:"all_time"`
}

// MonthlyBreakdownSeries is a named 12-month spend series used by the dashboard line chart.
type MonthlyBreakdownSeries struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

// MonthlyBreakdownData is the line-chart payload for labels, categories, or buckets.
type MonthlyBreakdownData struct {
	Labels []string                 `json:"labels"`
	Months []string                 `json:"months"`
	Series []MonthlyBreakdownSeries `json:"series"`
}

// WeekdayHourBucket holds transaction totals for a (weekday, hour) cell.
// Weekday follows PostgreSQL DOW convention: 0=Sunday … 6=Saturday.
type WeekdayHourBucket struct {
	Weekday int     `json:"weekday"` // 0–6 (0=Sunday)
	Hour    int     `json:"hour"`    // 0–23
	Amount  float64 `json:"amount"`
	Count   int     `json:"count"`
}

// DayOfMonthBucket holds transaction totals for a single calendar day (1–31).
type DayOfMonthBucket struct {
	Day    int     `json:"day"` // 1–31
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// HeatmapData contains both heatmap datasets returned by GetSpendingHeatmap.
type HeatmapData struct {
	ByWeekdayHour []WeekdayHourBucket `json:"by_weekday_hour"`
	ByDayOfMonth  []DayOfMonthBucket  `json:"by_day_of_month"`
}

// DailyBucket holds transaction totals for a single calendar date.
type DailyBucket struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
	Count  int       `json:"count"`
}

// Label is a managed label in the taxonomy.
type Label struct {
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// Category is a managed transaction category.
type Category struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// Bucket is a managed spend bucket (needs / wants / investments / income).
type Bucket struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// MCCEntry represents a single MCC code record from content/mcc.json.
type MCCEntry struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Bucket      string `json:"bucket"`
}

// MerchantCategoryEntry represents a single fragment mapping from content/categories.json.
type MerchantCategoryEntry struct {
	Fragment string  `json:"fragment"`
	MCC      *string `json:"mcc,omitempty"`
	Category *string `json:"category,omitempty"`
	Bucket   *string `json:"bucket,omitempty"`
}

// SyncStatus holds the result of the last community content sync.
type SyncStatus struct {
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	Error          *string    `json:"error"`
	EntriesUpdated int64      `json:"entries_updated"`
}

// RuleRow is a rule as stored in the database.
// Source is either "system" (seeded from embedded rules.json) or "user" (created via UI).
// TransactionSource is the human-readable identifier written to transaction.source (e.g. "Credit Card - HDFC").
type RuleRow struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	SenderEmail       string    `json:"sender_email"`
	SenderEmails      []string  `json:"sender_emails"`
	SubjectContains   string    `json:"subject_contains"`
	AmountRegex       string    `json:"amount_regex"`
	MerchantRegex     string    `json:"merchant_regex"`
	CurrencyRegex     string    `json:"currency_regex"`
	TransactionSource string    `json:"transaction_source"`
	SourceType        string    `json:"source_type"`
	SourceLabel       string    `json:"source_label"`
	Bank              string    `json:"bank"`
	Predefined        bool      `json:"predefined"` // true = seeded from embedded rules.json; editable but not deletable
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

const (
	DiagnosticStatusOpen     = "open"
	DiagnosticStatusResolved = "resolved"
	DiagnosticStatusIgnored  = "ignored"
	DiagnosticStatusAll      = "all"
)

// ExtractionDiagnosticRow is a persisted extraction diagnostic.
type ExtractionDiagnosticRow struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	Reader         string     `json:"reader"`
	MessageID      string     `json:"message_id"`
	Source         string     `json:"source"`
	Sender         string     `json:"sender"`
	SenderEmail    string     `json:"sender_email"`
	Subject        string     `json:"subject"`
	EmailBody      string     `json:"email_body"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	Snippet        string     `json:"snippet"`
	RuleID         *string    `json:"rule_id,omitempty"`
	RuleName       string     `json:"rule_name"`
	AmountRegex    string     `json:"amount_regex"`
	MerchantRegex  string     `json:"merchant_regex"`
	CurrencyRegex  string     `json:"currency_regex"`
	FailureReasons []string   `json:"failure_reasons"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// DiagnosticFilter controls filtering for extraction diagnostic listings.
type DiagnosticFilter struct {
	Status string
	Limit  int
}

// TransactionUpdate carries optional fields for updating a transaction.
// Only non-nil fields are written.
type TransactionUpdate struct {
	Description *string
	Category    *string
	Bucket      *string
}

// TransactionListResult captures aggregate metadata for a filtered transaction query.
type TransactionListResult struct {
	Total       int     `json:"total"`
	TotalAmount float64 `json:"total_amount"`
}
