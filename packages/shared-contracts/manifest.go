package contracts

// AuthMethod identifies an authentication flow
type AuthMethod string

const (
	AuthOAuth2       AuthMethod = "oauth2"
	AuthAPIToken     AuthMethod = "apiToken"
	AuthSSHKey       AuthMethod = "sshKey"
	AuthUsernamePass AuthMethod = "usernamePassword"
	AuthCertificate  AuthMethod = "certificate"
	AuthGitHubApp    AuthMethod = "githubApp"
)

// Manifest represents an integration package manifest
type Manifest struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Metadata   ManifestMeta `json:"metadata" yaml:"metadata"`
	Spec       ManifestSpec `json:"spec" yaml:"spec"`
}

// ManifestMeta holds metadata about the integration
type ManifestMeta struct {
	Name              string `json:"name" yaml:"name"`
	DisplayName       string `json:"displayName" yaml:"displayName"`
	Version           string `json:"version" yaml:"version"`
	PackageAPIVersion string `json:"packageApiVersion" yaml:"packageApiVersion"`
	Description       string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ManifestSpec holds the specification for the integration
type ManifestSpec struct {
	SupportedRuntimeVersions []string       `json:"supportedRuntimeVersions" yaml:"supportedRuntimeVersions"`
	ExecutionTarget          string         `json:"executionTarget" yaml:"executionTarget"` // cloud | local | any
	NetworkRequirements      NetworkReq     `json:"networkRequirements,omitempty" yaml:"networkRequirements,omitempty"`
	Dependencies             []string       `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	AuthMethods              []AuthMethod   `json:"authMethods" yaml:"authMethods"`
	ConfigSchema             map[string]any `json:"configSchema,omitempty" yaml:"configSchema,omitempty"`
	Capabilities             []Capability   `json:"capabilities" yaml:"capabilities"`
	Operations               []OperationDef `json:"operations" yaml:"operations"`
}

// NetworkReq holds network requirements
type NetworkReq struct {
	Outbound []string `json:"outbound,omitempty" yaml:"outbound,omitempty"`
	Inbound  []string `json:"inbound,omitempty" yaml:"inbound,omitempty"`
}

// Capability represents an abstract unit of functionality
type Capability struct {
	Name        string   `json:"name" yaml:"name"`
	Permissions []string `json:"permissions" yaml:"permissions"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}

// OperationDef defines an available operation
type OperationDef struct {
	ID             string `json:"id" yaml:"id"`
	Capability     string `json:"capability" yaml:"capability"`
	Handler        string `json:"handler" yaml:"handler"`
	TimeoutSeconds int    `json:"timeoutSeconds" yaml:"timeoutSeconds"`
	RetryPolicy    string `json:"retryPolicy" yaml:"retryPolicy"` // safe | unsafe | none
	Idempotent     bool   `json:"idempotent" yaml:"idempotent"`
	// InputSchema is a JSON-Schema subset (type, properties, required,
	// additionalProperties) the kernel validates input against before dispatch.
	InputSchema map[string]any `json:"inputSchema,omitempty" yaml:"inputSchema,omitempty"`
}
