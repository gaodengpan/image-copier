package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/pkg/retry"
)

// TestValidateConfig_Success tests successful config validation
func TestValidateConfig_Success(t *testing.T) {
	// Save and restore environment
	envVars := map[string]string{
		"GITHUB_OWNER":       os.Getenv("GITHUB_OWNER"),
		"GITHUB_REPO":        os.Getenv("GITHUB_REPO"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
		"REGISTRY_HOST":      os.Getenv("REGISTRY_HOST"),
		"REGISTRY_USERNAME":  os.Getenv("REGISTRY_USERNAME"),
		"REGISTRY_PASSWD":    os.Getenv("REGISTRY_PASSWD"),
	}
	defer func() {
		for k, v := range envVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set environment variables
	os.Setenv("GITHUB_OWNER", "test-owner")
	os.Setenv("GITHUB_REPO", "test-repo")
	os.Setenv("GITHUB_TOKEN", "test-token")
	os.Setenv("REGISTRY_HOST", "registry.example.com")
	os.Setenv("REGISTRY_USERNAME", "test-user")
	os.Setenv("REGISTRY_PASSWD", "test-pass")

	cfg, err := config.Load()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify the config values
	if cfg.Github.Owner != "test-owner" {
		t.Errorf("Expected Github.Owner to be 'test-owner', got '%s'", cfg.Github.Owner)
	}

	if cfg.Github.Repo != "test-repo" {
		t.Errorf("Expected Github.Repo to be 'test-repo', got '%s'", cfg.Github.Repo)
	}

	if cfg.Github.Token != "test-token" {
		t.Errorf("Expected Github.Token to be 'test-token', got '%s'", cfg.Github.Token)
	}

	if cfg.Registry.Host != "registry.example.com" {
		t.Errorf("Expected Registry.Host to be 'registry.example.com', got '%s'", cfg.Registry.Host)
	}

	if cfg.Registry.Username != "test-user" {
		t.Errorf("Expected Registry.Username to be 'test-user', got '%s'", cfg.Registry.Username)
	}

	if cfg.Registry.Password != "test-pass" {
		t.Errorf("Expected Registry.Password to be 'test-pass', got '%s'", cfg.Registry.Password)
	}

	// Check defaults
	if cfg.Registry.Arch != "amd64" {
		t.Errorf("Expected default Registry.Arch to be 'amd64', got '%s'", cfg.Registry.Arch)
	}

	if cfg.Registry.Os != "linux" {
		t.Errorf("Expected default Registry.Os to be 'linux', got '%s'", cfg.Registry.Os)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel to be 'info', got '%s'", cfg.LogLevel)
	}
}

// TestValidateConfig_MissingRequired tests validation errors for missing required fields
func TestValidateConfig_MissingRequired(t *testing.T) {
	// Create a temporary directory for isolated testing
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Save and restore environment
	envVars := map[string]string{
		"GITHUB_OWNER":       os.Getenv("GITHUB_OWNER"),
		"GITHUB_REPO":        os.Getenv("GITHUB_REPO"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
		"REGISTRY_HOST":      os.Getenv("REGISTRY_HOST"),
		"REGISTRY_USERNAME":  os.Getenv("REGISTRY_USERNAME"),
		"REGISTRY_PASSWD":    os.Getenv("REGISTRY_PASSWD"),
	}
	defer func() {
		for k, v := range envVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all environment variables first
	os.Unsetenv("GITHUB_OWNER")
	os.Unsetenv("GITHUB_REPO")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("REGISTRY_HOST")
	os.Unsetenv("REGISTRY_USERNAME")
	os.Unsetenv("REGISTRY_PASSWD")

	// Ensure no config file exists
	os.Remove("config.yaml")
	os.Remove(filepath.Join(os.Getenv("HOME"), ".image-copier", "config.yaml"))

	tests := []struct {
		name       string
		unsetVars  []string
		setVars    map[string]string
		wantErr    bool
		errContains string
	}{
		{
			name:      "all required vars set",
			setVars:   map[string]string{"GITHUB_OWNER": "o", "GITHUB_REPO": "r", "GITHUB_TOKEN": "t", "REGISTRY_HOST": "h", "REGISTRY_USERNAME": "u", "REGISTRY_PASSWD": "p"},
			wantErr:   false,
		},
		{
			name:       "missing GITHUB_OWNER",
			unsetVars:  []string{"GITHUB_OWNER"},
			setVars:    map[string]string{"GITHUB_REPO": "r", "GITHUB_TOKEN": "t", "REGISTRY_HOST": "h", "REGISTRY_USERNAME": "u", "REGISTRY_PASSWD": "p"},
			wantErr:    true,
			errContains: "github owner",
		},
		{
			name:       "missing GITHUB_REPO",
			unsetVars:  []string{"GITHUB_REPO"},
			setVars:    map[string]string{"GITHUB_OWNER": "o", "GITHUB_TOKEN": "t", "REGISTRY_HOST": "h", "REGISTRY_USERNAME": "u", "REGISTRY_PASSWD": "p"},
			wantErr:    true,
			errContains: "github repo",
		},
		{
			name:       "missing GITHUB_TOKEN",
			unsetVars:  []string{"GITHUB_TOKEN"},
			setVars:    map[string]string{"GITHUB_OWNER": "o", "GITHUB_REPO": "r", "REGISTRY_HOST": "h", "REGISTRY_USERNAME": "u", "REGISTRY_PASSWD": "p"},
			wantErr:    true,
			errContains: "github token",
		},
		{
			name:       "missing REGISTRY_HOST",
			unsetVars:  []string{"REGISTRY_HOST"},
			setVars:    map[string]string{"GITHUB_OWNER": "o", "GITHUB_REPO": "r", "GITHUB_TOKEN": "t", "REGISTRY_USERNAME": "u", "REGISTRY_PASSWD": "p"},
			wantErr:    true,
			errContains: "registry host",
		},
		{
			name:       "missing REGISTRY_USERNAME",
			unsetVars:  []string{"REGISTRY_USERNAME"},
			setVars:    map[string]string{"GITHUB_OWNER": "o", "GITHUB_REPO": "r", "GITHUB_TOKEN": "t", "REGISTRY_HOST": "h", "REGISTRY_PASSWD": "p"},
			wantErr:    true,
			errContains: "registry username",
		},
		{
			name:       "missing REGISTRY_PASSWD",
			unsetVars:  []string{"REGISTRY_PASSWD"},
			setVars:    map[string]string{"GITHUB_OWNER": "o", "GITHUB_REPO": "r", "GITHUB_TOKEN": "t", "REGISTRY_HOST": "h", "REGISTRY_USERNAME": "u"},
			wantErr:    true,
			errContains: "registry password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save current environment
			savedVars := make(map[string]string)
			for k := range tt.setVars {
				savedVars[k] = os.Getenv(k)
			}
			for _, k := range tt.unsetVars {
				savedVars[k] = os.Getenv(k)
			}

			// Clean up at the end of test
			defer func() {
				for k, v := range savedVars {
					if v == "" {
						os.Unsetenv(k)
					} else {
						os.Setenv(k, v)
					}
				}
			}()

			// Unset all var first
			for _, v := range tt.unsetVars {
				os.Unsetenv(v)
			}

			// Set specific vars
			for k, v := range tt.setVars {
				os.Setenv(k, v)
			}

			_, err := config.Load()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" {
					if !containsString(err.Error(), tt.errContains) {
						t.Errorf("Expected error to contain '%s', got '%s'", tt.errContains, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestLoadConfig_WithDefaults tests that default values are set
func TestLoadConfig_WithDefaults(t *testing.T) {
	// Create a temporary directory for isolated testing
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Save and restore environment
	envVars := map[string]string{
		"GITHUB_OWNER":       os.Getenv("GITHUB_OWNER"),
		"GITHUB_REPO":        os.Getenv("GITHUB_REPO"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
		"REGISTRY_HOST":      os.Getenv("REGISTRY_HOST"),
		"REGISTRY_USERNAME":  os.Getenv("REGISTRY_USERNAME"),
		"REGISTRY_PASSWD":    os.Getenv("REGISTRY_PASSWD"),
		"GITHUB_WORKFLOW_ID": os.Getenv("GITHUB_WORKFLOW_ID"),
		"REGISTRY_ARCH":      os.Getenv("REGISTRY_ARCH"),
		"REGISTRY_OS":        os.Getenv("REGISTRY_OS"),
	}
	defer func() {
		for k, v := range envVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all relevant environment variables
	os.Unsetenv("GITHUB_OWNER")
	os.Unsetenv("GITHUB_REPO")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("REGISTRY_HOST")
	os.Unsetenv("REGISTRY_USERNAME")
	os.Unsetenv("REGISTRY_PASSWD")
	os.Unsetenv("GITHUB_WORKFLOW_ID")
	os.Unsetenv("REGISTRY_ARCH")
	os.Unsetenv("REGISTRY_OS")

	// Ensure no config file exists
	os.Remove("config.yaml")
	os.Remove(filepath.Join(os.Getenv("HOME"), ".image-copier", "config.yaml"))

	// Setup minimal required environment
	os.Setenv("GITHUB_OWNER", "test-owner")
	os.Setenv("GITHUB_REPO", "test-repo")
	os.Setenv("GITHUB_TOKEN", "test-token")
	os.Setenv("REGISTRY_HOST", "registry.example.com")
	os.Setenv("REGISTRY_USERNAME", "test-user")
	os.Setenv("REGISTRY_PASSWD", "test-pass")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify defaults
	if cfg.Registry.Arch != "amd64" {
		t.Errorf("Expected default Arch to be 'amd64', got '%s'", cfg.Registry.Arch)
	}

	if cfg.Registry.Os != "linux" {
		t.Errorf("Expected default Os to be 'linux', got '%s'", cfg.Registry.Os)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel to be 'info', got '%s'", cfg.LogLevel)
	}

	if cfg.Github.WorkflowID != "image-copier-v2.yaml" {
		t.Errorf("Expected default WorkflowID to be 'image-copier-v2.yaml', got '%s'", cfg.Github.WorkflowID)
	}
}

// TestGetConfigPath tests getting config file path
func TestGetConfigPath(t *testing.T) {
	// Set up a temp XDG dir
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// No file yet — should return empty
	path := config.GetConfigPath()
	if path != "" {
		t.Errorf("Expected empty path when config file doesn't exist, got '%s'", path)
	}

	// Create the config file
	configDir := filepath.Join(tmpDir, "image-copier")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("test: config"), 0644)

	// Now GetConfigPath should find the file
	path = config.GetConfigPath()
	expected := filepath.Join(configDir, "config.yaml")
	if path != expected {
		t.Errorf("Expected path to be '%s', got '%s'", expected, path)
	}
}

// TestConfigFilePath tests ConfigFilePath with XDG_CONFIG_HOME
func TestConfigFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	expected := filepath.Join(tmpDir, "image-copier", "config.yaml")
	got := config.ConfigFilePath()
	if got != expected {
		t.Errorf("ConfigFilePath() = %q, want %q", got, expected)
	}
}

// TestConfigFilePath_DefaultXDG tests ConfigFilePath without XDG_CONFIG_HOME
func TestConfigFilePath_DefaultXDG(t *testing.T) {
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	os.Unsetenv("XDG_CONFIG_HOME")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "image-copier", "config.yaml")
	got := config.ConfigFilePath()
	if got != expected {
		t.Errorf("ConfigFilePath() = %q, want %q", got, expected)
	}
}

// TestLoadConfig_WithOptionalEnv tests optional environment variables
func TestLoadConfig_WithOptionalEnv(t *testing.T) {
	// Save and restore environment
	envVars := map[string]string{
		"GITHUB_OWNER":       os.Getenv("GITHUB_OWNER"),
		"GITHUB_REPO":        os.Getenv("GITHUB_REPO"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
		"REGISTRY_HOST":      os.Getenv("REGISTRY_HOST"),
		"REGISTRY_USERNAME":  os.Getenv("REGISTRY_USERNAME"),
		"REGISTRY_PASSWD":    os.Getenv("REGISTRY_PASSWD"),
		"REGISTRY_NAMESPACE": os.Getenv("REGISTRY_NAMESPACE"),
		"REGISTRY_ARCH":      os.Getenv("REGISTRY_ARCH"),
		"REGISTRY_OS":        os.Getenv("REGISTRY_OS"),
	}
	defer func() {
		for k, v := range envVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Setup required env vars
	os.Setenv("GITHUB_OWNER", "test-owner")
	os.Setenv("GITHUB_REPO", "test-repo")
	os.Setenv("GITHUB_TOKEN", "test-token")
	os.Setenv("REGISTRY_HOST", "registry.example.com")
	os.Setenv("REGISTRY_USERNAME", "test-user")
	os.Setenv("REGISTRY_PASSWD", "test-pass")

	// Test custom optional values
	os.Setenv("REGISTRY_NAMESPACE", "my-namespace")
	os.Setenv("REGISTRY_ARCH", "arm64")
	os.Setenv("REGISTRY_OS", "windows")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Registry.Namespace != "my-namespace" {
		t.Errorf("Expected Namespace to be 'my-namespace', got '%s'", cfg.Registry.Namespace)
	}

	if cfg.Registry.Arch != "arm64" {
		t.Errorf("Expected Arch to be 'arm64', got '%s'", cfg.Registry.Arch)
	}

	if cfg.Registry.Os != "windows" {
		t.Errorf("Expected Os to be 'windows', got '%s'", cfg.Registry.Os)
	}
}

// TestLoadConfig_WithFile tests loading config from a YAML file
func TestLoadConfig_WithFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	configContent := `
github:
  owner: test-owner-from-file
  repo: test-repo-from-file
  token: test-token-from-file
  workflow_id: custom-workflow.yaml

registry:
  host: registry.file.example.com
  username: user-from-file
  password: pass-from-file
  namespace: namespace-from-file
  arch: arm64
  os: darwin

retry:
  max_attempts: "5"
  initial_interval: "2s"
  max_interval: "60s"

log_level: "debug"
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	os.Chdir(tmpDir)

	// Save and restore environment
	envVars := map[string]string{
		"GITHUB_OWNER":       os.Getenv("GITHUB_OWNER"),
		"GITHUB_REPO":        os.Getenv("GITHUB_REPO"),
		"GITHUB_TOKEN":       os.Getenv("GITHUB_TOKEN"),
		"REGISTRY_HOST":      os.Getenv("REGISTRY_HOST"),
		"REGISTRY_USERNAME":  os.Getenv("REGISTRY_USERNAME"),
		"REGISTRY_PASSWD":    os.Getenv("REGISTRY_PASSWD"),
	}
	defer func() {
		for k, v := range envVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear environment variables to ensure file values are used
	os.Unsetenv("GITHUB_OWNER")
	os.Unsetenv("GITHUB_REPO")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("REGISTRY_HOST")
	os.Unsetenv("REGISTRY_USERNAME")
	os.Unsetenv("REGISTRY_PASSWD")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	// Verify file values (they should override environment)
	if cfg.Github.Owner != "test-owner-from-file" {
		t.Errorf("Expected Github.Owner from file, got '%s'", cfg.Github.Owner)
	}

	if cfg.Registry.Host != "registry.file.example.com" {
		t.Errorf("Expected Registry.Host from file, got '%s'", cfg.Registry.Host)
	}

	if cfg.Registry.Namespace != "namespace-from-file" {
		t.Errorf("Expected Registry.Namespace from file, got '%s'", cfg.Registry.Namespace)
	}

	if cfg.Registry.Arch != "arm64" {
		t.Errorf("Expected Registry.Arch from file to be 'arm64', got '%s'", cfg.Registry.Arch)
	}

	if cfg.Registry.Os != "darwin" {
		t.Errorf("Expected Registry.Os from file to be 'darwin', got '%s'", cfg.Registry.Os)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel from file to be 'debug', got '%s'", cfg.LogLevel)
	}

	if cfg.Retry.MaxAttempts != "5" {
		t.Errorf("Expected MaxAttempts from file to be '5', got '%s'", cfg.Retry.MaxAttempts)
	}
}

// --- ParseRetryConfig tests ---

func TestParseRetryConfig_AllFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Retry.MaxAttempts = "5"
	cfg.Retry.InitialInterval = "2s"
	cfg.Retry.MaxInterval = "60s"

	rc := cfg.ParseRetryConfig()

	if rc.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", rc.MaxAttempts)
	}
	if rc.InitialInterval != 2*time.Second {
		t.Errorf("InitialInterval = %v, want 2s", rc.InitialInterval)
	}
	if rc.MaxInterval != 60*time.Second {
		t.Errorf("MaxInterval = %v, want 60s", rc.MaxInterval)
	}
}

func TestParseRetryConfig_EmptyFields(t *testing.T) {
	cfg := &config.Config{}

	rc := cfg.ParseRetryConfig()
	defaults := retry.DefaultConfig()

	if rc.MaxAttempts != defaults.MaxAttempts {
		t.Errorf("MaxAttempts = %d, want default %d", rc.MaxAttempts, defaults.MaxAttempts)
	}
	if rc.InitialInterval != defaults.InitialInterval {
		t.Errorf("InitialInterval = %v, want default %v", rc.InitialInterval, defaults.InitialInterval)
	}
	if rc.MaxInterval != defaults.MaxInterval {
		t.Errorf("MaxInterval = %v, want default %v", rc.MaxInterval, defaults.MaxInterval)
	}
}

func TestParseRetryConfig_PartialFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Retry.MaxAttempts = "10"

	rc := cfg.ParseRetryConfig()
	defaults := retry.DefaultConfig()

	if rc.MaxAttempts != 10 {
		t.Errorf("MaxAttempts = %d, want 10", rc.MaxAttempts)
	}
	if rc.InitialInterval != defaults.InitialInterval {
		t.Errorf("InitialInterval = %v, want default %v", rc.InitialInterval, defaults.InitialInterval)
	}
	if rc.MaxInterval != defaults.MaxInterval {
		t.Errorf("MaxInterval = %v, want default %v", rc.MaxInterval, defaults.MaxInterval)
	}
}

func TestParseRetryConfig_InvalidFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Retry.MaxAttempts = "notanumber"
	cfg.Retry.InitialInterval = "invalid"
	cfg.Retry.MaxInterval = "invalid"

	rc := cfg.ParseRetryConfig()
	defaults := retry.DefaultConfig()

	if rc.MaxAttempts != defaults.MaxAttempts {
		t.Errorf("MaxAttempts = %d, want default %d", rc.MaxAttempts, defaults.MaxAttempts)
	}
	if rc.InitialInterval != defaults.InitialInterval {
		t.Errorf("InitialInterval = %v, want default %v", rc.InitialInterval, defaults.InitialInterval)
	}
	if rc.MaxInterval != defaults.MaxInterval {
		t.Errorf("MaxInterval = %v, want default %v", rc.MaxInterval, defaults.MaxInterval)
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}