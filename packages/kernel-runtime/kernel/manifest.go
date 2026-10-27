package kernel

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

var (
	packageNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	identifierPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	semverPattern      = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
)

// ValidateManifest checks a manifest for completeness, internal consistency,
// and compatibility with this kernel. It reports every problem, not just the first.
func ValidateManifest(m *contracts.Manifest) error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if m.APIVersion != ManifestAPIVersion {
		add("apiVersion must be %q, got %q", ManifestAPIVersion, m.APIVersion)
	}
	if !packageNamePattern.MatchString(m.Metadata.Name) {
		add("metadata.name %q must be lowercase alphanumeric with dashes", m.Metadata.Name)
	}
	if !semverPattern.MatchString(m.Metadata.Version) {
		add("metadata.version %q must be semantic (x.y.z)", m.Metadata.Version)
	}
	if m.Metadata.PackageAPIVersion != PackageAPIVersion {
		add("metadata.packageApiVersion %q is not supported (kernel speaks %q)", m.Metadata.PackageAPIVersion, PackageAPIVersion)
	}

	if !containsString(m.Spec.SupportedRuntimeVersions, RuntimeVersion) {
		add("spec.supportedRuntimeVersions %v does not include kernel runtime %q", m.Spec.SupportedRuntimeVersions, RuntimeVersion)
	}
	switch m.Spec.ExecutionTarget {
	case "cloud", "local", "any":
	default:
		add("spec.executionTarget must be cloud, local, or any, got %q", m.Spec.ExecutionTarget)
	}
	if len(m.Spec.AuthMethods) == 0 {
		add("spec.authMethods must not be empty")
	}

	if len(m.Spec.Capabilities) == 0 {
		add("spec.capabilities must not be empty")
	}
	caps := make(map[string]bool, len(m.Spec.Capabilities))
	for i, c := range m.Spec.Capabilities {
		if !identifierPattern.MatchString(c.Name) {
			add("spec.capabilities[%d].name %q is not a valid dotted identifier", i, c.Name)
			continue
		}
		if caps[c.Name] {
			add("spec.capabilities: duplicate name %q", c.Name)
		}
		caps[c.Name] = true
		for _, p := range c.Permissions {
			// Wildcards and admin belong to grants, never to requirements
			if p == "" || p == "admin" || !identifierPattern.MatchString(p) {
				add("spec.capabilities[%s]: invalid required permission %q", c.Name, p)
			}
		}
	}

	ops := make(map[string]bool, len(m.Spec.Operations))
	for i, op := range m.Spec.Operations {
		if !identifierPattern.MatchString(op.ID) {
			add("spec.operations[%d].id %q is not a valid dotted identifier", i, op.ID)
			continue
		}
		if ops[op.ID] {
			add("spec.operations: duplicate id %q", op.ID)
		}
		ops[op.ID] = true
		if !caps[op.Capability] {
			add("spec.operations[%s]: capability %q is not declared", op.ID, op.Capability)
		}
		if op.TimeoutSeconds < 0 {
			add("spec.operations[%s]: timeoutSeconds must not be negative", op.ID)
		}
		switch op.RetryPolicy {
		case "safe", "unsafe", "none":
		default:
			add("spec.operations[%s]: retryPolicy must be safe, unsafe, or none, got %q", op.ID, op.RetryPolicy)
		}
		if op.RetryPolicy == "safe" && !op.Idempotent {
			add("spec.operations[%s]: retryPolicy safe requires idempotent: true", op.ID)
		}
		if op.InputSchema != nil {
			if err := checkSchema(op.InputSchema); err != nil {
				add("spec.operations[%s].inputSchema: %v", op.ID, err)
			}
		}
	}

	return errors.Join(errs...)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
