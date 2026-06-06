package api

import (
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/form/v4"
)

//nolint:gochecknoglobals // immutable query metadata is shared across handler instances.
var queryFieldTypesCache sync.Map

func newQueryDecoder() *form.Decoder {
	decoder := form.NewDecoder()
	decoder.SetMode(form.ModeExplicit)
	return decoder
}

func decodeAndValidateQuery[T any](
	h *Handlers,
	w http.ResponseWriter,
	r *http.Request,
) (T, bool) {
	var query T
	err := h.queryDecoder.Decode(&query, r.URL.Query())
	if err != nil {
		writeQueryDecodeError[T](h, w, err)
		return query, false
	}
	if !h.validateRequest(w, "query", query) {
		return query, false
	}
	return query, true
}

func writeQueryDecodeError[T any](h *Handlers, w http.ResponseWriter, err error) {
	var decodeErrors form.DecodeErrors
	if !errors.As(err, &decodeErrors) {
		h.logger.Error("decode query", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decode query")
		return
	}

	fieldTypes := queryFieldTypes[T]()
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
}

func queryFieldTypes[T any]() map[string]reflect.Type {
	targetType := reflect.TypeFor[T]()
	if cached, ok := queryFieldTypesCache.Load(targetType); ok {
		fieldTypes, valid := cached.(map[string]reflect.Type)
		if valid {
			return fieldTypes
		}
	}

	fieldTypes := buildQueryFieldTypes(targetType)
	queryFieldTypesCache.Store(targetType, fieldTypes)
	return fieldTypes
}

func buildQueryFieldTypes(targetType reflect.Type) map[string]reflect.Type {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Struct {
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
