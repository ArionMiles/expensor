package sqlite

import (
	"math"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func ftsQuery(value string) string {
	fields := strings.Fields(value)
	quoted := make([]string, 0, len(fields))
	for _, field := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(field, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " AND ")
}

func normalizeTransactionListFilter(filter store.ListFilter) store.ListFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	return filter
}

func transactionPageOffset(filter store.ListFilter) (int, error) {
	if filter.Page-1 > math.MaxInt/filter.PageSize {
		return 0, errors.B.Op("store.transactions.list").KindInvalidInput().Text("pagination offset overflow").Build()
	}
	return (filter.Page - 1) * filter.PageSize, nil
}

func buildTransactionWhere(tenant store.Tenant, search string, filter store.ListFilter) (string, []any) {
	where := transactionWhere{conditions: []string{"t.tenant_id=?"}, args: []any{tenant.ID}}
	appendTransactionMuteWhere(&where, filter)
	where.contains("t.category", filter.Category)
	where.contains("t.bucket", filter.Bucket)
	where.contains("t.currency", filter.Currency)
	where.contains("t.source", filter.Source)
	where.contains("t.source_type", filter.SourceType)
	where.contains("t.bank", filter.Bank)
	where.contains("t.merchant_info", filter.Merchant)
	appendTransactionTaxonomyWhere(&where, filter)
	appendTransactionTimeWhere(&where, filter)
	if condition, searchArgs := buildTransactionSearch(strings.TrimSpace(search)); condition != "" {
		where.conditions = append(where.conditions, condition)
		where.args = append(where.args, searchArgs...)
	}
	return " WHERE " + strings.Join(where.conditions, " AND "), where.args
}

type transactionWhere struct {
	conditions []string
	args       []any
}

func (w *transactionWhere) contains(column, value string) {
	if value == "" {
		return
	}
	w.conditions = append(w.conditions, "instr(expensor_casefold(coalesce("+column+",'')),expensor_casefold(?))>0")
	w.args = append(w.args, value)
}

func (w *transactionWhere) exclude(column string, values []string) {
	if len(values) == 0 {
		return
	}
	w.conditions = append(w.conditions, "coalesce("+column+",'')<>''", column+" NOT IN ("+placeholders(len(values))+")")
	for _, value := range values {
		w.args = append(w.args, value)
	}
}

func appendTransactionMuteWhere(where *transactionWhere, filter store.ListFilter) {
	switch {
	case filter.IndividualOnly:
		where.conditions = append(where.conditions, "t.muted=1", "t.muted_by_merchant=0")
	case filter.MutedOnly:
		where.conditions = append(where.conditions, "t.muted=1")
	case !filter.ShowMuted:
		where.conditions = append(where.conditions, "t.muted=0")
	}
}

func appendTransactionTaxonomyWhere(where *transactionWhere, filter store.ListFilter) {
	if filter.Label != "" {
		where.conditions = append(where.conditions, `EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id
			AND instr(expensor_casefold(tl.label),expensor_casefold(?))>0)`)
		where.args = append(where.args, filter.Label)
	}
	if filter.CategoryMissing {
		where.conditions = append(where.conditions, "coalesce(t.category,'')='' ")
	}
	if filter.BucketMissing {
		where.conditions = append(where.conditions, "coalesce(t.bucket,'')='' ")
	}
	if filter.LabelMissing {
		where.conditions = append(where.conditions, "NOT EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id)")
	}
	where.exclude("t.category", filter.ExcludeCategories)
	where.exclude("t.bucket", filter.ExcludeBuckets)
	where.exclude("t.source", filter.ExcludeSources)
	where.exclude("t.source_type", filter.ExcludeSourceTypes)
	where.exclude("t.bank", filter.ExcludeBanks)
	if len(filter.ExcludeLabels) > 0 {
		where.conditions = append(where.conditions, `EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id
			AND tl.label NOT IN (`+placeholders(len(filter.ExcludeLabels))+`))`)
		for _, value := range filter.ExcludeLabels {
			where.args = append(where.args, value)
		}
	}
}

func appendTransactionTimeWhere(where *transactionWhere, filter store.ListFilter) {
	if filter.From != nil {
		where.conditions = append(where.conditions, "t.timestamp>=?")
		where.args = append(where.args, timeToText(*filter.From))
	}
	if filter.To != nil {
		where.conditions = append(where.conditions, "t.timestamp<=?")
		where.args = append(where.args, timeToText(*filter.To))
	}
	timezone := filter.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	if filter.Weekday != nil {
		where.conditions = append(where.conditions, "expensor_local_weekday(t.timestamp,?)=?")
		where.args = append(where.args, timezone, *filter.Weekday)
	}
	if filter.HourFrom != nil {
		where.conditions = append(where.conditions, "expensor_local_hour(t.timestamp,?)>=?")
		where.args = append(where.args, timezone, *filter.HourFrom)
	}
	if filter.HourTo != nil {
		where.conditions = append(where.conditions, "expensor_local_hour(t.timestamp,?)<=?")
		where.args = append(where.args, timezone, *filter.HourTo)
	}
}

func buildTransactionSearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	haystack := "expensor_casefold(t.merchant_info||' '||coalesce(t.description,''))"
	conditions := []string{"instr(" + haystack + ",expensor_casefold(?))>0"}
	args := []any{search}
	groups := [][]string{{}}
	for _, term := range strings.Fields(search) {
		if strings.EqualFold(term, "or") {
			if len(groups[len(groups)-1]) > 0 {
				groups = append(groups, []string{})
			}
			continue
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], term)
	}
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		substrings := make([]string, 0, len(group))
		args = append(args, ftsQuery(strings.Join(group, " ")))
		for _, term := range group {
			substrings = append(substrings, "instr("+haystack+",expensor_casefold(?))>0")
			args = append(args, term)
		}
		conditions = append(conditions, `(EXISTS (SELECT 1 FROM transactions_fts
			WHERE transaction_id=t.id AND transactions_fts MATCH ?) OR (`+strings.Join(substrings, " AND ")+`))`)
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}
