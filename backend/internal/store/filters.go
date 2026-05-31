package store

import (
	"fmt"
	"strings"
	"time"
)

// ListFilter controls pagination and filtering for ListTransactions.
type ListFilter struct {
	Page               int    // 1-based
	PageSize           int    // max rows per page
	Category           string // partial match (ILIKE), empty = all
	CategoryMissing    bool   // true = category is NULL or empty
	ExcludeCategories  []string
	Currency           string // partial match (ILIKE), empty = all
	Source             string // partial match (ILIKE), empty = all
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
	Merchant           string // partial match (ILIKE) on merchant_info, empty = all
	ShowMuted          bool   // when true, muted transactions are included; default hides them
	MutedOnly          bool   // when true, only muted=true (for click-through from Muted page)
	IndividualOnly     bool   // when true, only muted=true AND muted_by_merchant=false (per-tx mutes)
	Weekday            *int   // nil = all weekdays; uses configured timezone and PostgreSQL DOW convention
	HourFrom           *int   // nil = all hours; non-nil filters EXTRACT(HOUR FROM timestamp) >= *HourFrom
	HourTo             *int   // nil = all hours; non-nil filters EXTRACT(HOUR FROM timestamp) <= *HourTo
	Timezone           string // IANA timezone for hour extraction; defaults to UTC when empty
	From               *time.Time
	To                 *time.Time
	SortBy             string // "timestamp" (only supported value for now); default = "timestamp"
	SortDir            string // "asc" | "desc"; default = "desc"
}

func joinLabel(label string) string {
	if label == "" {
		return ""
	}
	return " JOIN transaction_labels tl ON tl.transaction_id = t.id"
}

// buildListWhere builds the WHERE clause and argument list for ListTransactions / SearchTransactions.
// args is grown in-place; the first placeholder index is len(existingArgs)+1.
func buildListWhere(f ListFilter) (string, []any) {
	var conds []string
	var args []any

	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	switch {
	case f.IndividualOnly:
		conds = append(conds, "t.muted = true AND t.muted_by_merchant = false")
	case f.MutedOnly:
		conds = append(conds, "t.muted = true")
	case !f.ShowMuted:
		conds = append(conds, "t.muted = false")
	}
	if f.Label != "" {
		conds = append(conds, fmt.Sprintf("tl.label ILIKE %s", next("%"+f.Label+"%")))
	}
	if f.Merchant != "" {
		conds = append(conds, fmt.Sprintf("t.merchant_info ILIKE %s", next("%"+f.Merchant+"%")))
	}
	conds = appendTaxonomyListWhere(conds, f, next)
	if f.Currency != "" {
		conds = append(conds, fmt.Sprintf("t.currency ILIKE %s", next("%"+f.Currency+"%")))
	}
	if f.Source != "" {
		conds = append(conds, fmt.Sprintf("t.source ILIKE %s", next("%"+f.Source+"%")))
	}
	if len(f.ExcludeSources) > 0 {
		conds = append(conds, "COALESCE(t.source, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.source = ANY(%s))", next(f.ExcludeSources)))
	}
	if f.SourceType != "" {
		conds = append(conds, fmt.Sprintf("t.source_type ILIKE %s", next("%"+f.SourceType+"%")))
	}
	if len(f.ExcludeSourceTypes) > 0 {
		conds = append(conds, "COALESCE(t.source_type, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.source_type = ANY(%s))", next(f.ExcludeSourceTypes)))
	}
	if f.Bank != "" {
		conds = append(conds, fmt.Sprintf("t.bank ILIKE %s", next("%"+f.Bank+"%")))
	}
	if len(f.ExcludeBanks) > 0 {
		conds = append(conds, "COALESCE(t.bank, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.bank = ANY(%s))", next(f.ExcludeBanks)))
	}
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp >= %s", next(*f.From)))
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp <= %s", next(*f.To)))
	}
	tz := f.Timezone
	if tz == "" {
		tz = "UTC"
	}
	localTimestampExpr := func() string {
		return fmt.Sprintf("t.timestamp AT TIME ZONE %s", next(tz))
	}
	if f.Weekday != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(DOW FROM %s)::int = %s",
			localTimestampExpr(), next(*f.Weekday)))
	}
	if f.HourFrom != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(HOUR FROM %s)::int >= %s",
			localTimestampExpr(), next(*f.HourFrom)))
	}
	if f.HourTo != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(HOUR FROM %s)::int <= %s",
			localTimestampExpr(), next(*f.HourTo)))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func appendTaxonomyListWhere(conds []string, f ListFilter, next func(any) string) []string {
	if f.Category != "" {
		conds = append(conds, fmt.Sprintf("t.category ILIKE %s", next("%"+f.Category+"%")))
	}
	if f.CategoryMissing {
		conds = append(conds, "COALESCE(t.category, '') = ''")
	}
	if len(f.ExcludeCategories) > 0 {
		conds = append(conds, "COALESCE(t.category, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.category = ANY(%s))", next(f.ExcludeCategories)))
	}
	if f.Bucket != "" {
		conds = append(conds, fmt.Sprintf("t.bucket ILIKE %s", next("%"+f.Bucket+"%")))
	}
	if f.BucketMissing {
		conds = append(conds, "COALESCE(t.bucket, '') = ''")
	}
	if len(f.ExcludeBuckets) > 0 {
		conds = append(conds, "COALESCE(t.bucket, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.bucket = ANY(%s))", next(f.ExcludeBuckets)))
	}
	if len(f.ExcludeLabels) > 0 {
		conds = append(conds, fmt.Sprintf(
			`EXISTS (
				SELECT 1
				FROM transaction_labels tl_include
				WHERE tl_include.transaction_id = t.id
				  AND NOT (tl_include.label = ANY(%s))
			)`,
			next(f.ExcludeLabels),
		))
	}
	if f.LabelMissing {
		conds = append(conds, `NOT EXISTS (
			SELECT 1
			FROM transaction_labels tl_missing
			WHERE tl_missing.transaction_id = t.id
		)`)
	}
	return conds
}

// buildSearchCondition appends raw search text and returns a safe hybrid search condition.
func buildSearchCondition(query string, args *[]any) string {
	*args = append(*args, query)
	tsArg := len(*args)
	*args = append(*args, escapeLikePattern(query))
	likeArg := len(*args)
	return fmt.Sprintf(
		`(
			(to_tsvector('english', t.merchant_info) || to_tsvector('english', COALESCE(t.description,''))) @@ websearch_to_tsquery('english', $%d)
			OR t.merchant_info ILIKE $%d ESCAPE '\'
			OR COALESCE(t.description, '') ILIKE $%d ESCAPE '\'
		)`,
		tsArg,
		likeArg,
		likeArg,
	)
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

// combineWhere merges a bare condition with an existing WHERE clause.
func combineWhere(cond, existing string) string {
	if existing == "" {
		return " WHERE " + cond
	}
	// existing already starts with " WHERE "
	return existing + " AND " + cond
}
