package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// BaseURL is the default Vercel API endpoint
const BaseURL = "https://api.vercel.com"

// Client wraps an HTTP client for Vercel API calls
type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

// NewClient creates a new Vercel API client
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

// ListProjects lists accessible projects
func (c *Client) ListProjects(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v3/projects", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := decodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	projects, ok := result["projects"].([]any)
	if !ok {
		return nil, fmt.Errorf("no projects found in response")
	}

	resultProjects := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		proj, ok := p.(map[string]any)
		if ok {
			resultProjects = append(resultProjects, proj)
		}
	}
	return resultProjects, nil
}

// ListDeployments lists deployments for a project
func (c *Client) ListDeployments(ctx context.Context, projectID string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v3/projects/"+projectID+"/deployments?limit=50", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := decodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	deployments, ok := result["deployments"].([]any)
	if !ok {
		return nil, fmt.Errorf("no deployments found in response")
	}

	resultDeployments := make([]map[string]any, 0, len(deployments))
	for _, d := range deployments {
		dep, ok := d.(map[string]any)
		if ok {
			resultDeployments = append(resultDeployments, dep)
		}
	}
	return resultDeployments, nil
}

// GetDeployment retrieves a deployment by ID
func (c *Client) GetDeployment(ctx context.Context, deploymentID string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v3/deployments/"+deploymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := decodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// Integration implements the contracts.Package interface
type Integration struct {
	manifest *contracts.Manifest
}

// NewIntegration creates a new Vercel integration
func NewIntegration() *Integration {
	return &Integration{
		manifest: &contracts.Manifest{
			APIVersion: "omni/v1",
			Metadata: contracts.ManifestMeta{
				Name:              "vercel",
				DisplayName:       "Vercel",
				Version:           "1.0.0",
				PackageAPIVersion: "v1",
				Description:       "Vercel integration for deployment and project management",
			},
			Spec: contracts.ManifestSpec{
				SupportedRuntimeVersions: []string{"v1.0"},
				ExecutionTarget:          "cloud",
				NetworkRequirements: contracts.NetworkReq{
					Outbound: []string{"api.vercel.com:443"},
				},
				AuthMethods: []contracts.AuthMethod{contracts.AuthOAuth2, contracts.AuthAPIToken},
				Capabilities: []contracts.Capability{
					{Name: "project.list", Permissions: []string{"project.read"}},
					{Name: "deployment.list", Permissions: []string{"deployment.read"}},
					{Name: "deployment.status", Permissions: []string{"deployment.read"}},
				},
				Operations: []contracts.OperationDef{
					{ID: "project.list", Capability: "project.list", Handler: "ListProjects", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "deployment.list", Capability: "deployment.list", Handler: "ListDeployments", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
					{ID: "deployment.status", Capability: "deployment.status", Handler: "GetDeploymentStatus", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true},
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
	return []contracts.AuthMethod{contracts.AuthOAuth2, contracts.AuthAPIToken}
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

// decodeJSON is a helper to decode JSON from an io.Reader
func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
