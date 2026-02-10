package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- readImagesFromFile tests ---

func TestReadImagesFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "images.txt")

	content := `nginx:latest
# this is a comment
redis:alpine

postgres:15
# another comment
ubuntu:22.04
`
	os.WriteFile(filePath, []byte(content), 0644)

	images, err := readImagesFromFile(filePath)
	if err != nil {
		t.Fatalf("readImagesFromFile failed: %v", err)
	}

	expected := []string{"nginx:latest", "redis:alpine", "postgres:15", "ubuntu:22.04"}
	if len(images) != len(expected) {
		t.Fatalf("expected %d images, got %d: %v", len(expected), len(images), images)
	}

	for i, img := range images {
		if img != expected[i] {
			t.Errorf("expected images[%d] = '%s', got '%s'", i, expected[i], img)
		}
	}
}

func TestReadImagesFromFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(filePath, []byte(""), 0644)

	images, err := readImagesFromFile(filePath)
	if err != nil {
		t.Fatalf("readImagesFromFile failed: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestReadImagesFromFile_OnlyComments(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "comments.txt")
	content := `# comment 1
# comment 2
# comment 3
`
	os.WriteFile(filePath, []byte(content), 0644)

	images, err := readImagesFromFile(filePath)
	if err != nil {
		t.Fatalf("readImagesFromFile failed: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestReadImagesFromFile_Nonexistent(t *testing.T) {
	_, err := readImagesFromFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

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
