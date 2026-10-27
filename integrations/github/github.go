package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// BaseURL is the default GitHub API endpoint
const BaseURL = "https://api.github.com"

// Client wraps an HTTP client for GitHub API calls
type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

// NewClient creates a new GitHub API client
func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = BaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		baseURL:    baseURL,
	}
}

// GetAccount retrieves the authenticated user account
func (c *Client) GetAccount(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := jsonDecode(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// ListRepositories lists accessible repositories
func (c *Client) ListRepositories(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/user/repos?per_page=100", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result []map[string]any
	if err := jsonDecode(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// Integration implements the contracts.Package interface
type Integration struct {
	manifest *contracts.Manifest
}

// NewIntegration creates a new GitHub integration
func NewIntegration() *Integration {
	return &Integration{
		manifest: &contracts.Manifest{
			APIVersion: "omni/v1",
			Metadata: contracts.ManifestMeta{
				Name:              "github",
				DisplayName:       "GitHub",
				Version:           "1.0.0",
				PackageAPIVersion: "v1",
				Description:       "GitHub integration for repository and deployment operations",
			},
			Spec: contracts.ManifestSpec{
				SupportedRuntimeVersions: []string{"v1.0"},
				ExecutionTarget:          "cloud",
				NetworkRequirements: contracts.NetworkReq{
					Outbound: []string{"api.github.com:443"},
				},
				AuthMethods: []contracts.AuthMethod{contracts.AuthOAuth2, contracts.AuthAPIToken, contracts.AuthGitHubApp},
				Capabilities: []contracts.Capability{
					{Name: "account.read", Permissions: []string{"account.read"}},
					{Name: "repository.list", Permissions: []string{"repo.read"}},
					{Name: "repository.read", Permissions: []string{"repo.read"}},
					{Name: "repository.write", Permissions: []string{"repo.write"}},
					{Name: "pull_request.write", Permissions: []string{"repo.write"}},
					{Name: "issue.read", Permissions: []string{"repo.read"}},
				},
				Operations: []contracts.OperationDef{
					{ID: "account.get", Capability: "account.read", Handler: "GetAccount", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "repositories.list", Capability: "repository.list", Handler: "ListRepositories", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "repository.read_file", Capability: "repository.read", Handler: "ReadRepositoryFile", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "repository.create_branch", Capability: "repository.write", Handler: "CreateBranch", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "pull_request.create", Capability: "pull_request.write", Handler: "CreatePullRequest", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false},
					{ID: "issue.list", Capability: "issue.read", Handler: "ListIssues", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
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
	return []contracts.AuthMethod{contracts.AuthOAuth2, contracts.AuthAPIToken, contracts.AuthGitHubApp}
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

// jsonDecode is a helper to decode JSON from an io.Reader
func jsonDecode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
