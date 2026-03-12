package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	data := &config.ConfigData{
		GitHubOwner:       "owner",
		GitHubRepo:        "repo",
		GitHubToken:       "token",
		GitHubWorkflowID:  "workflow.yaml",
		RegistryHost:      "ghcr.io",
		RegistryUsername:  "user",
		RegistryPassword:  "pass",
		RegistryNamespace: "ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	err := WriteConfigFile(data, configPath)
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "owner")
	assert.Contains(t, string(content), "repo")
	assert.Contains(t, string(content), "ghcr.io")
}

func TestWriteConfigFile_WithPrivateRegistries(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	data := &config.ConfigData{
		GitHubOwner:       "owner",
		GitHubRepo:        "repo",
		GitHubToken:       "token",
		GitHubWorkflowID:  "workflow.yaml",
		RegistryHost:      "ghcr.io",
		RegistryUsername:  "user",
		RegistryPassword:  "pass",
		RegistryNamespace: "ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
		PrivateRegistries: []config.PrivateRegistry{
			{Name: "private1", Host: "private1.io", Username: "user1", Password: "pass1"},
			{Name: "private2", Host: "private2.io", Username: "user2", Password: "pass2"},
		},
	}

	err := WriteConfigFile(data, configPath)
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "private1")
	assert.Contains(t, string(content), "private2")
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "****"},
		{"verylongtokenvalue", "very****alue"},
		{"1234567890", "1234****7890"},
		{"abcdefgh", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskToken(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadConfig_WithEncryption(t *testing.T) {
	// Set required environment variables for config validation
	t.Setenv("GITHUB_OWNER", "test-owner")
	t.Setenv("GITHUB_REPO", "test-repo")
	t.Setenv("GITHUB_TOKEN", "test-token")

	provider := config.DefaultConfigProvider()
	cfg, err := provider.Load()
	require.NoError(t, err)

	assert.NotNil(t, cfg.Github)
	assert.Equal(t, "test-owner", cfg.Github.Owner)
}
