package postgres

import (
	"fmt"
	"strings"
)

func tenantIDParam(tenant Tenant) any {
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return nil
	}
	return tenantID
}

func tenantWhere(column, placeholder string) string {
	return fmt.Sprintf("%s IS NOT DISTINCT FROM %s", column, placeholder)
}
