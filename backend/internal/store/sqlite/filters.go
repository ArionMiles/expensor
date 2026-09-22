package sqlite

import (
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

var caseFolder = cases.Fold()

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

func foldedContains(value, fragment string) bool {
	return strings.Contains(caseFolder.String(value), caseFolder.String(fragment))
}

func containsExact(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func matchesListFilter(transaction store.Transaction, filter store.ListFilter) bool {
	if !matchesMutedFilter(transaction, filter) {
		return false
	}
	if !matchesMissingFilter(transaction, filter) {
		return false
	}
	if !matchesValueFilter(transaction, filter) {
		return false
	}
	if !matchesLabelFilter(transaction.Labels, filter.Label) {
		return false
	}
	if !matchesExclusionFilter(transaction, filter) {
		return false
	}
	if !matchesDateFilter(transaction.Timestamp, filter) {
		return false
	}
	return matchesLocalTimeFilter(transaction.Timestamp, filter)
}

func matchesMutedFilter(transaction store.Transaction, filter store.ListFilter) bool {
	switch {
	case filter.IndividualOnly:
		return transaction.Muted && !transaction.MutedByMerchant
	case filter.MutedOnly:
		return transaction.Muted
	case !filter.ShowMuted && transaction.Muted:
		return false
	default:
		return true
	}
}

func matchesMissingFilter(transaction store.Transaction, filter store.ListFilter) bool {
	if filter.CategoryMissing && transaction.Category != "" ||
		filter.BucketMissing && transaction.Bucket != "" ||
		filter.LabelMissing && len(transaction.Labels) != 0 {
		return false
	}
	return true
}

func matchesValueFilter(transaction store.Transaction, filter store.ListFilter) bool {
	if filter.Category != "" && !foldedContains(transaction.Category, filter.Category) ||
		filter.Bucket != "" && !foldedContains(transaction.Bucket, filter.Bucket) ||
		filter.Currency != "" && !foldedContains(transaction.Currency, filter.Currency) ||
		filter.Source != "" && !foldedContains(transaction.Source.Display(), filter.Source) ||
		filter.SourceType != "" && !foldedContains(transaction.Source.Type, filter.SourceType) ||
		filter.Bank != "" && !foldedContains(transaction.Source.Bank, filter.Bank) ||
		filter.Merchant != "" && !foldedContains(transaction.MerchantInfo, filter.Merchant) {
		return false
	}
	return true
}

func matchesLabelFilter(labels []string, labelFilter string) bool {
	if labelFilter == "" {
		return true
	}
	for _, label := range labels {
		if foldedContains(label, labelFilter) {
			return true
		}
	}
	return false
}

func matchesExclusionFilter(transaction store.Transaction, filter store.ListFilter) bool {
	if transaction.Category == "" && len(filter.ExcludeCategories) > 0 ||
		transaction.Bucket == "" && len(filter.ExcludeBuckets) > 0 ||
		transaction.Source.Display() == "" && len(filter.ExcludeSources) > 0 || transaction.Source.Type == "" && len(filter.ExcludeSourceTypes) > 0 ||
		transaction.Source.Bank == "" && len(filter.ExcludeBanks) > 0 {
		return false
	}
	if containsExact(filter.ExcludeCategories, transaction.Category) || containsExact(filter.ExcludeBuckets, transaction.Bucket) ||
		containsExact(filter.ExcludeSources, transaction.Source.Display()) || containsExact(filter.ExcludeSourceTypes, transaction.Source.Type) ||
		containsExact(filter.ExcludeBanks, transaction.Source.Bank) {
		return false
	}
	if len(filter.ExcludeLabels) > 0 {
		included := false
		for _, label := range transaction.Labels {
			if !containsExact(filter.ExcludeLabels, label) {
				included = true
			}
		}
		if !included {
			return false
		}
	}
	return true
}

func matchesDateFilter(timestamp time.Time, filter store.ListFilter) bool {
	if filter.From != nil && timestamp.Before(*filter.From) ||
		filter.To != nil && timestamp.After(*filter.To) {
		return false
	}
	return true
}

func matchesLocalTimeFilter(timestamp time.Time, filter store.ListFilter) bool {
	if filter.Weekday == nil && filter.HourFrom == nil && filter.HourTo == nil {
		return true
	}
	location := time.UTC
	if filter.Timezone != "" {
		if loaded, err := time.LoadLocation(filter.Timezone); err == nil {
			location = loaded
		}
	}
	local := timestamp.In(location)
	if filter.Weekday != nil && int(local.Weekday()) != *filter.Weekday ||
		filter.HourFrom != nil && local.Hour() < *filter.HourFrom ||
		filter.HourTo != nil && local.Hour() > *filter.HourTo {
		return false
	}
	return true
}

func matchesSearch(transaction store.Transaction, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	haystack := caseFolder.String(transaction.MerchantInfo + " " + transaction.Description)
	foldedQuery := caseFolder.String(query)
	if strings.Contains(haystack, foldedQuery) {
		return true
	}
	groupMatches := true
	groupHasTerm := false
	for _, term := range strings.Fields(foldedQuery) {
		if term == "or" {
			if groupHasTerm && groupMatches {
				return true
			}
			groupMatches = true
			groupHasTerm = false
			continue
		}
		groupHasTerm = true
		if !strings.Contains(haystack, term) {
			groupMatches = false
		}
	}
	return groupHasTerm && groupMatches
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?" + strconv.Itoa(i+1)
	}
	return strings.Join(values, ",")
}
