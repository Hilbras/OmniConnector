package ssh

import (
	"context"
	"fmt"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// Config holds SSH connection configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	Timeout  time.Duration
}

// Client wraps an SSH connection (stub implementation)
type Client struct {
	config *sshClientConfig
}

// sshClientConfig holds the internal SSH configuration
type sshClientConfig struct {
	host      string
	port      int
	user      string
	timeout   time.Duration
	connected bool
}

// DefaultTimeout is the default connection timeout
const DefaultTimeout = 30 * time.Second

// NewClient creates a new SSH client
func NewClient(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	return &Client{
		config: &sshClientConfig{
			host:    cfg.Host,
			port:    cfg.Port,
			user:    cfg.User,
			timeout: cfg.Timeout,
		},
	}
}

// Connect establishes an SSH connection (stub)
func (c *Client) Connect(ctx context.Context, cfg Config) error {
	c.config.connected = true
	return nil
}

// Execute runs a command on the remote server (stub)
func (c *Client) Execute(ctx context.Context, command string) (string, int, error) {
	if !c.config.connected {
		return "", -1, fmt.Errorf("not connected")
	}
	return "stub output", 0, nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	c.config.connected = false
	return nil
}

// IsConnected reports whether the client is connected
func (c *Client) IsConnected() bool {
	return c.config.connected
}

// Integration implements the contracts.Package interface
type Integration struct {
	manifest *contracts.Manifest
}

// NewIntegration creates a new SSH integration
func NewIntegration() *Integration {
	return &Integration{
		manifest: &contracts.Manifest{
			APIVersion: "omni/v1",
			Metadata: contracts.ManifestMeta{
				Name:              "ssh",
				DisplayName:       "SSH",
				Version:           "1.0.0",
				PackageAPIVersion: "v1",
				Description:       "SSH integration for remote server management",
			},
			Spec: contracts.ManifestSpec{
				SupportedRuntimeVersions: []string{"v1.0"},
				ExecutionTarget:          "local",
				AuthMethods:              []contracts.AuthMethod{contracts.AuthSSHKey, contracts.AuthUsernamePass},
				Capabilities: []contracts.Capability{
					{Name: "connection.test", Permissions: []string{"server.read"}},
					{Name: "system.info", Permissions: []string{"server.read"}},
					{Name: "command.execute", Permissions: []string{"server.execute"}},
					{Name: "directory.list", Permissions: []string{"server.read"}},
				},
				Operations: []contracts.OperationDef{
					{ID: "connection.test", Capability: "connection.test", Handler: "TestConnection", TimeoutSeconds: 10, RetryPolicy: "safe", Idempotent: true},
					{ID: "system.info", Capability: "system.info", Handler: "GetSystemInfo", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "command.execute", Capability: "command.execute", Handler: "ExecuteCommand", TimeoutSeconds: 60, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "directory.list", Capability: "directory.list", Handler: "ListDirectory", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
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
