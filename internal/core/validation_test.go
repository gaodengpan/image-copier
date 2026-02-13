package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageValidator_ValidateImageNameInput(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple image", "nginx", true},
		{"valid image with tag", "nginx:latest", true},
		{"valid fully qualified image", "docker.io/library/nginx:latest", true},
		{"valid image with digest", "nginx@sha256:abc123", true},
		{"image with newlines", "nginx\nrm -rf /", false},
		{"image with command injection", "nginx;rm -rf /", false},
		{"image with pipe", "nginx | whoami", false},
		{"image with backtick", "nginx `whoami`", false},
		{"path traversal attempt", "../../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateImageNameInput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_ValidateCredentials(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"valid credentials", "user", "pass", true},
		{"empty username", "", "pass", false},
		{"empty password", "user", "", false},
		{"username with newlines", "user\n", "pass", false},
		{"password with command injection", "user", "pass;rm -rf /", false},
		{"username with special chars", "user;malicious", "pass", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateCredentials(tt.username, tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_ValidateFilePath(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"valid path", "/tmp/file.txt", true},
		{"path traversal attempt", "../../../etc/passwd", false},
		{"path with backslashes", "..\\..\\..\\windows\\system32", false},
		{"valid temp path", "/tmp/some-file", true},
		{"path with null byte", "/tmp/file\x00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateFilePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_ValidateYAMLContent(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"safe YAML", "key: value", true},
		{"YAML with shell command", "{{ shell \"rm -rf /\" }}", false},
		{"YAML with pipe", "cmd | sh", false}, // This should fail as it contains a dangerous pattern
		{"YAML with eval", "eval malicious_code", false}, // This should fail as it contains eval
		{"YAML with eval in template", "{{ eval \"malicious_code\" }}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateYAMLContent(tt.content)
			if tt.expected {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}