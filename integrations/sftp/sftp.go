package sftp

import (
	"context"
	"fmt"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// FileInfo represents a file or directory entry
type FileInfo struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// Client wraps an SFTP client (stub implementation)
type Client struct {
	connected bool
}

// Connect creates an SFTP client (stub)
func (c *Client) Connect(ctx context.Context, host string, port int) error {
	c.connected = true
	return nil
}

// ListDir lists directory contents (stub)
func (c *Client) ListDir(path string) ([]FileInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	return []FileInfo{{Name: path, IsDir: true}}, nil
}

// Download downloads a file (stub)
func (c *Client) Download(remotePath, localPath string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// Upload uploads a file (stub)
func (c *Client) Upload(localPath, remotePath string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// Close closes the SFTP client
func (c *Client) Close() error {
	c.connected = false
	return nil
}

// Integration implements the contracts.Package interface
type Integration struct {
	manifest *contracts.Manifest
}

// NewIntegration creates a new SFTP integration
func NewIntegration() *Integration {
	return &Integration{
		manifest: &contracts.Manifest{
			APIVersion: "omni/v1",
			Metadata: contracts.ManifestMeta{
				Name:              "sftp",
				DisplayName:       "SFTP",
				Version:           "1.0.0",
				PackageAPIVersion: "v1",
				Description:       "SFTP integration for file transfers",
			},
			Spec: contracts.ManifestSpec{
				SupportedRuntimeVersions: []string{"v1.0"},
				ExecutionTarget:          "local",
				AuthMethods:              []contracts.AuthMethod{contracts.AuthSSHKey, contracts.AuthUsernamePass},
				Capabilities: []contracts.Capability{
					{Name: "directory.list", Permissions: []string{"file.read"}},
					{Name: "file.read", Permissions: []string{"file.read"}},
					{Name: "file.write", Permissions: []string{"file.write"}},
					{Name: "file.delete", Permissions: []string{"file.delete"}},
					{Name: "file.rename", Permissions: []string{"file.write"}},
					{Name: "directory.create", Permissions: []string{"file.write"}},
				},
				Operations: []contracts.OperationDef{
					{ID: "directory.list", Capability: "directory.list", Handler: "ListDirectory", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "file.read", Capability: "file.read", Handler: "ReadFile", TimeoutSeconds: 60, RetryPolicy: "safe", Idempotent: true},
					{ID: "file.write", Capability: "file.write", Handler: "WriteFile", TimeoutSeconds: 120, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "file.delete", Capability: "file.delete", Handler: "DeleteFile", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "file.rename", Capability: "file.rename", Handler: "RenameFile", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "directory.create", Capability: "directory.create", Handler: "CreateDirectory", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false},
				},
			},
		},
	}
}

func (i *Integration) Manifest() *contracts.Manifest { return i.manifest }
func (i *Integration) Initialize(ctx context.Context, runtime contracts.RuntimeContext) error {
	return nil
}
func (i *Integration) Shutdown(ctx context.Context) error { return nil }
func (i *Integration) SupportedAuthMethods() []contracts.AuthMethod {
	return []contracts.AuthMethod{contracts.AuthSSHKey, contracts.AuthUsernamePass}
}
func (i *Integration) StartAuth(ctx context.Context, config contracts.AuthConfig) (*contracts.AuthChallenge, error) {
	return nil, nil
}
func (i *Integration) CompleteAuth(ctx context.Context, response contracts.AuthResponse) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (i *Integration) ValidateCredential(ctx context.Context, cred *contracts.CredentialRef) error {
	return nil
}
func (i *Integration) RefreshCredential(ctx context.Context, cred *contracts.CredentialRef) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (i *Integration) RevokeCredential(ctx context.Context, cred *contracts.CredentialRef) error {
	return nil
}
func (i *Integration) ValidateConnection(ctx context.Context, conn *contracts.Connection) (*contracts.ConnectionStatus, error) {
	return &contracts.ConnectionStatus{Healthy: true, LastChecked: time.Now()}, nil
}
func (i *Integration) Capabilities() []contracts.Capability {
	return i.manifest.Spec.Capabilities
}

// Execute is not implemented yet; see docs/STATUS.md for the phase that delivers it
func (i *Integration) Execute(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
	return nil, contracts.NewError(contracts.ErrorCategoryNotSupported, "not_implemented", "operation "+op.OperationID+" is not implemented yet")
}
