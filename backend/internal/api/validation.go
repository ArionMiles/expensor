package api

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

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
	return validate
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
		return "must be one of: " + strings.Join(strings.Fields(fieldError.Param()), ", ")
	case "min":
		return fmt.Sprintf("must be at least %s", fieldError.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fieldError.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fieldError.Param())
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
