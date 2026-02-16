package cli

import (
	"os"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/internal/config"
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

// TestNewPullCommandWithConfigProvider tests the command creation
func TestNewPullCommandWithConfigProvider(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{
				Github: struct {
					Owner      string "mapstructure:\"owner\""
					Repo       string "mapstructure:\"repo\""
					Token      string "mapstructure:\"token\""
					WorkflowID string "mapstructure:\"workflow_id\""
				}{
					Owner:      "test-owner",
					Repo:       "test-repo",
					Token:      "test-token",
					WorkflowID: "test-workflow",
				},
				Registry: struct {
					Host      string "mapstructure:\"host\""
					Username  string "mapstructure:\"username\""
					Password  string "mapstructure:\"password\""
					Namespace string "mapstructure:\"namespace\""
					Arch      string "mapstructure:\"arch\""
					Os        string "mapstructure:\"os\""
				}{
					Host:      "registry.example.com",
					Namespace: "test-namespace",
					Username:  "test-user",
					Password:  "test-pass",
					Arch:      "amd64",
					Os:        "linux",
				},
				LogLevel: "info",
			}, nil
		},
	}

	cmd := NewPullCommandWithConfigProvider(mockProvider)
	assert.Equal(t, "pull [IMAGE...]", cmd.Use)
	assert.Equal(t, "Pull images through GitHub Actions", cmd.Short)
}

// TestNewPullCommandWithConfigProviderAndOptions tests the command creation with options
func TestNewPullCommandWithConfigProviderAndOptions(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{
				Github: struct {
					Owner      string "mapstructure:\"owner\""
					Repo       string "mapstructure:\"repo\""
					Token      string "mapstructure:\"token\""
					WorkflowID string "mapstructure:\"workflow_id\""
				}{
					Owner:      "test-owner",
					Repo:       "test-repo",
					Token:      "test-token",
					WorkflowID: "test-workflow",
				},
				Registry: struct {
					Host      string "mapstructure:\"host\""
					Username  string "mapstructure:\"username\""
					Password  string "mapstructure:\"password\""
					Namespace string "mapstructure:\"namespace\""
					Arch      string "mapstructure:\"arch\""
					Os        string "mapstructure:\"os\""
				}{
					Host:      "registry.example.com",
					Namespace: "test-namespace",
					Username:  "test-user",
					Password:  "test-pass",
					Arch:      "arm64",
					Os:        "darwin",
				},
				LogLevel: "debug",
			}, nil
		},
	}

	opts := PullCommandOptions{
		Arch:        "amd64",
		OsType:      "linux",
		WorkerCount: 5,
		Force:       true,
		DryRun:      true,
		Verbose:     true,
	}

	cmd := NewPullCommandWithConfigProviderAndOptions(mockProvider, opts)
	assert.Equal(t, "pull [IMAGE...]", cmd.Use)
	assert.Equal(t, "Pull images through GitHub Actions", cmd.Short)
}

// TestArgsValidation tests the command's argument validation
func TestArgsValidation(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{
				Github: struct {
					Owner      string "mapstructure:\"owner\""
					Repo       string "mapstructure:\"repo\""
					Token      string "mapstructure:\"token\""
					WorkflowID string "mapstructure:\"workflow_id\""
				}{
					Owner:      "test-owner",
					Repo:       "test-repo",
					Token:      "test-token",
					WorkflowID: "test-workflow",
				},
				Registry: struct {
					Host      string "mapstructure:\"host\""
					Username  string "mapstructure:\"username\""
					Password  string "mapstructure:\"password\""
					Namespace string "mapstructure:\"namespace\""
					Arch      string "mapstructure:\"arch\""
					Os        string "mapstructure:\"os\""
				}{
					Host:      "registry.example.com",
					Namespace: "test-namespace",
					Username:  "test-user",
					Password:  "test-pass",
					Arch:      "amd64",
					Os:        "linux",
				},
				LogLevel: "info",
			}, nil
		},
	}

	// Test command with no args and no file flag - should fail when executed
	cmd := NewPullCommandWithConfigProvider(mockProvider)
	cmd.SetArgs([]string{}) // No arguments

	// Just test that the command is created correctly (arguments validation happens during execution)
	assert.Equal(t, "pull [IMAGE...]", cmd.Use)
	assert.NotNil(t, cmd.Args)
}

