// Package auth contains authentication primitives shared by the HTTP and store layers.
package auth

// Role identifies an instance-level account role.
type Role string

const (
	// RoleAdmin can manage instance users.
	RoleAdmin Role = "admin"
	// RoleUser can access their own tenant data.
	RoleUser Role = "user"
)

// Principal is the authenticated account identity attached to a request.
type Principal struct {
	UserID     string
	TenantID   string
	Role       Role
	AuthMethod string
}
