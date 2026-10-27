package config

import (
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("expected HTTP addr :8080, got %s", cfg.HTTP.Addr)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level info, got %s", cfg.Log.Level)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Addr: ""},
		Log:  LogConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty HTTP addr")
	}

	cfg.HTTP.Addr = ":8080"
	cfg.Log.Level = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid log level")
	}
}

func validConfig() *Config {
	return &Config{
		HTTP:      HTTPConfig{Addr: ":8080"},
		Log:       LogConfig{Level: "info"},
		Execution: ExecutionConfig{MaxConcurrent: 1, Timeout: 1, MaxOutputBytes: 1},
	}
}

func TestSecurityValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{"defaults", func(c *Config) {}, false},
		{"valid credential key", func(c *Config) { c.Security.CredentialKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" }, false},
		{"short credential key", func(c *Config) { c.Security.CredentialKey = "c2hvcnQ=" }, true},
		{"non-base64 credential key", func(c *Config) { c.Security.CredentialKey = "!!!" }, true},
		{"short dev token", func(c *Config) { c.Security.DevToken = "short" }, true},
		{"zero concurrency", func(c *Config) { c.Execution.MaxConcurrent = 0 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(c)
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDevPermissionsParsing(t *testing.T) {
	t.Setenv("KERNEL_DEV_PERMISSIONS", " operation.execute, test.read ,,")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Security.DevPermissions) != 2 || cfg.Security.DevPermissions[1] != "test.read" {
		t.Fatalf("unexpected permissions %v", cfg.Security.DevPermissions)
	}
}
