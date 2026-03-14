package gateways

import (
	"testing"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
)

type mockLogger struct{}

func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Info(args ...interface{})                  {}
func (m *mockLogger) Warn(args ...interface{})                  {}
func (m *mockLogger) Error(args ...interface{})                 {}

func TestConfigAdapter_StagingRegistryConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Registry.Host = "staging.example.com"
	cfg.Registry.Namespace = "myapp"
	cfg.Registry.Username = "user"
	cfg.Registry.Password = "pass"
	cfg.Registry.Arch = "amd64"
	cfg.Registry.Os = "linux"

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	assert.Equal(t, "staging.example.com", adapter.StagingRegistryHost())
	assert.Equal(t, "myapp", adapter.StagingRegistryNamespace())
	assert.Equal(t, "user", adapter.StagingRegistryUsername())
	assert.Equal(t, "pass", adapter.StagingRegistryPassword())
	assert.Equal(t, "amd64", adapter.DefaultArch())
	assert.Equal(t, "linux", adapter.DefaultOS())
}

func TestConfigAdapter_GetDistributionTargets(t *testing.T) {
	cfg := &config.Config{}
	cfg.Distribution.DefaultTargets = []string{"docker", "registry1"}

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	// With explicit targets
	targets := adapter.GetDistributionTargets([]string{"custom-target"})
	assert.Equal(t, []string{"custom-target"}, targets)

	// Without explicit targets (use config)
	targets = adapter.GetDistributionTargets(nil)
	assert.Equal(t, []string{"docker", "registry1"}, targets)
}

func TestConfigAdapter_GetPrivateRegistry(t *testing.T) {
	cfg := &config.Config{
		PrivateRegistries: []config.PrivateRegistry{
			{
				Name:     "registry1",
				Host:     "registry.example.com",
				Username: "user1",
				Password: "pass1",
			},
		},
	}

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	reg := adapter.GetPrivateRegistry("registry1")
	assert.NotNil(t, reg)
	assert.Equal(t, "registry1", reg.Name)
	assert.Equal(t, "registry.example.com", reg.Host)
	assert.Equal(t, "user1", reg.Username)
	assert.Equal(t, "pass1", reg.Password)

	// Non-existent registry
	reg = adapter.GetPrivateRegistry("nonexistent")
	assert.Nil(t, reg)
}

func TestConfigAdapter_BuildTargets(t *testing.T) {
	cfg := &config.Config{
		PrivateRegistries: []config.PrivateRegistry{
			{
				Name:     "my-registry",
				Host:     "registry.example.com",
				Username: "user",
				Password: "pass",
			},
		},
	}

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	targets := adapter.BuildTargets([]string{"docker", "my-registry"})

	assert.Len(t, targets, 2)
	// First should be Docker target
	assert.Equal(t, "docker", targets[0].Name())
	// Second should be registry target
	assert.Equal(t, "my-registry", targets[1].Name())
}

func TestConfigAdapter_BuildTargets_WithNonExistentRegistry(t *testing.T) {
	cfg := &config.Config{
		PrivateRegistries: []config.PrivateRegistry{},
	}

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	// Only docker target should be created, non-existent registry should be skipped
	targets := adapter.BuildTargets([]string{"docker", "nonexistent"})

	assert.Len(t, targets, 1)
	assert.Equal(t, "docker", targets[0].Name())
}

func TestConfigAdapter_NewConfigAdapter(t *testing.T) {
	cfg := &config.Config{}
	logger := &mockLogger{}

	adapter := NewConfigAdapter(cfg, logger)

	assert.NotNil(t, adapter)
}

func TestConfigAdapter_EmptyConfig(t *testing.T) {
	cfg := &config.Config{}

	adapter := NewConfigAdapter(cfg, &mockLogger{})

	// Should return empty values for empty config
	assert.Equal(t, "", adapter.StagingRegistryHost())
	assert.Equal(t, "", adapter.StagingRegistryNamespace())
	assert.Equal(t, "", adapter.StagingRegistryUsername())
	assert.Equal(t, "", adapter.StagingRegistryPassword())
	assert.Equal(t, "", adapter.DefaultArch())
	assert.Equal(t, "", adapter.DefaultOS())
}
