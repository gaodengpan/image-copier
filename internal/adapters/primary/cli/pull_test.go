package cli

import (
	"os"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

func TestCreateCoreConfigFromConfig(t *testing.T) {
	cfg := &config.Config{
		Github: struct {
			Owner      string `mapstructure:"owner"`
			Repo       string `mapstructure:"repo"`
			Token      string `mapstructure:"token"`
			WorkflowID string `mapstructure:"workflow_id"`
		}{
			Owner:      "owner",
			Repo:       "repo",
			Token:      "token",
			WorkflowID: "workflow.yaml",
		},
		Registry: struct {
			Host      string `mapstructure:"host"`
			Username  string `mapstructure:"username"`
			Password  string `mapstructure:"password"`
			Namespace string `mapstructure:"namespace"`
			Arch      string `mapstructure:"arch"`
			Os        string `mapstructure:"os"`
		}{
			Host:      "registry.example.com",
			Username:  "user",
			Password:  "pass",
			Namespace: "ns",
			Arch:      "amd64",
			Os:        "linux",
		},
		LogLevel: "info",
	}

	resultCfg := CreateCoreConfigFromConfig(cfg, true, true)

	assert.Equal(t, "owner", resultCfg.Github.Owner)
	assert.Equal(t, "repo", resultCfg.Github.Repo)
	assert.Equal(t, "token", resultCfg.Github.Token)
	assert.Equal(t, "workflow.yaml", resultCfg.Github.WorkflowID)
	assert.Equal(t, "registry.example.com", resultCfg.Registry.Host)
	assert.Equal(t, "user", resultCfg.Registry.Username)
	assert.Equal(t, "pass", resultCfg.Registry.Password)
	assert.Equal(t, "ns", resultCfg.Registry.Namespace)
	assert.Equal(t, "amd64", resultCfg.Registry.Arch)
	assert.Equal(t, "linux", resultCfg.Registry.Os)
	assert.Equal(t, true, resultCfg.Force)
	assert.Equal(t, true, resultCfg.DryRun)
}

func TestSetupLogger(t *testing.T) {
	t.Run("info level", func(t *testing.T) {
		cfg := &config.Config{LogLevel: "info"}
		logger := SetupLogger(cfg, false)
		assert.Equal(t, logrus.InfoLevel, logger.Level)
	})

	t.Run("debug level with verbose", func(t *testing.T) {
		cfg := &config.Config{LogLevel: "info"}
		logger := SetupLogger(cfg, true)
		assert.Equal(t, logrus.DebugLevel, logger.Level)
	})

	t.Run("invalid level defaults to info", func(t *testing.T) {
		cfg := &config.Config{LogLevel: "invalid"}
		logger := SetupLogger(cfg, false)
		assert.Equal(t, logrus.InfoLevel, logger.Level)
	})
}

func TestCalculateAdaptiveWorkerCount(t *testing.T) {
	tests := []struct {
		name          string
		userSpecified bool
		userValue     int
		taskCount     int
		cpuCount      int
		expected      int
	}{
		{
			name:          "user not specified, more tasks than cpu*4",
			userSpecified: false,
			userValue:     0,
			taskCount:     100,
			cpuCount:      4,
			expected:      16, // cpuCount * 4 = 16
		},
		{
			name:          "user not specified, fewer tasks than cpu*4",
			userSpecified: false,
			userValue:     0,
			taskCount:     5,
			cpuCount:      8,
			expected:      5, // taskCount < cpuCount * 4
		},
		{
			name:          "user not specified, exactly cpu*4",
			userSpecified: false,
			userValue:     0,
			taskCount:     16,
			cpuCount:      4,
			expected:      16,
		},
		{
			name:          "user not specified, single task",
			userSpecified: false,
			userValue:     0,
			taskCount:     1,
			cpuCount:      4,
			expected:      1,
		},
		{
			name:          "user not specified, zero tasks",
			userSpecified: false,
			userValue:     0,
			taskCount:     0,
			cpuCount:      4,
			expected:      1, // minimum is 1
		},
		{
			name:          "user specified, uses user value",
			userSpecified: true,
			userValue:     10,
			taskCount:     100,
			cpuCount:      4,
			expected:      10,
		},
		{
			name:          "user specified, zero tasks still uses user value",
			userSpecified: true,
			userValue:     5,
			taskCount:     0,
			cpuCount:      4,
			expected:      5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAdaptiveWorkerCount(tt.userSpecified, tt.userValue, tt.taskCount, tt.cpuCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCLIPresenter(t *testing.T) {
	presenter := NewCLIPresenter()

	t.Run("PresentCheckingImageCount", func(t *testing.T) {
		presenter.PresentCheckingImageCount(5)
	})

	t.Run("PresentDiffSummary", func(t *testing.T) {
		presenter.PresentDiffSummary(3, 2)
	})

	t.Run("PresentDryRunResults", func(t *testing.T) {
		synced := []syncTask{{Source: "img1", Arch: "amd64", Os: "linux"}}
		toSync := []syncTask{{Source: "img2", Arch: "amd64", Os: "linux"}}
		presenter.PresentDryRunResults(synced, toSync)
	})

	t.Run("PresentProgress", func(t *testing.T) {
		presenter.PresentProgress(1, 5)
	})

	t.Run("PresentSummary", func(t *testing.T) {
		summary := &PullSummary{Succeeded: 1, Skipped: 0, Failed: 0}
		results := []ImageResult{{Image: "test:latest", Success: true}}
		presenter.PresentSummary(summary, results)
	})

	t.Run("PresentError", func(t *testing.T) {
		presenter.PresentError(assert.AnError)
	})
}

func TestPullCommandFlags(t *testing.T) {
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

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, *cobra.Command)
	}{
		{
			name: "jobs flag exists",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("jobs")
				assert.NotNil(t, flag)
			},
		},
		{
			name: "output flag exists",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("output")
				assert.NotNil(t, flag)
				assert.Equal(t, "text", flag.Value.String())
			},
		},
		{
			name: "timeout flag exists",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("timeout")
				assert.NotNil(t, flag)
			},
		},
		{
			name: "target flag exists",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("target")
				assert.NotNil(t, flag)
				assert.Equal(t, "docker", flag.Value.String())
			},
		},
		{
			name: "registry flag exists",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("registry")
				assert.NotNil(t, flag)
			},
		},
		{
			name: "all flags exist",
			args: []string{"nginx:latest"},
			check: func(t *testing.T, cmd *cobra.Command) {
				assert.NotNil(t, cmd.Flags().Lookup("arch"))
				assert.NotNil(t, cmd.Flags().Lookup("os"))
				assert.NotNil(t, cmd.Flags().Lookup("file"))
				assert.NotNil(t, cmd.Flags().Lookup("jobs"))
				assert.NotNil(t, cmd.Flags().Lookup("force"))
				assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
				assert.NotNil(t, cmd.Flags().Lookup("verbose"))
				assert.NotNil(t, cmd.Flags().Lookup("output"))
				assert.NotNil(t, cmd.Flags().Lookup("timeout"))
				assert.NotNil(t, cmd.Flags().Lookup("target"))
				assert.NotNil(t, cmd.Flags().Lookup("registry"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewPullCommandWithConfigProvider(mockProvider)
			cmd.SetArgs(tt.args)
			tt.check(t, cmd)
		})
	}
}

func TestPullCommandDefaultValues(t *testing.T) {
	mockProvider := &MockConfigProvider{
		LoadFunc: func() (*config.Config, error) {
			return &config.Config{}, nil
		},
	}

	cmd := NewPullCommandWithConfigProvider(mockProvider)

	assert.Equal(t, "3", cmd.Flags().Lookup("jobs").DefValue)
	assert.Equal(t, "text", cmd.Flags().Lookup("output").DefValue)
	assert.Equal(t, "docker", cmd.Flags().Lookup("target").DefValue)
	assert.Equal(t, "0s", cmd.Flags().Lookup("timeout").DefValue)
}
