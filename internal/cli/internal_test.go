package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- maskToken tests ---

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "****"},
		{"12345678", "****"},
		{"1234567890abcdef", "1234****cdef"},
		{"ghp_1234567890abcdefghij", "ghp_****ghij"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskToken(tt.input)
			if got != tt.expected {
				t.Errorf("maskToken(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- WriteConfigFile tests ---

func TestWriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")

	data := &ConfigData{
		GitHubOwner:       "test-owner",
		GitHubRepo:        "test-repo",
		GitHubToken:       "test-token",
		GitHubWorkflowID:  "workflow.yaml",
		RegistryHost:      "registry.example.com",
		RegistryUsername:   "test-user",
		RegistryPassword:   "test-pass",
		RegistryNamespace:  "test-ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	err := WriteConfigFile(data, filePath)
	if err != nil {
		t.Fatalf("WriteConfigFile failed: %v", err)
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Verify key values are present
	checks := []string{
		"test-owner",
		"test-repo",
		"test-token",
		"workflow.yaml",
		"registry.example.com",
		"test-user",
		"test-pass",
		"test-ns",
		"amd64",
		"linux",
	}

	for _, check := range checks {
		if !strings.Contains(contentStr, check) {
			t.Errorf("expected config to contain '%s'", check)
		}
	}
}

// --- validateGitHubToken tests ---

func TestValidateGitHubToken_EmptyOwnerRepo(t *testing.T) {
	ctx := context.Background()
	err := validateGitHubToken(ctx, "", "", "token")
	if err == nil {
		t.Error("expected error for empty owner/repo")
	}
}

func TestValidateGitHubToken_EmptyOwner(t *testing.T) {
	ctx := context.Background()
	err := validateGitHubToken(ctx, "owner", "", "token")
	if err == nil {
		t.Error("expected error for empty repo")
	}
}

// --- promptString tests (using a reader) ---
// promptString requires stdin, so we test it indirectly via WriteConfigFile

// --- confirmDeletion can't be easily tested (reads stdin + mousetrap) ---
// --- RunWizard can't be easily tested (reads stdin) ---

// --- readSyncManifest tests ---

func TestReadSyncManifest(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.yaml")

	content := `images:
  - source: ghcr.io/tektoncd/pipeline/controller:v1.1.0
    platforms: [linux/amd64, linux/arm64]
  - source: ghcr.io/nginx/nginx-gateway-fabric:2.0.1
  - source: docker.io/library/nginx:1.25
    platforms: [linux/amd64]
`
	os.WriteFile(filePath, []byte(content), 0644)

	tasks, err := readSyncManifest(filePath, "amd64", "linux")
	if err != nil {
		t.Fatalf("readSyncManifest failed: %v", err)
	}

	// Expected: controller*2 + nginx-gateway*1 + nginx*1 = 4 tasks
	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d: %v", len(tasks), tasks)
	}

	// First image: two platforms
	if tasks[0].Source != "ghcr.io/tektoncd/pipeline/controller:v1.1.0" || tasks[0].Arch != "amd64" || tasks[0].Os != "linux" {
		t.Errorf("task[0] = %+v, unexpected", tasks[0])
	}
	if tasks[1].Source != "ghcr.io/tektoncd/pipeline/controller:v1.1.0" || tasks[1].Arch != "arm64" || tasks[1].Os != "linux" {
		t.Errorf("task[1] = %+v, unexpected", tasks[1])
	}

	// Second image: default platform
	if tasks[2].Source != "ghcr.io/nginx/nginx-gateway-fabric:2.0.1" || tasks[2].Arch != "amd64" || tasks[2].Os != "linux" {
		t.Errorf("task[2] = %+v, unexpected (should use default platform)", tasks[2])
	}

	// Third image: explicit single platform
	if tasks[3].Source != "docker.io/library/nginx:1.25" || tasks[3].Arch != "amd64" || tasks[3].Os != "linux" {
		t.Errorf("task[3] = %+v, unexpected", tasks[3])
	}
}

func TestReadSyncManifest_DefaultPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.yaml")

	content := `images:
  - source: nginx:latest
`
	os.WriteFile(filePath, []byte(content), 0644)

	tasks, err := readSyncManifest(filePath, "arm64", "linux")
	if err != nil {
		t.Fatalf("readSyncManifest failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Arch != "arm64" || tasks[0].Os != "linux" {
		t.Errorf("expected default platform linux/arm64, got %s/%s", tasks[0].Os, tasks[0].Arch)
	}
}

func TestReadSyncManifest_EmptyImages(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.yaml")

	content := `images: []
`
	os.WriteFile(filePath, []byte(content), 0644)

	tasks, err := readSyncManifest(filePath, "amd64", "linux")
	if err != nil {
		t.Fatalf("readSyncManifest failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestReadSyncManifest_InvalidPlatformFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.yaml")

	content := `images:
  - source: nginx:latest
    platforms: [amd64]
`
	os.WriteFile(filePath, []byte(content), 0644)

	_, err := readSyncManifest(filePath, "amd64", "linux")
	if err == nil {
		t.Error("expected error for invalid platform format")
	}
	if !strings.Contains(err.Error(), "invalid platform format") {
		t.Errorf("expected 'invalid platform format' error, got: %v", err)
	}
}

func TestReadSyncManifest_Nonexistent(t *testing.T) {
	_, err := readSyncManifest("/nonexistent/manifest.yaml", "amd64", "linux")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadSyncManifest_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.yaml")

	content := `this is not valid yaml: [[[`
	os.WriteFile(filePath, []byte(content), 0644)

	_, err := readSyncManifest(filePath, "amd64", "linux")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSyncTask_DisplayName(t *testing.T) {
	task := syncTask{
		Source: "nginx:latest",
		Arch:   "amd64",
		Os:     "linux",
	}
	expected := "nginx:latest (linux/amd64)"
	if got := task.displayName(); got != expected {
		t.Errorf("displayName() = %q, want %q", got, expected)
	}
}

// --- ConfigData struct test ---

func TestConfigData_Fields(t *testing.T) {
	data := &ConfigData{
		GitHubOwner:       "o",
		GitHubRepo:        "r",
		GitHubToken:       "t",
		GitHubWorkflowID:  "w",
		RegistryHost:      "h",
		RegistryUsername:   "u",
		RegistryPassword:   "p",
		RegistryNamespace:  "n",
		RegistryArch:      "a",
		RegistryOs:        "os",
	}

	if data.GitHubOwner != "o" {
		t.Error("GitHubOwner mismatch")
	}
	if data.GitHubRepo != "r" {
		t.Error("GitHubRepo mismatch")
	}
	if data.GitHubToken != "t" {
		t.Error("GitHubToken mismatch")
	}
	if data.GitHubWorkflowID != "w" {
		t.Error("GitHubWorkflowID mismatch")
	}
	if data.RegistryHost != "h" {
		t.Error("RegistryHost mismatch")
	}
	if data.RegistryUsername != "u" {
		t.Error("RegistryUsername mismatch")
	}
	if data.RegistryPassword != "p" {
		t.Error("RegistryPassword mismatch")
	}
	if data.RegistryNamespace != "n" {
		t.Error("RegistryNamespace mismatch")
	}
	if data.RegistryArch != "a" {
		t.Error("RegistryArch mismatch")
	}
	if data.RegistryOs != "os" {
		t.Error("RegistryOs mismatch")
	}
}