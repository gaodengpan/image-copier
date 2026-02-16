package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeSourceID(t *testing.T) {
	tests := []struct {
		name     string
		sourceID string
		expected string
	}{
		{
			name:     "short id",
			sourceID: "nginx:latest",
			expected: "nginx:latest",
		},
		{
			name:     "exactly at max length",
			sourceID: "abcdefghijklmnopqrstuvwx",
			expected: "abcdefghijklmnopqrstuvwx",
		},
		{
			name:     "long id truncated",
			sourceID: "verylongimagetagthatneedstruncation:latest",
			expected: "verylongimagetagthatneedstruncation:late...",
		},
		{
			name:     "empty string",
			sourceID: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSourceID(tt.sourceID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPullStage(t *testing.T) {
	assert.Equal(t, PullStage(0), StageCheckLocal)
	assert.Equal(t, PullStage(1), StageCheckRegistry)
	assert.Equal(t, PullStage(2), StageTriggerWorkflow)
	assert.Equal(t, PullStage(3), StageWaitWorkflow)
	assert.Equal(t, PullStage(4), StageDownload)
	assert.Equal(t, PullStage(5), StageLoad)
}

func TestConfig(t *testing.T) {
	cfg := Config{
		GithubOwner:       "owner",
		GithubRepo:        "repo",
		GithubToken:       "token",
		GithubWorkflowID:  "workflow.yaml",
		RegistryHost:      "registry.io",
		RegistryUsername:  "user",
		RegistryPassword:  "pass",
		RegistryNamespace: "ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
		Force:             true,
		DryRun:            false,
	}

	assert.Equal(t, "owner", cfg.GithubOwner)
	assert.Equal(t, "repo", cfg.GithubRepo)
	assert.Equal(t, "registry.io", cfg.RegistryHost)
	assert.True(t, cfg.Force)
	assert.False(t, cfg.DryRun)
}

func TestRetryConfig(t *testing.T) {
	retryCfg := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1,
		MaxDelay:     30,
	}

	assert.Equal(t, 5, retryCfg.MaxAttempts)
	assert.Equal(t, 1, retryCfg.InitialDelay)
	assert.Equal(t, 30, retryCfg.MaxDelay)
}

func TestErrorConstants(t *testing.T) {
	assert.NotNil(t, ErrSkipped)
	assert.Equal(t, "image skipped", ErrSkipped.Error())

	assert.NotNil(t, ErrDryRun)
	assert.Equal(t, "dry run mode", ErrDryRun.Error())
}
