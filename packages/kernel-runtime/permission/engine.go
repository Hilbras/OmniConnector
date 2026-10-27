package permission

import (
	"fmt"
	"strings"
)

// Permission is a dotted permission string such as "repo.read"
type Permission string

const (
	// Connection lifecycle
	PermissionConnectionRead   Permission = "connection.read"
	PermissionConnectionWrite  Permission = "connection.write"
	PermissionConnectionDelete Permission = "connection.delete"

	// Operation execution
	PermissionOperationExecute Permission = "operation.execute"
	PermissionOperationCancel  Permission = "operation.cancel"

	// Approval workflow
	PermissionApprovalReview Permission = "approval.review"

	// Audit access
	PermissionAuditRead Permission = "audit.read"

	// Admin
	PermissionAdmin Permission = "admin"
)

// Grantee represents an entity that holds permissions
type Grantee struct {
	UserID      string
	AppID       string
	DeviceID    string
	Permissions []string
}

// Engine evaluates permissions
type Engine struct{}

// New creates a permission engine
func New() *Engine {
	return &Engine{}
}

// Decision describes an authorization decision
type Decision struct {
	Allowed bool
	Reason  string
}

// HasPermission reports whether the grantee holds the given permission
func (e *Engine) HasPermission(g *Grantee, required Permission) bool {
	if g == nil {
		return false
	}
	requiredStr := string(required)

	for _, p := range g.Permissions {
		if p == requiredStr {
			return true
		}
		// admin grants everything
		if p == string(PermissionAdmin) {
			return true
		}
		// wildcard matching: "repo.*" matches "repo.read"
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(requiredStr, prefix) {
				return true
			}
		}
	}

	return false
}

// HasAllPermissions reports whether the grantee holds all required permissions
func (e *Engine) HasAllPermissions(g *Grantee, required []Permission) bool {
	for _, p := range required {
		if !e.HasPermission(g, p) {
			return false
		}
	}
	return true
}

// Authorize evaluates an operation request against capability permissions
func (e *Engine) Authorize(g *Grantee, capabilityPermissions []string) Decision {
	if g == nil {
		return Decision{Allowed: false, Reason: "no identity context"}
	}
	if len(capabilityPermissions) == 0 {
		return Decision{Allowed: true, Reason: "capability requires no permissions"}
	}

	for _, required := range capabilityPermissions {
		if !e.HasPermission(g, Permission(required)) {
			return Decision{
				Allowed: false,
				Reason:  fmt.Sprintf("missing required permission: %s", required),
			}
		}
	}

	return Decision{Allowed: true, Reason: "all required permissions granted"}
}

// FilterCapabilities returns only the capabilities the grantee is allowed to use
func (e *Engine) FilterCapabilities(g *Grantee, capabilities []CapabilityRef) []CapabilityRef {
	allowed := make([]CapabilityRef, 0, len(capabilities))
	for _, cap := range capabilities {
		if e.HasAllPermissions(g, toPermissions(cap.Permissions)) {
			allowed = append(allowed, cap)
		}
	}
	return allowed
}

// CapabilityRef is a lightweight capability reference
type CapabilityRef struct {
	Name        string
	Permissions []string
}

func toPermissions(perms []string) []Permission {
	result := make([]Permission, len(perms))
	for i, p := range perms {
		result[i] = Permission(p)
	}
	return result
}
