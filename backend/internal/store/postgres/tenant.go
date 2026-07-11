package postgres

import (
	"fmt"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func tenantIDParam(tenant store.Tenant) any {
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return nil
	}
	return tenantID
}

func tenantWhere(column, placeholder string) string {
	return fmt.Sprintf("%s IS NOT DISTINCT FROM %s", column, placeholder)
}
