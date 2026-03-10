package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockConfigProvider is a mock implementation of the ConfigProvider interface
type MockConfigProvider struct {
	LoadFunc func() (*config.Config, error)
}

func (m *MockConfigProvider) Load() (*config.Config, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}
	return &config.Config{}, nil
}

func (m *MockConfigProvider) GetConfigPath() string {
	return "/mock/path/config.yaml"
}

func TestNewConfigCommandWithConfigProvider(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{}, nil
		},
	}

	cmd := NewConfigCommandWithConfigProvider(mockProvider)
	assert.Equal(t, "config", cmd.Use)
	assert.Len(t, cmd.Commands(), 2)

	for _, c := range cmd.Commands() {
		switch c.Use {
		case "show":
			assert.Equal(t, "Show current configuration", c.Short)
		case "init":
			assert.Equal(t, "Create configuration file interactively", c.Short)
		default:
			t.Errorf("unexpected subcommand: %s", c.Use)
		}
	}
}

func TestConfigShowCommand(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{
				Github: struct {
					Owner      string "mapstructure:\"owner\""
					Repo       string "mapstructure:\"repo\""
					Token      string "mapstructure:\"token\""
					WorkflowID string "mapstructure:\"workflow_id\""
				}{
					Owner:      "my-owner",
					Repo:       "my-repo",
					Token:      "my-token",
					WorkflowID: "workflow.yaml",
				},
				Registry: struct {
					Host      string "mapstructure:\"host\""
					Username  string "mapstructure:\"username\""
					Password  string "mapstructure:\"password\""
					Namespace string "mapstructure:\"namespace\""
					Arch      string "mapstructure:\"arch\""
					Os        string "mapstructure:\"os\""
				}{
					Host:      "ghcr.io",
					Namespace: "my-ns",
					Username:  "user",
					Password:  "pass",
					Arch:      "arm64",
					Os:        "linux",
				},
				LogLevel: "debug",
			}, nil
		},
	}

	cmd := newConfigShowCommandWithProvider(mockProvider)
	cmd.SetArgs([]string{})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	outputStr := buf.String()

	assert.Contains(t, outputStr, "GitHub Owner: my-owner")
	assert.Contains(t, outputStr, "GitHub Repo: my-repo")
	assert.Contains(t, outputStr, "GitHub Workflow ID: workflow.yaml")
	assert.Contains(t, outputStr, "Registry Host: ghcr.io")
	assert.Contains(t, outputStr, "Registry Namespace: my-ns")
	assert.Contains(t, outputStr, "Registry Arch: arm64")
	assert.Contains(t, outputStr, "Registry OS: linux")
	assert.Contains(t, outputStr, "Log Level: debug")
}

func TestConfigShowCommandLoadError(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return nil, assert.AnError
		},
	}

	cmd := newConfigShowCommandWithProvider(mockProvider)
	cmd.SetArgs([]string{})

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

func TestConfigInitCommandFlags(t *testing.T) {
	mockProvider := &MockConfigProvider{}

	cmd := newConfigInitCommand(mockProvider)

	flag := cmd.Flags().Lookup("skip-existing")
	require.NotNil(t, flag)
	assert.Equal(t, "true", flag.Value.String())

	flag = cmd.Flags().Lookup("force")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.Value.String())
}

func TestConfigShowCommandAllFields(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{
				Github: struct {
					Owner      string "mapstructure:\"owner\""
					Repo       string "mapstructure:\"repo\""
					Token      string "mapstructure:\"token\""
					WorkflowID string "mapstructure:\"workflow_id\""
				}{
					Owner:      "owner",
					Repo:       "repo",
					Token:      "token",
					WorkflowID: "workflow.yaml",
				},
				Registry: struct {
					Host      string "mapstructure:\"host\""
					Username  string "mapstructure:\"username\""
					Password  string "mapstructure:\"password\""
					Namespace string "mapstructure:\"namespace\""
					Arch      string "mapstructure:\"arch\""
					Os        string "mapstructure:\"os\""
				}{
					Host:      "ghcr.io",
					Namespace: "ns",
					Username:  "user",
					Password:  "pass",
					Arch:      "amd64",
					Os:        "linux",
				},
				LogLevel: "info",
			}, nil
		},
	}

	cmd := newConfigShowCommandWithProvider(mockProvider)
	cmd.SetArgs([]string{})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	outputStr := buf.String()

	assert.Contains(t, outputStr, "GitHub Owner: owner")
	assert.Contains(t, outputStr, "GitHub Repo: repo")
	assert.Contains(t, outputStr, "GitHub Workflow ID: workflow.yaml")
	assert.Contains(t, outputStr, "Registry Host: ghcr.io")
	assert.Contains(t, outputStr, "Registry Username: user")
	assert.Contains(t, outputStr, "Registry Namespace: ns")
	assert.Contains(t, outputStr, "Registry Arch: amd64")
	assert.Contains(t, outputStr, "Registry OS: linux")
	assert.Contains(t, outputStr, "Log Level: info")
}

func TestConfigShowCommand_NoPathMethod(t *testing.T) {
	mp := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{}, nil
		},
	}

	_ = mp.GetConfigPath()
}
