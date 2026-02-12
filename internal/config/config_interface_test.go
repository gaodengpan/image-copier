package config_test

import (
	"testing"

	"github.com/gaodengpan/image-copier/internal/config"
)

// TestConfigProviderInterface tests the ConfigProvider interface with ViperConfigProvider
func TestConfigProviderInterface(t *testing.T) {
	// Create a new environment sandbox for this test
	envSandbox := config.NewEnvSandbox()
	defer envSandbox.Restore()

	// Setup required environment
	envSandbox.SetEnv("GITHUB_OWNER", "test-owner")
	envSandbox.SetEnv("GITHUB_REPO", "test-repo")
	envSandbox.SetEnv("GITHUB_TOKEN", "test-token")
	envSandbox.SetEnv("REGISTRY_HOST", "registry.example.com")
	envSandbox.SetEnv("REGISTRY_USERNAME", "test-user")
	envSandbox.SetEnv("REGISTRY_PASSWD", "test-pass")

	provider := config.NewViperConfigProvider()

	cfg, err := provider.Load()
	if err != nil {
		t.Fatalf("Expected no error when loading config, got %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config from provider")
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
}

// TestMockConfigProvider tests the mock implementation of ConfigProvider
func TestMockConfigProvider(t *testing.T) {
	expectedConfig := config.NewConfigBuilder().
		WithGithubOwner("test-owner").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		Build()

	mockProvider := &config.MockConfigProvider{
		ConfigToReturn: expectedConfig,
		ErrorToReturn:  nil,
	}

	actualConfig, err := mockProvider.Load()
	if err != nil {
		t.Fatalf("Expected no error from mock provider, got %v", err)
	}

	if actualConfig == nil {
		t.Fatal("Expected non-nil config from mock provider")
	}

	if actualConfig.Github.Owner != "test-owner" {
		t.Errorf("Expected Github.Owner to be 'test-owner', got '%s'", actualConfig.Github.Owner)
	}

	if actualConfig.Github.Repo != "test-repo" {
		t.Errorf("Expected Github.Repo to be 'test-repo', got '%s'", actualConfig.Github.Repo)
	}
}

// TestMockConfigProviderWithError tests the mock implementation returning an error
func TestMockConfigProviderWithError(t *testing.T) {
	mockProvider := &config.MockConfigProvider{
		ConfigToReturn: nil,
		ErrorToReturn:  &config.ValidationError{Field: "test-field", Value: "test-value"},
	}

	_, err := mockProvider.Load()
	if err == nil {
		t.Fatalf("Expected error from mock provider, got none")
	}
}

// TestConfigBuilder tests the ConfigBuilder implementation
func TestConfigBuilder(t *testing.T) {
	builder := config.NewConfigBuilder()

	cfg := builder.
		WithGithubOwner("test-owner").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		WithRegistryNamespace("test-namespace").
		WithRegistryArch("arm64").
		WithRegistryOs("darwin").
		WithLogLevel("debug").
		WithRetryMaxAttempts("5").
		WithRetryInitialInterval("2s").
		WithRetryMaxInterval("60s").
		Build()

	// Verify all values are set correctly
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

	if cfg.Registry.Namespace != "test-namespace" {
		t.Errorf("Expected Registry.Namespace to be 'test-namespace', got '%s'", cfg.Registry.Namespace)
	}

	if cfg.Registry.Arch != "arm64" {
		t.Errorf("Expected Registry.Arch to be 'arm64', got '%s'", cfg.Registry.Arch)
	}

	if cfg.Registry.Os != "darwin" {
		t.Errorf("Expected Registry.Os to be 'darwin', got '%s'", cfg.Registry.Os)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel to be 'debug', got '%s'", cfg.LogLevel)
	}

	if cfg.Retry.MaxAttempts != "5" {
		t.Errorf("Expected Retry.MaxAttempts to be '5', got '%s'", cfg.Retry.MaxAttempts)
	}

	if cfg.Retry.InitialInterval != "2s" {
		t.Errorf("Expected Retry.InitialInterval to be '2s', got '%s'", cfg.Retry.InitialInterval)
	}

	if cfg.Retry.MaxInterval != "60s" {
		t.Errorf("Expected Retry.MaxInterval to be '60s', got '%s'", cfg.Retry.MaxInterval)
	}
}

// TestConfigBuilderWithRetryConfig tests the ConfigBuilder with retry config
func TestConfigBuilderWithRetryConfig(t *testing.T) {
	builder := config.NewConfigBuilder()

	cfg := builder.
		WithGithubOwner("test-owner").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		WithRetryConfig("3", "1s", "30s").
		Build()

	if cfg.Retry.MaxAttempts != "3" {
		t.Errorf("Expected Retry.MaxAttempts to be '3', got '%s'", cfg.Retry.MaxAttempts)
	}

	if cfg.Retry.InitialInterval != "1s" {
		t.Errorf("Expected Retry.InitialInterval to be '1s', got '%s'", cfg.Retry.InitialInterval)
	}

	if cfg.Retry.MaxInterval != "30s" {
		t.Errorf("Expected Retry.MaxInterval to be '30s', got '%s'", cfg.Retry.MaxInterval)
	}
}