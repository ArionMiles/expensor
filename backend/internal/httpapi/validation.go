package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func newRequestValidator() *validator.Validate {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, tagName := range []string{"json", "form"} {
			name := strings.Split(field.Tag.Get(tagName), ",")[0]
			if name != "" && name != "-" {
				return name
			}
		}
		return field.Name
	})
	mustRegisterValidation(validate, "no_control_chars", hasNoControlChars)
	mustRegisterValidation(validate, "iana_timezone", isIANATimezone)
	mustRegisterValidation(validate, "currency_code", isCurrencyCode)
	mustRegisterValidation(validate, "time_format", isTimeFormat)
	mustRegisterValidation(validate, "regexp", isRegularExpression)
	validate.RegisterStructValidation(validateTransactionPagination, transactionListQuery{})
	validate.RegisterStructValidation(validateHeatmapQuery, heatmapQuery{})
	return validate
}

func validateTransactionPagination(level validator.StructLevel) {
	query, ok := level.Current().Interface().(transactionListQuery)
	if !ok {
		return
	}
	// Pages 0 and 1 both resolve to the first page and have offset 0.
	if query.Page <= 1 {
		return
	}

	pageSize := 20
	if query.PageSize != nil {
		pageSize = *query.PageSize
	}
	if pageSize < 1 {
		return
	}

	if query.Page-1 > math.MaxInt/pageSize {
		level.ReportError(query.Page, "page", "Page", "page_offset", "")
	}
}

func hasNoControlChars(field validator.FieldLevel) bool {
	for _, char := range field.Field().String() {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func isIANATimezone(field validator.FieldLevel) bool {
	_, err := time.LoadLocation(field.Field().String())
	return err == nil
}

func isCurrencyCode(field validator.FieldLevel) bool {
	value := field.Field().String()
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func isTimeFormat(field validator.FieldLevel) bool {
	switch field.Field().String() {
	case "HH:mm", "HH:mm:ss", "h:mm a", "h:mm:ss a":
		return true
	default:
		return false
	}
}

func isRegularExpression(field validator.FieldLevel) bool {
	_, err := regexp.Compile(field.Field().String())
	return err == nil
}

func mustRegisterValidation(validate *validator.Validate, tag string, fn validator.Func) {
	if err := validate.RegisterValidation(tag, fn); err != nil {
		panic(err)
	}
}

func (h *Handlers) validateRequest(w http.ResponseWriter, location string, request any) bool {
	err := h.validate.Struct(request)
	if err == nil {
		return true
	}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		h.logger.Error("validate request", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to validate request")
		return false
	}

	details := make([]ValidationErrorDetail, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		details = append(details, ValidationErrorDetail{
			Field:    validationFieldPath(fieldError),
			Location: location,
			Message:  validationMessage(fieldError),
		})
	}
	writeValidationErrors(w, details)
	return false
}

func validationFieldPath(fieldError validator.FieldError) string {
	namespace := fieldError.Namespace()
	if _, path, ok := strings.Cut(namespace, "."); ok {
		return path
	}
	return fieldError.Field()
}

func decodeAndValidateJSON[T any](
	h *Handlers,
	w http.ResponseWriter,
	r *http.Request,
) (T, bool) {
	request, ok := decodeJSONRequest[T](w, r)
	if !ok {
		return request, false
	}
	if !h.validateRequest(w, "body", request) {
		return request, false
	}
	return request, true
}

func decodeJSONRequest[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var request T
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return request, false
	}
	return request, true
}

func validationMessage(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return "is required"
	case "oneof":
		if fieldError.Param() == "1" {
			return "must be 1 when present"
		}
		return "must be one of: " + strings.Join(strings.Fields(fieldError.Param()), ", ")
	case "min":
		return fmt.Sprintf("must be at least %s", fieldError.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fieldError.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fieldError.Param())
	case "hexcolor":
		return "must be a valid hexadecimal color"
	case "url":
		return "must be a valid URL"
	case "email":
		return "must be a valid email address"
	case "regexp":
		return "must be a valid regular expression"
	case "no_control_chars":
		return "must not contain control characters"
	case "iana_timezone":
		return "must be a valid IANA timezone"
	case "currency_code":
		return "must be a 3-letter ISO 4217 code"
	case "time_format":
		return "must be one of: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a"
	case "page_offset":
		return "is too large for page_size"
	case "heatmap_range":
		return "cannot be combined with from or to"
	default:
		return "is invalid"
	}
}

func writeValidationErrors(w http.ResponseWriter, details []ValidationErrorDetail) {
	writeJSON(w, http.StatusUnprocessableEntity, ValidationErrorResponse{
		Error:   "request validation failed",
		Details: details,
	})
}
