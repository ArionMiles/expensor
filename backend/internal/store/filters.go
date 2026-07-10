package store

import "time"

// ListFilter controls pagination and filtering for ListTransactions.
type ListFilter struct {
	Page               int    // 1-based
	PageSize           int    // max rows per page
	Category           string // partial match, empty = all
	CategoryMissing    bool   // true = category is NULL or empty
	ExcludeCategories  []string
	Currency           string // partial match, empty = all
	Source             string // partial match, empty = all
	ExcludeSources     []string
	SourceType         string
	ExcludeSourceTypes []string
	Bank               string
	ExcludeBanks       []string
	Bucket             string // partial match, empty = all
	BucketMissing      bool   // true = bucket is NULL or empty
	ExcludeBuckets     []string
	Label              string // filter by label, empty = all
	LabelMissing       bool   // true = no labels assigned
	ExcludeLabels      []string
	Merchant           string // partial match on merchant_info, empty = all
	ShowMuted          bool   // when true, muted transactions are included; default hides them
	MutedOnly          bool   // when true, only muted=true (for click-through from Muted page)
	IndividualOnly     bool   // when true, only muted=true AND muted_by_merchant=false (per-tx mutes)
	Weekday            *int   // nil = all weekdays; uses configured timezone and backend weekday convention
	HourFrom           *int   // nil = all hours; non-nil filters transaction hour >= *HourFrom
	HourTo             *int   // nil = all hours; non-nil filters transaction hour <= *HourTo
	Timezone           string // IANA timezone for hour extraction; defaults to UTC when empty
	From               *time.Time
	To                 *time.Time
	SortBy             string // "timestamp" (only supported value for now); default = "timestamp"
	SortDir            string // "asc" | "desc"; default = "desc"
}
