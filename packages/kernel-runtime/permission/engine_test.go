package permission

import "testing"

func TestHasPermission(t *testing.T) {
	engine := New()

	g := &Grantee{Permissions: []string{"repo.read", "repo.write"}}
	if !engine.HasPermission(g, Permission("repo.read")) {
		t.Error("expected repo.read to be granted")
	}
	if !engine.HasPermission(g, Permission("repo.write")) {
		t.Error("expected repo.write to be granted")
	}
	if engine.HasPermission(g, Permission("repo.admin")) {
		t.Error("expected repo.admin to be denied")
	}

	// Admin grants everything
	gAdmin := &Grantee{Permissions: []string{"admin"}}
	if !engine.HasPermission(gAdmin, Permission("repo.read")) {
		t.Error("admin should grant repo.read")
	}
	if !engine.HasPermission(gAdmin, Permission("anything")) {
		t.Error("admin should grant anything")
	}
}

func TestWildcardPermission(t *testing.T) {
	engine := New()
	g := &Grantee{Permissions: []string{"repo.*"}}
	if !engine.HasPermission(g, Permission("repo.read")) {
		t.Error("wildcard should grant repo.read")
	}
	if !engine.HasPermission(g, Permission("repo.write")) {
		t.Error("wildcard should grant repo.write")
	}
	if engine.HasPermission(g, Permission("repo")) {
		t.Error("wildcard should not grant bare 'repo'")
	}
}

func TestAuthorize(t *testing.T) {
	engine := New()
	g := &Grantee{Permissions: []string{"repo.read"}}

	decision := engine.Authorize(g, []string{"repo.read"})
	if !decision.Allowed {
		t.Errorf("expected allowed, got %s", decision.Reason)
	}

	decision = engine.Authorize(g, []string{"repo.read", "repo.admin"})
	if decision.Allowed {
		t.Error("expected denied for missing repo.admin")
	}
}

func TestFilterCapabilities(t *testing.T) {
	engine := New()
	g := &Grantee{Permissions: []string{"repo.read"}}

	caps := []CapabilityRef{
		{Name: "read", Permissions: []string{"repo.read"}},
		{Name: "admin", Permissions: []string{"repo.admin"}},
	}
	filtered := engine.FilterCapabilities(g, caps)
	if len(filtered) != 1 {
		t.Errorf("expected 1 capability, got %d", len(filtered))
	}
	if filtered[0].Name != "read" {
		t.Errorf("expected 'read', got %s", filtered[0].Name)
	}
}
