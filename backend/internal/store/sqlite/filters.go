package sqlite

import (
	"math"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

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
	conditions := []string{"t.tenant_id=?"}
	args := []any{tenant.ID}
	contains := func(column, value string) {
		if value == "" {
			return
		}
		conditions = append(conditions, "instr(expensor_casefold(coalesce("+column+",'')),expensor_casefold(?))>0")
		args = append(args, value)
	}
	exclude := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		conditions = append(conditions, "coalesce("+column+",'')<>''", column+" NOT IN ("+placeholders(len(values))+")")
		for _, value := range values {
			args = append(args, value)
		}
	}

	switch {
	case filter.IndividualOnly:
		conditions = append(conditions, "t.muted=1", "t.muted_by_merchant=0")
	case filter.MutedOnly:
		conditions = append(conditions, "t.muted=1")
	case !filter.ShowMuted:
		conditions = append(conditions, "t.muted=0")
	}
	contains("t.category", filter.Category)
	contains("t.bucket", filter.Bucket)
	contains("t.currency", filter.Currency)
	contains("t.source", filter.Source)
	contains("t.source_type", filter.SourceType)
	contains("t.bank", filter.Bank)
	contains("t.merchant_info", filter.Merchant)
	if filter.Label != "" {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id
			AND instr(expensor_casefold(tl.label),expensor_casefold(?))>0)`)
		args = append(args, filter.Label)
	}
	if filter.CategoryMissing {
		conditions = append(conditions, "coalesce(t.category,'')='' ")
	}
	if filter.BucketMissing {
		conditions = append(conditions, "coalesce(t.bucket,'')='' ")
	}
	if filter.LabelMissing {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id)")
	}
	exclude("t.category", filter.ExcludeCategories)
	exclude("t.bucket", filter.ExcludeBuckets)
	exclude("t.source", filter.ExcludeSources)
	exclude("t.source_type", filter.ExcludeSourceTypes)
	exclude("t.bank", filter.ExcludeBanks)
	if len(filter.ExcludeLabels) > 0 {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM transaction_labels tl WHERE tl.transaction_id=t.id
			AND tl.label NOT IN (`+placeholders(len(filter.ExcludeLabels))+`))`)
		for _, value := range filter.ExcludeLabels {
			args = append(args, value)
		}
	}
	if filter.From != nil {
		conditions = append(conditions, "t.timestamp>=?")
		args = append(args, timeToText(*filter.From))
	}
	if filter.To != nil {
		conditions = append(conditions, "t.timestamp<=?")
		args = append(args, timeToText(*filter.To))
	}
	timezone := filter.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	if filter.Weekday != nil {
		conditions = append(conditions, "expensor_local_weekday(t.timestamp,?)=?")
		args = append(args, timezone, *filter.Weekday)
	}
	if filter.HourFrom != nil {
		conditions = append(conditions, "expensor_local_hour(t.timestamp,?)>=?")
		args = append(args, timezone, *filter.HourFrom)
	}
	if filter.HourTo != nil {
		conditions = append(conditions, "expensor_local_hour(t.timestamp,?)<=?")
		args = append(args, timezone, *filter.HourTo)
	}
	if condition, searchArgs := buildTransactionSearch(strings.TrimSpace(search)); condition != "" {
		conditions = append(conditions, condition)
		args = append(args, searchArgs...)
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
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
