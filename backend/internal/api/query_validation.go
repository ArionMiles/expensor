package api

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

func optionalQueryInt(values url.Values, key string) (*int, *ValidationErrorDetail) {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, queryValidationError(key, "must be an integer")
	}
	return &value, nil
}

func optionalQueryTime(values url.Values, key string) (*time.Time, *ValidationErrorDetail) {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, queryValidationError(key, "must be an RFC3339 timestamp")
	}
	return &value, nil
}

func queryValidationError(field, message string) *ValidationErrorDetail {
	return &ValidationErrorDetail{Field: field, Location: "query", Message: message}
}