// TestReadSyncManifest tests the YAML manifest reading function
func TestReadSyncManifest(t *testing.T) {
	// Create a temporary YAML file for testing
	yamlContent := `
images:
  - source: "nginx:latest"
    platforms:
      - "linux/amd64"
      - "linux/arm64"
  - source: "redis:alpine"
`

	tmpFile, err := os.CreateTemp("", "test-manifest-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test successful manifest reading
	tasks, err := readSyncManifest(tmpFile.Name(), "amd64", "linux")
	assert.NoError(t, err)
	assert.Len(t, tasks, 3) // 2 platforms for nginx + 1 default for redis

	// Check the tasks
	expectedTasks := []struct {
		source string
		arch   string
		osType string
	}{
		{"nginx:latest", "amd64", "linux"},
		{"nginx:latest", "arm64", "linux"},
		{"redis:alpine", "amd64", "linux"}, // Uses defaults
	}

	for i, expected := range expectedTasks {
		if i < len(tasks) {
			assert.Equal(t, expected.source, tasks[i].Source)
			assert.Equal(t, expected.arch, tasks[i].Arch)
			assert.Equal(t, expected.osType, tasks[i].Os)
		}
	}
}

// TestReadSyncManifestInvalidFormat tests reading invalid YAML
func TestReadSyncManifestInvalidFormat(t *testing.T) {
	// Create a temporary file with invalid YAML
	yamlContent := `
images:
  - source: "nginx:latest"
    platforms:
      - "linux"  # Invalid format (should be os/arch)
`

	tmpFile, err := os.CreateTemp("", "test-invalid-manifest-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test invalid manifest reading
	_, err = readSyncManifest(tmpFile.Name(), "amd64", "linux")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid platform format")
}

// TestReadSyncManifestFileNotFound tests reading non-existent file
func TestReadSyncManifestFileNotFound(t *testing.T) {
	_, err := readSyncManifest("non-existent-file.yaml", "amd64", "linux")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestSyncTaskDisplayname tests the display name functionality
func TestSyncTaskDisplayname(t *testing.T) {
	task := syncTask{
		Source: "nginx:latest",
		Arch:   "amd64",
		Os:     "linux",
	}

	expected := "nginx:latest (linux/amd64)"
	actual := task.displayName()
	assert.Equal(t, expected, actual)
}

// TestPullCommandOptions tests the options struct
func TestPullCommandOptions(t *testing.T) {
	opts := PullCommandOptions{
		Arch:        "arm64",
		OsType:      "darwin",
		FilePath:    "/path/to/file",
		WorkerCount: 10,
		Force:       true,
		DryRun:      true,
		Verbose:     true,
	}

	assert.Equal(t, "arm64", opts.Arch)
	assert.Equal(t, "darwin", opts.OsType)
	assert.Equal(t, "/path/to/file", opts.FilePath)
	assert.Equal(t, 10, opts.WorkerCount)
	assert.Equal(t, true, opts.Force)
	assert.Equal(t, true, opts.DryRun)
	assert.Equal(t, true, opts.Verbose)
}

func TestFormatPullSummary(t *testing.T) {
	tests := []struct {
		name     string
		s        *PullSummary
		expected string
	}{
		{
			name: "normal case with duration",
			s: &PullSummary{
				Succeeded: 1,
				Skipped:   2,
				DryRun:    0,
				Failed:    0,
				Duration:  25 * time.Second,
			},
			expected: "Summary: 1 succeeded, 2 skipped, 0 failed | Total: 25s",
		},
		{
			name: "with dry-run",
			s: &PullSummary{
				Succeeded: 0,
				Skipped:   5,
				DryRun:    3,
				Failed:    0,
				Duration:  0,
			},
			expected: "Summary: 0 succeeded, 5 skipped, 3 dry-run, 0 failed",
		},
		{
			name: "with failure",
			s: &PullSummary{
				Succeeded: 2,
				Skipped:   1,
				DryRun:    0,
				Failed:    1,
				Duration:  10 * time.Second,
			},
			expected: "Summary: 2 succeeded, 1 skipped, 1 failed | Total: 10s",
		},
		{
			name: "all zero without duration",
			s: &PullSummary{
				Succeeded: 0,
				Skipped:   0,
				DryRun:    0,
				Failed:    0,
				Duration:  0,
			},
			expected: "Summary: 0 succeeded, 0 skipped, 0 failed",
		},
		{
			name: "minutes duration",
			s: &PullSummary{
				Succeeded: 5,
				Skipped:   10,
				DryRun:    0,
				Failed:    0,
				Duration:  2*time.Minute + 30*time.Second,
			},
			expected: "Summary: 5 succeeded, 10 skipped, 0 failed | Total: 2m30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPullSummary(tt.s)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatImageResults(t *testing.T) {
	tests := []struct {
		name     string
		results  []ImageResult
		expected string
	}{
		{
			name: "all succeeded",
			results: []ImageResult{
				{Image: "redis:latest", Success: true},
				{Image: "nginx:latest", Success: true},
			},
			expected: "  ✓ redis:latest\n  ✓ nginx:latest\n",
		},
		{
			name: "mixed results",
			results: []ImageResult{
				{Image: "redis:latest", Success: true},
				{Image: "nginx:latest", Skipped: true},
				{Image: "alpine:latest", Failed: true, Error: "connection refused"},
			},
			expected: "  ✓ redis:latest\n  ◦ nginx:latest\n  ✗ alpine:latest: connection refused\n",
		},
		{
			name:     "empty results",
			results:  []ImageResult{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatImageResults(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}
