package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field: "github.owner",
		Value: "",
	}
	expected := "validation error for field github.owner: invalid value "
	assert.Equal(t, expected, err.Error())
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		Github: struct {
			Owner      string `mapstructure:"owner"`
			Repo       string `mapstructure:"repo"`
			Token      string `mapstructure:"token"`
			WorkflowID string `mapstructure:"workflow_id"`
		}{
			Owner: "owner", Repo: "repo", Token: "token",
		},
		Registry: struct {
			Host      string `mapstructure:"host"`
			Username  string `mapstructure:"username"`
			Password  string `mapstructure:"password"`
			Namespace string `mapstructure:"namespace"`
			Arch      string `mapstructure:"arch"`
			Os        string `mapstructure:"os"`
		}{
			Host: "registry.io", Username: "user", Password: "pass",
		},
	}
	err := ValidateConfig(cfg)
	assert.NoError(t, err)

	cfg.Github.Owner = ""
	err = ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "github owner")
}

func TestConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder().
		WithGithubOwner("owner").
		WithGithubRepo("repo").
		WithGithubToken("token").
		WithGithubWorkflowID("wf.yaml").
		WithRegistryHost("registry.io").
		WithRegistryUsername("user").
		WithRegistryPassword("pass").
		WithRegistryNamespace("ns").
		WithRegistryArch("amd64").
		WithRegistryOs("linux").
		WithLogLevel("debug").
		WithRetryMaxAttempts("5").
		WithRetryInitialInterval("1s").
		WithRetryMaxInterval("30s")

	cfg := builder.Build()
	assert.Equal(t, "owner", cfg.Github.Owner)
	assert.Equal(t, "repo", cfg.Github.Repo)
	assert.Equal(t, "token", cfg.Github.Token)
	assert.Equal(t, "wf.yaml", cfg.Github.WorkflowID)
	assert.Equal(t, "registry.io", cfg.Registry.Host)
	assert.Equal(t, "user", cfg.Registry.Username)
	assert.Equal(t, "pass", cfg.Registry.Password)
	assert.Equal(t, "ns", cfg.Registry.Namespace)
	assert.Equal(t, "amd64", cfg.Registry.Arch)
	assert.Equal(t, "linux", cfg.Registry.Os)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "5", cfg.Retry.MaxAttempts)
	assert.Equal(t, "1s", cfg.Retry.InitialInterval)
	assert.Equal(t, "30s", cfg.Retry.MaxInterval)
}

func TestParseRetryConfig(t *testing.T) {
	cfg := &Config{
		Retry: struct {
			MaxAttempts     string `mapstructure:"max_attempts"`
			InitialInterval string `mapstructure:"initial_interval"`
			MaxInterval     string `mapstructure:"max_interval"`
		}{
			MaxAttempts: "5", InitialInterval: "1s", MaxInterval: "30s",
		},
	}
	result := cfg.ParseRetryConfig()
	assert.Equal(t, 5, result.MaxAttempts)
	assert.Equal(t, 1*time.Second, result.InitialInterval)
	assert.Equal(t, 30*time.Second, result.MaxInterval)

	cfg.Retry.MaxAttempts = "invalid"
	cfg.Retry.InitialInterval = "invalid"
	cfg.Retry.MaxInterval = "invalid"
	result = cfg.ParseRetryConfig()
	assert.Equal(t, 3, result.MaxAttempts)
}
