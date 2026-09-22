package sqlite

import (
	"database/sql"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestAmountToScaled(t *testing.T) {
	t.Parallel()

	positiveBoundary := math.Nextafter(float64(math.MaxInt64)/amountScale, 0)
	positiveBoundaryScaled := int64(math.Round(positiveBoundary * amountScale))
	negativeOverflow := math.Nextafter(float64(math.MinInt64), math.Inf(-1)) / amountScale
	tests := []struct {
		name    string
		value   float64
		want    int64
		wantErr bool
	}{
		{name: "four decimal places", value: 12.3456, want: 123456},
		{name: "positive half rounds away from zero", value: 0.00005, want: 1},
		{name: "negative half rounds away from zero", value: -0.00005, want: -1},
		{name: "positive boundary", value: positiveBoundary, want: positiveBoundaryScaled},
		{name: "negative boundary", value: float64(math.MinInt64) / amountScale, want: math.MinInt64},
		{name: "positive overflow", value: float64(math.MaxInt64) / amountScale, wantErr: true},
		{name: "negative overflow", value: negativeOverflow, wantErr: true},
		{name: "NaN", value: math.NaN(), wantErr: true},
		{name: "positive infinity", value: math.Inf(1), wantErr: true},
		{name: "negative infinity", value: math.Inf(-1), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := amountToScaled(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("amountToScaled(%v) error = nil, want error", test.value)
				}
				if kind := apperrors.WhatKind(err); kind != apperrors.InvalidInput {
					t.Fatalf("amountToScaled(%v) error kind = %v, want %v", test.value, kind, apperrors.InvalidInput)
				}
				return
			}
			if err != nil {
				t.Fatalf("amountToScaled(%v): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("amountToScaled(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestExchangeRateToScaled(t *testing.T) {
	t.Parallel()

	positiveBoundary := math.Nextafter(float64(math.MaxInt64)/exchangeRateScale, 0)
	positiveBoundaryScaled := int64(math.Round(positiveBoundary * exchangeRateScale))
	negativeOverflow := math.Nextafter(float64(math.MinInt64), math.Inf(-1)) / exchangeRateScale
	tests := []struct {
		name    string
		value   float64
		want    int64
		wantErr bool
	}{
		{name: "six decimal places", value: 1.234567, want: 1234567},
		{name: "positive half rounds away from zero", value: 0.0000005, want: 1},
		{name: "negative half rounds away from zero", value: -0.0000005, want: -1},
		{name: "positive boundary", value: positiveBoundary, want: positiveBoundaryScaled},
		{name: "negative boundary", value: float64(math.MinInt64) / exchangeRateScale, want: math.MinInt64},
		{name: "positive overflow", value: float64(math.MaxInt64) / exchangeRateScale, wantErr: true},
		{name: "negative overflow", value: negativeOverflow, wantErr: true},
		{name: "NaN", value: math.NaN(), wantErr: true},
		{name: "positive infinity", value: math.Inf(1), wantErr: true},
		{name: "negative infinity", value: math.Inf(-1), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchangeRateToScaled(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("exchangeRateToScaled(%v) error = nil, want error", test.value)
				}
				if kind := apperrors.WhatKind(err); kind != apperrors.InvalidInput {
					t.Fatalf("exchangeRateToScaled(%v) error kind = %v, want %v", test.value, kind, apperrors.InvalidInput)
				}
				return
			}
			if err != nil {
				t.Fatalf("exchangeRateToScaled(%v): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("exchangeRateToScaled(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestScaledValueConversions(t *testing.T) {
	t.Parallel()

	if got := amountFromScaled(123456); got != 12.3456 {
		t.Fatalf("amountFromScaled(123456) = %v, want 12.3456", got)
	}
	if got := exchangeRateFromScaled(1234567); got != 1.234567 {
		t.Fatalf("exchangeRateFromScaled(1234567) = %v, want 1.234567", got)
	}
}

func TestScaledAmountTotalExceedsInt64(t *testing.T) {
	t.Parallel()

	var total scaledAmountTotal
	total.add(math.MaxInt64)
	total.add(math.MaxInt64)
	total.add(-7)

	wantScaled := "18446744073709551607"
	if got := total.scaled.String(); got != wantScaled {
		t.Fatalf("scaled total = %s, want %s", got, wantScaled)
	}
	want := float64(math.MaxInt64)/amountScale*2 - 7/amountScale
	if got := total.float64(); got != want {
		t.Fatalf("public total = %v, want %v", got, want)
	}
}

func TestScaledAmountTotalRoundsOnceAtPublicBoundary(t *testing.T) {
	t.Parallel()

	var total scaledAmountTotal
	total.add(9_885_137_215_378_057)
	before := total.scaled.String()

	const want = 988_513_721_537.8057
	if got := total.float64(); got != want {
		t.Fatalf("public total = %.17g, want independently rounded %.17g", got, want)
	}
	if got := total.scaled.String(); got != before {
		t.Fatalf("scaled total changed from %s to %s during conversion", before, got)
	}
}

func TestTimestampTextUsesFixedUTCFormat(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("test offset", 5*60*60+30*60)
	value := time.Date(2026, time.September, 17, 12, 34, 56, 123456789, location)
	const want = "2026-09-17T07:04:56.123456Z"
	if got := timeToText(value); got != want {
		t.Fatalf("timeToText() = %q, want %q", got, want)
	}
	if len(want)-strings.IndexByte(want, '.')-2 != 6 {
		t.Fatalf("timestamp %q does not contain exactly six fractional digits", want)
	}

	parsed, err := timeFromText(want)
	if err != nil {
		t.Fatalf("timeFromText(%q): %v", want, err)
	}
	wantTime := time.Date(2026, time.September, 17, 7, 4, 56, 123456000, time.UTC)
	if !parsed.Equal(wantTime) || parsed.Location() != time.UTC {
		t.Fatalf("timeFromText(%q) = %v in %v, want %v in UTC", want, parsed, parsed.Location(), wantTime)
	}
}

func TestTimestampTextNormalizesDaylightSavingInstants(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load time zone: %v", err)
	}
	tests := []struct {
		name  string
		value time.Time
		want  string
	}{
		{
			name:  "standard time before spring transition",
			value: time.Date(2026, time.March, 8, 1, 30, 0, 0, location),
			want:  "2026-03-08T06:30:00.000000Z",
		},
		{
			name:  "daylight time after spring transition",
			value: time.Date(2026, time.March, 8, 3, 30, 0, 0, location),
			want:  "2026-03-08T07:30:00.000000Z",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := timeToText(test.value); got != test.want {
				t.Fatalf("timeToText(%v) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestNullableTimestampText(t *testing.T) {
	t.Parallel()

	if got := nullableTimeToText(time.Time{}); got != nil {
		t.Fatalf("nullableTimeToText(zero) = %#v, want nil", got)
	}
	value := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if got := nullableTimeToText(value); got != "2026-01-02T03:04:05.000000Z" {
		t.Fatalf("nullableTimeToText(value) = %#v", got)
	}

	parsed, err := nullableTimeFromText(sql.NullString{})
	if err != nil {
		t.Fatalf("nullableTimeFromText(NULL): %v", err)
	}
	if !parsed.IsZero() {
		t.Fatalf("nullableTimeFromText(NULL) = %v, want zero", parsed)
	}
}

func TestTimeFromTextRejectsInvalidFormats(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"2026-09-17T07:04:56Z",
		"2026-09-17T07:04:56.12345Z",
		"2026-09-17T07:04:56.1234567Z",
		"2026-09-17T07:04:56.123456+00:00",
		"not a timestamp",
	} {
		if _, err := timeFromText(value); err == nil {
			t.Fatalf("timeFromText(%q) error = nil, want error", value)
		}
	}
}

func TestJSONArrayText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value []string
		want  string
	}{
		{name: "nil", value: nil, want: "[]"},
		{name: "empty", value: []string{}, want: "[]"},
		{name: "valid", value: []string{"one", "two"}, want: `["one","two"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonArrayToText(test.value)
			if err != nil {
				t.Fatalf("jsonArrayToText(%#v): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("jsonArrayToText(%#v) = %q, want %q", test.value, got, test.want)
			}
			var decoded []string
			if err := jsonArrayFromText(got, &decoded); err != nil {
				t.Fatalf("jsonArrayFromText(%q): %v", got, err)
			}
			if decoded == nil {
				t.Fatalf("jsonArrayFromText(%q) returned a nil slice", got)
			}
			if !reflect.DeepEqual(decoded, test.value) && (len(decoded) != 0 || len(test.value) != 0) {
				t.Fatalf("decoded array = %#v, want %#v", decoded, test.value)
			}
		})
	}

	if _, err := jsonArrayToText([]float64{math.NaN()}); err == nil {
		t.Fatal("jsonArrayToText(NaN) error = nil, want error")
	}
	var nullArray []string
	if err := jsonArrayFromText("null", &nullArray); err == nil {
		t.Fatal("jsonArrayFromText(null) error = nil, want error")
	}
	for _, value := range []string{"[", `{}`} {
		var decoded []string
		if err := jsonArrayFromText(value, &decoded); err == nil {
			t.Fatalf("jsonArrayFromText(%q) error = nil, want error", value)
		}
	}
}

func TestJSONObjectText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value map[string]string
		want  string
	}{
		{name: "nil", value: nil, want: "{}"},
		{name: "empty", value: map[string]string{}, want: "{}"},
		{name: "valid", value: map[string]string{"key": "value"}, want: `{"key":"value"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonObjectToText(test.value)
			if err != nil {
				t.Fatalf("jsonObjectToText(%#v): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("jsonObjectToText(%#v) = %q, want %q", test.value, got, test.want)
			}
			decoded, err := jsonObjectFromText[string](got)
			if err != nil {
				t.Fatalf("jsonObjectFromText(%q): %v", got, err)
			}
			if decoded == nil {
				t.Fatalf("jsonObjectFromText(%q) returned a nil map", got)
			}
			if !reflect.DeepEqual(decoded, test.value) && (len(decoded) != 0 || len(test.value) != 0) {
				t.Fatalf("decoded object = %#v, want %#v", decoded, test.value)
			}
		})
	}

	if _, err := jsonObjectToText(map[string]float64{"invalid": math.Inf(1)}); err == nil {
		t.Fatal("jsonObjectToText(infinity) error = nil, want error")
	}
	if _, err := jsonObjectFromText[string]("null"); err == nil {
		t.Fatal("jsonObjectFromText(null) error = nil, want error")
	}
	for _, value := range []string{"{", `[]`} {
		if _, err := jsonObjectFromText[string](value); err == nil {
			t.Fatalf("jsonObjectFromText(%q) error = nil, want error", value)
		}
	}
}
