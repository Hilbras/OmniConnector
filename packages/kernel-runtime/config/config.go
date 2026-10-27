package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Config holds the kernel runtime configuration
type Config struct {
	HTTP HTTPConfig
	GRPC GRPCConfig
	Log  LogConfig
	// Database
	Database DatabaseConfig
	// Execution
	Execution ExecutionConfig
	// Security
	Security SecurityConfig
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	Addr string
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Addr string
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	DSN string
}

// ExecutionConfig holds execution configuration
type ExecutionConfig struct {
	MaxConcurrent  int
	Timeout        int // seconds; default when an operation declares none
	MaxOutputBytes int // upper bound on a serialized operation result
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	JWTSecret string
	// CredentialKey is the base64-encoded 32-byte key for credential
	// encryption. When empty, an ephemeral key is generated at startup and
	// stored credentials do not survive a restart.
	CredentialKey string
	// DevToken is a static bearer token accepted by the API until user
	// authentication lands (Phase 2). Empty disables it: every
	// authenticated route then returns 401.
	DevToken string
	// DevPermissions are the permissions granted to the DevToken caller
	DevPermissions []string
	// EnableTestIntegration registers the test integration at startup
	EnableTestIntegration bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr: getEnv("KERNEL_HTTP_ADDR", ":8080"),
		},
		GRPC: GRPCConfig{
			Addr: getEnv("KERNEL_GRPC_ADDR", ":9090"),
		},
		Log: LogConfig{
			Level: getEnv("KERNEL_LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			DSN: getEnv("KERNEL_DATABASE_DSN", "postgres://omniconnector:omniconnector@localhost:5432/omniconnector?sslmode=disable"),
		},
		Execution: ExecutionConfig{
			MaxConcurrent:  getEnvInt("KERNEL_EXECUTION_MAX_CONCURRENT", 10),
			Timeout:        getEnvInt("KERNEL_EXECUTION_TIMEOUT", 300),
			MaxOutputBytes: getEnvInt("KERNEL_EXECUTION_MAX_OUTPUT_BYTES", 1<<20),
		},
		Security: SecurityConfig{
			JWTSecret:             getEnv("KERNEL_JWT_SECRET", "change-me-in-production"),
			CredentialKey:         getEnv("KERNEL_CREDENTIAL_KEY", ""),
			DevToken:              getEnv("KERNEL_DEV_TOKEN", ""),
			DevPermissions:        getEnvList("KERNEL_DEV_PERMISSIONS", []string{"operation.execute", "operation.cancel"}),
			EnableTestIntegration: getEnv("KERNEL_ENABLE_TEST_INTEGRATION", "") == "true",
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("HTTP address is required")
	}
	if c.Log.Level == "" {
		return fmt.Errorf("log level is required")
	}
	// Validate log level
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}
	if c.Execution.MaxConcurrent < 1 {
		return fmt.Errorf("execution max concurrent must be at least 1")
	}
	if c.Execution.Timeout < 1 {
		return fmt.Errorf("execution timeout must be at least 1 second")
	}
	if c.Execution.MaxOutputBytes < 1 {
		return fmt.Errorf("execution max output bytes must be positive")
	}
	if c.Security.CredentialKey != "" {
		key, err := base64.StdEncoding.DecodeString(c.Security.CredentialKey)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("credential key must be base64 encoding of exactly 32 bytes")
		}
	}
	if c.Security.DevToken != "" && len(c.Security.DevToken) < 32 {
		return fmt.Errorf("dev token must be at least 32 characters")
	}
	return nil
}

// CredentialKeyBytes returns the decoded credential key, or nil if unset
func (c *Config) CredentialKeyBytes() []byte {
	if c.Security.CredentialKey == "" {
		return nil
	}
	key, _ := base64.StdEncoding.DecodeString(c.Security.CredentialKey)
	return key
}

func getEnvList(key string, defaultVal []string) []string {
	val := getEnv(key, "")
	if val == "" {
		return defaultVal
	}
	var out []string
	for _, p := range strings.Split(val, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := getEnv(key, "")
	if val == "" {
		return defaultVal
	}
	var i int
	if _, err := fmt.Sscanf(val, "%d", &i); err != nil {
		return defaultVal
	}
	return i
}
