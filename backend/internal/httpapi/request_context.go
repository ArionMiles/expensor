package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func requestTenant(r *http.Request) store.Tenant {
	if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
		return store.Tenant{ID: principal.TenantID}
	}
	return store.Tenant{}
}

func uuidPathValue(w http.ResponseWriter, r *http.Request, name, label string) (string, bool) {
	value := r.PathValue(name)
	if _, err := uuid.Parse(value); err != nil {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid "+label+" id"), err))
		return "", false
	}
	return value, true
}
