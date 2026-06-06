package api

import (
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/form/v4"
)

func newQueryDecoder() *form.Decoder {
	decoder := form.NewDecoder()
	decoder.SetMode(form.ModeExplicit)
	return decoder
}

func (h *Handlers) decodeQuery(w http.ResponseWriter, r *http.Request, target any) bool {
	err := h.queryDecoder.Decode(target, r.URL.Query())
	if err == nil {
		return true
	}

	var decodeErrors form.DecodeErrors
	if !errors.As(err, &decodeErrors) {
		h.logger.Error("decode query", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decode query")
		return false
	}

	fieldTypes := queryFieldTypes(target)
	fields := make([]string, 0, len(decodeErrors))
	for field := range decodeErrors {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	violations := make([]ValidationErrorDetail, 0, len(fields))
	for _, field := range fields {
		violations = append(violations, ValidationErrorDetail{
			Field:    field,
			Location: "query",
			Message:  queryConversionMessage(fieldTypes[field]),
		})
	}
	writeValidationErrors(w, violations)
	return false
}

func queryFieldTypes(target any) map[string]reflect.Type {
	targetType := reflect.TypeOf(target)
	for targetType != nil && targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == nil || targetType.Kind() != reflect.Struct {
		return nil
	}

	fieldTypes := make(map[string]reflect.Type, targetType.NumField())
	for index := range targetType.NumField() {
		field := targetType.Field(index)
		name := strings.Split(field.Tag.Get("form"), ",")[0]
		if name == "" || name == "-" {
			name = field.Name
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		fieldTypes[name] = fieldType
	}
	return fieldTypes
}

func queryConversionMessage(fieldType reflect.Type) string {
	if fieldType == reflect.TypeOf(time.Time{}) {
		return "must be an RFC3339 timestamp"
	}
	if fieldType != nil {
		switch fieldType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "must be an integer"
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "must be a non-negative integer"
		case reflect.Float32, reflect.Float64:
			return "must be a number"
		case reflect.Bool:
			return "must be a boolean"
		default:
		}
	}
	return "has an invalid format"
}
