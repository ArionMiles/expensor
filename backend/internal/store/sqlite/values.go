package sqlite

import (
	"database/sql"
	"encoding/json"
	"math"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	amountScale       = 10_000
	exchangeRateScale = 1_000_000
	timeLayout        = "2006-01-02T15:04:05.000000Z"
)

func amountToScaled(value float64) (int64, error) {
	return toScaled(value, amountScale, "amount")
}

func exchangeRateToScaled(value float64) (int64, error) {
	return toScaled(value, exchangeRateScale, "exchange rate")
}

func toScaled(value, scale float64, name string) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.B.Op("sqlite.values.to_scaled").KindInvalidInput().Text(name + " must be finite").Build()
	}

	scaled := math.Round(value * scale)
	if math.IsInf(scaled, 0) || scaled < float64(math.MinInt64) || scaled >= float64(math.MaxInt64) {
		return 0, errors.B.Op("sqlite.values.to_scaled").KindInvalidInput().Text(name + " is outside the supported range").Build()
	}
	return int64(scaled), nil
}

func amountFromScaled(value int64) float64 {
	return float64(value) / amountScale
}

func exchangeRateFromScaled(value int64) float64 {
	return float64(value) / exchangeRateScale
}

func timeToText(value time.Time) string {
	return value.UTC().Format(timeLayout)
}

func nullableTimeToText(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return timeToText(value)
}

func timeFromText(value string) (time.Time, error) {
	parsed, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}, errors.B.Op("sqlite.values.parse_time").KindInternal().Text("stored timestamp is invalid").Err(err).Build()
	}
	return parsed.UTC(), nil
}

func nullableTimeFromText(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}
	return timeFromText(value.String)
}

func jsonArrayToText[T any](value []T) (string, error) {
	if value == nil {
		value = []T{}
	}
	return jsonToText(value, "JSON array")
}

func jsonObjectToText[T any](value map[string]T) (string, error) {
	if value == nil {
		value = map[string]T{}
	}
	return jsonToText(value, "JSON object")
}

func jsonToText(value any, name string) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", errors.B.Op("sqlite.values.marshal_json").KindInvalidInput().Text(name + " is invalid").Err(err).Build()
	}
	return string(encoded), nil
}

func jsonArrayFromText[T any](value string, destination *[]T) error {
	var decoded []T
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return errors.B.Op("sqlite.values.parse_json_array").KindInternal().Text("stored JSON array is invalid").Err(err).Build()
	}
	if decoded == nil {
		return errors.B.Op("sqlite.values.parse_json_array").KindInternal().Text("stored JSON array is invalid").Build()
	}
	*destination = decoded
	return nil
}

func jsonObjectFromText[T any](value string) (map[string]T, error) {
	var decoded map[string]T
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, errors.B.Op("sqlite.values.parse_json_object").KindInternal().Text("stored JSON object is invalid").Err(err).Build()
	}
	if decoded == nil {
		return nil, errors.B.Op("sqlite.values.parse_json_object").KindInternal().Text("stored JSON object is invalid").Build()
	}
	return decoded, nil
}
