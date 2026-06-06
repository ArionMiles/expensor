package api

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"
)

func newRequestValidator() *validator.Validate {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, tagName := range []string{"json", "query"} {
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
	return validate
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
			Field:    fieldError.Field(),
			Location: location,
			Message:  validationMessage(fieldError),
		})
	}
	writeValidationErrors(w, details)
	return false
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
	case "no_control_chars":
		return "must not contain control characters"
	case "iana_timezone":
		return "must be a valid IANA timezone"
	case "currency_code":
		return "must be a 3-letter ISO 4217 code"
	case "time_format":
		return "must be one of: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a"
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
