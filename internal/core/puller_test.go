package core

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

func newTestPuller(cfg *Config) *Puller {
	if cfg == nil {
		cfg = &Config{
			GithubOwner:      "owner",
			GithubRepo:       "repo",
			GithubToken:      "token",
			GithubWorkflowID: "workflow.yaml",
			RegistryHost:     "registry.example.com",
			RegistryUsername:  "user",
			RegistryPassword:  "pass",
			RegistryNamespace: "ns",
			RegistryArch:     "amd64",
			RegistryOs:       "linux",
		}
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return NewPuller(cfg, logger)
}

// --- NewPuller tests ---

func TestNewPuller(t *testing.T) {
	cfg := &Config{
		GithubOwner:  "myowner",
		RegistryHost: "myhost",
	}
	logger := logrus.New()
	p := NewPuller(cfg, logger)

	if p == nil {
		t.Fatal("expected non-nil puller")
	}
	if p.Config != cfg {
		t.Error("expected config to be set")
	}
	if p.Logger != logger {
		t.Error("expected logger to be set")
	}
	if p.RetryConfig == nil {
		t.Error("expected retry config to be set")
	}
}

func TestNewPuller_UsesRetryConfig(t *testing.T) {
	customRC := &retry.Config{
		MaxAttempts:     10,
		InitialInterval: 5 * time.Second,
		MaxInterval:     120 * time.Second,
	}
	cfg := &Config{
		GithubOwner:  "owner",
		RegistryHost: "host",
		RetryConfig:  customRC,
	}
	logger := logrus.New()
	p := NewPuller(cfg, logger)

	if p.RetryConfig != customRC {
		t.Error("expected Puller to use the provided RetryConfig")
	}
	if p.RetryConfig.MaxAttempts != 10 {
		t.Errorf("MaxAttempts = %d, want 10", p.RetryConfig.MaxAttempts)
	}
}

func TestNewPuller_NilRetryConfigUsesDefault(t *testing.T) {
	cfg := &Config{
		GithubOwner:  "owner",
		RegistryHost: "host",
	}
	logger := logrus.New()
	p := NewPuller(cfg, logger)

	defaults := retry.DefaultConfig()
	if p.RetryConfig.MaxAttempts != defaults.MaxAttempts {
		t.Errorf("MaxAttempts = %d, want default %d", p.RetryConfig.MaxAttempts, defaults.MaxAttempts)
	}
	if p.RetryConfig.InitialInterval != defaults.InitialInterval {
		t.Errorf("InitialInterval = %v, want default %v", p.RetryConfig.InitialInterval, defaults.InitialInterval)
	}
}

// --- ErrSkipped tests ---

func TestErrSkipped(t *testing.T) {
	if !errors.Is(ErrSkipped, ErrSkipped) {
		t.Error("expected ErrSkipped to match itself via errors.Is")
	}

	wrapped := fmt.Errorf("wrapped: %w", ErrSkipped)
	if !errors.Is(wrapped, ErrSkipped) {
		t.Error("expected wrapped ErrSkipped to match via errors.Is")
	}
}

// --- ErrDryRun tests ---

func TestErrDryRun(t *testing.T) {
	if !errors.Is(ErrDryRun, ErrDryRun) {
		t.Error("expected ErrDryRun to match itself via errors.Is")
	}

	wrapped := fmt.Errorf("wrapped: %w", ErrDryRun)
	if !errors.Is(wrapped, ErrDryRun) {
		t.Error("expected wrapped ErrDryRun to match via errors.Is")
	}
}

// --- NormalizeSourceID tests ---

func TestNormalizeSourceID_SingleSegment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"nginx", "docker.io/library/nginx:latest"},
		{"redis", "docker.io/library/redis:latest"},
		{"alpine", "docker.io/library/alpine:latest"},
		{"nginx:latest", "docker.io/library/nginx:latest"},
		{"nginx:1.21", "docker.io/library/nginx:1.21"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeSourceID(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeSourceID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeSourceID_TwoSegments(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// No dot or colon in first segment -> prepend docker.io, append :latest
		{"library/nginx", "docker.io/library/nginx:latest"},
		{"myuser/myrepo", "docker.io/myuser/myrepo:latest"},
		// Domain-like first segment (has dot) -> keep as-is, append :latest
		{"ghcr.io/nginx", "ghcr.io/nginx:latest"},
		{"quay.io/redis", "quay.io/redis:latest"},
		// Port in first segment (has colon) -> keep as-is, append :latest
		{"localhost:5000/myimage", "localhost:5000/myimage:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeSourceID(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeSourceID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeSourceID_FullyQualified(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// No tag -> append :latest
		{"docker.io/library/nginx", "docker.io/library/nginx:latest"},
		{"a/b/c", "a/b/c:latest"},
		// Has tag -> unchanged
		{"ghcr.io/owner/repo:tag", "ghcr.io/owner/repo:tag"},
		{"registry.example.com/ns/image:v1", "registry.example.com/ns/image:v1"},
		// Has digest -> unchanged
		{"ghcr.io/img@sha256:abc123", "ghcr.io/img@sha256:abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeSourceID(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeSourceID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- BuildDestImageID tests ---

func TestBuildDestImageID_WithNamespace(t *testing.T) {
	tests := []struct {
		sourceID string
		expected string
	}{
		{"docker.io/library/nginx", "registry.example.com/mynamespace/docker.io_library_nginx"},
		{"redis", "registry.example.com/mynamespace/redis"},
	}

	for _, tt := range tests {
		t.Run(tt.sourceID, func(t *testing.T) {
			got := BuildDestImageID("registry.example.com", "mynamespace", tt.sourceID)
			if got != tt.expected {
				t.Errorf("BuildDestImageID(%q) = %q, want %q", tt.sourceID, got, tt.expected)
			}
		})
	}
}

func TestBuildDestImageID_WithoutNamespace(t *testing.T) {
	got := BuildDestImageID("registry.example.com", "", "docker.io/library/nginx")
	expected := "registry.example.com/docker.io/library/nginx"
	if got != expected {
		t.Errorf("BuildDestImageID = %q, want %q", got, expected)
	}
}

func TestBuildDestImageID_Truncation(t *testing.T) {
	// A very long source ID should be truncated to 40 chars after normalization
	longSource := "docker.io/library/a-very-long-image-name-that-will-be-truncated-for-sure"
	got := BuildDestImageID("registry.example.com", "ns", longSource)

	// The normalized part (slashes replaced with underscores) should be at most 40 chars
	// Format: registry.example.com/ns/<normalized>
	prefix := "registry.example.com/ns/"
	if len(got) <= len(prefix) {
		t.Fatalf("unexpected short result: %s", got)
	}
	normalizedPart := got[len(prefix):]
	if len(normalizedPart) > 40 {
		t.Errorf("expected normalized part to be <=40 chars, got %d: '%s'", len(normalizedPart), normalizedPart)
	}
}

// --- PullStage tests ---

func TestPullStage_Constants(t *testing.T) {
	if StageCheckLocal != 0 {
		t.Errorf("StageCheckLocal = %d, want 0", StageCheckLocal)
	}
	if StageCheckRegistry != 1 {
		t.Errorf("StageCheckRegistry = %d, want 1", StageCheckRegistry)
	}
	if StageTriggerWorkflow != 2 {
		t.Errorf("StageTriggerWorkflow = %d, want 2", StageTriggerWorkflow)
	}
	if StageWaitWorkflow != 3 {
		t.Errorf("StageWaitWorkflow = %d, want 3", StageWaitWorkflow)
	}
	if StageCopyImage != 4 {
		t.Errorf("StageCopyImage = %d, want 4", StageCopyImage)
	}
	if StageLoadImage != 5 {
		t.Errorf("StageLoadImage = %d, want 5", StageLoadImage)
	}
}

func TestPuller_NotifyStage_Nil(t *testing.T) {
	p := newTestPuller(nil)
	// Should not panic when StageCallback is nil
	p.notifyStage(StageCheckLocal, 0)
	p.notifyStage(StageWaitWorkflow, 5)
}

func TestPuller_NotifyStage_Called(t *testing.T) {
	p := newTestPuller(nil)

	var calls []struct {
		stage PullStage
		polls int
	}

	p.StageCallback = func(stage PullStage, polls int) {
		calls = append(calls, struct {
			stage PullStage
			polls int
		}{stage, polls})
	}

	p.notifyStage(StageCheckLocal, 0)
	p.notifyStage(StageCheckRegistry, 0)
	p.notifyStage(StageWaitWorkflow, 10)

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}
	if calls[0].stage != StageCheckLocal || calls[0].polls != 0 {
		t.Errorf("call[0] = {%d, %d}, want {%d, 0}", calls[0].stage, calls[0].polls, StageCheckLocal)
	}
	if calls[1].stage != StageCheckRegistry || calls[1].polls != 0 {
		t.Errorf("call[1] = {%d, %d}, want {%d, 0}", calls[1].stage, calls[1].polls, StageCheckRegistry)
	}
	if calls[2].stage != StageWaitWorkflow || calls[2].polls != 10 {
		t.Errorf("call[2] = {%d, %d}, want {%d, 10}", calls[2].stage, calls[2].polls, StageWaitWorkflow)
	}
}

// --- Config struct tests ---

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		GithubOwner:      "owner",
		GithubRepo:       "repo",
		GithubToken:      "token",
		GithubWorkflowID: "wf.yaml",
		RegistryHost:     "host",
		RegistryUsername:  "user",
		RegistryPassword:  "pass",
		RegistryNamespace: "ns",
		RegistryArch:     "arm64",
		RegistryOs:       "linux",
	}

	if cfg.GithubOwner != "owner" {
		t.Error("GithubOwner mismatch")
	}
	if cfg.GithubRepo != "repo" {
		t.Error("GithubRepo mismatch")
	}
	if cfg.GithubToken != "token" {
		t.Error("GithubToken mismatch")
	}
	if cfg.GithubWorkflowID != "wf.yaml" {
		t.Error("GithubWorkflowID mismatch")
	}
	if cfg.RegistryHost != "host" {
		t.Error("RegistryHost mismatch")
	}
	if cfg.RegistryUsername != "user" {
		t.Error("RegistryUsername mismatch")
	}
	if cfg.RegistryPassword != "pass" {
		t.Error("RegistryPassword mismatch")
	}
	if cfg.RegistryNamespace != "ns" {
		t.Error("RegistryNamespace mismatch")
	}
	if cfg.RegistryArch != "arm64" {
		t.Error("RegistryArch mismatch")
	}
	if cfg.RegistryOs != "linux" {
		t.Error("RegistryOs mismatch")
	}
}

// --- Security tests ---

func TestIsValidImageName_ValidInputs(t *testing.T) {
	validNames := []string{
		"nginx",
		"nginx:latest",
		"nginx:1.21",
		"my.registry.com/image:tag",
		"namespace/image:tag",
		"namespace/subnamespace/image:tag",
		"image@sha256:abc123def456",
		"my_image_v1.0:latest",
		"my.repo.com:5000/my-image:v1.0",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if !isValidImageName(name) {
				t.Errorf("Expected '%s' to be valid, but it was rejected", name)
			}
		})
	}
}

func TestIsValidImageName_InvalidInputs(t *testing.T) {
	invalidNames := []string{
		"nginx;rm -rf /",         // semicolon injection
		"nginx && echo hi",       // AND injection
		"nginx || exit 1",        // OR injection
		"nginx `whoami`",         // command substitution
		"$(malicious_command)",   // command substitution
		"image\" malicious",       // quote injection
		"image' malicious",       // single quote injection
		"image\\ malicious",      // backslash in unexpected place
		"image\nmalicious",       // newline injection
		"image\r\nmalicious",     // CRLF injection
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			if isValidImageName(name) {
				t.Errorf("Expected '%s' to be invalid, but it was accepted", name)
			}
		})
	}
}
