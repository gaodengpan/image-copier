package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestValidateImageNameInputEdgeCases tests various edge cases for image name validation
func TestValidateImageNameInputEdgeCases(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Boundary cases for lengths
		{"empty string", "", false},
		{"single character", "a", true},
		{"two characters", "ab", true},
		{"very long valid name", strings.Repeat("a", 1000) + "/b:latest", true},
		{"very long invalid name with injection", strings.Repeat("a", 100) + ";rm -rf /", false},

		// Unicode and special characters
		{"unicode in name", "测试:latest", false}, // Should fail since it's not alphanumeric
		{"special chars", "image_1.2-rc1:latest", true},
		{"name with underscore", "my_image:tag", true},
		{"name with dots", "my.image:tag", true},
		{"name with dashes", "my-image:tag", true},

		// Path traversal variations
		{"traversal with dots", ".../etc/passwd", false},
		{"traversal with encoded dots", "%2E%2E%2Fetc%2Fpasswd", true}, // Should pass as not directly matching ../
		{"mixed slash traversal", "/../", false},
		{"backslash traversal", "..\\", false},
		{"reverse traversal", "/..", false},

		// Command injection variations
		{"semicolon injection", "nginx;ls -la", false},
		{"ampersand injection", "nginx&rm -rf /", false},
		{"double ampersand injection", "nginx&&rm -rf /", false},
		{"pipe injection", "nginx|rm -rf /", false},
		{"double pipe injection", "nginx||rm -rf /", false},
		{"backtick injection", "nginx`rm -rf /`", false},
		{"subshell injection", "nginx$(rm -rf /)", false},
		{"dollar paren injection", "nginx${rm -rf /}", false},

		// Newline and carriage return
		{"newline injection", "nginx\nrm -rf /", false},
		{"carriage return injection", "nginx\rwhoami", false},
		{"crlf injection", "nginx\r\nps aux", false},

		// Multiple separators
		{"multiple colons", "repo:tag:suffix", true}, // Could be valid depending on implementation
		{"multiple at symbols", "image@sha256:hash@another", false},
		{"mixed separators", "repo:tag@digest", true},

		// Edge cases with digests
		{"valid sha256 digest", "nginx@sha256:abc123def4567890123456789012345678901234567890123456789012345678", true},
		{"invalid sha256 digest", "nginx@sha256:invalid_digest_chars!", false},
		{"short algorithm", "a:hash", false},
		{"digit algorithm", "123:hash", false},
		{"proper algorithm", "sha256:abc123def4567890123456789012345678901234567890123456789012345678", true},

		// Image tag edge cases
		{"image without tag", "nginx", true},
		{"just imagetag (should fail)", "imagetag", false},
		{"camel case that looks like imagetag", "imageTag", false},
		{"snake case that looks like imagetag", "image_tag", true},
		{"common image names", "alpine", true},
		{"common image names with tag", "alpine:latest", true},
		{"non-common name with no separator", "notacommonimagename", true}, // Should pass since it's not recognized as a mistyped version

		// Complex paths
		{"complex path", "registry.example.com/namespace/subnamespace/image:tag", true},
		{"IP address as registry", "192.168.1.100:5000/myimage:latest", true},
		{"port in registry", "registry.com:8080/image:tag", true},
		{"subdomain registry", "docker.registry.com/namespace/image:tag", true},

		{"null byte injection", "nginx\x00", true}, // Null byte check happens in file path validation, not general validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateImageNameInput(tt.input)
			assert.Equal(t, tt.expected, result, "Test %s failed: expected %v for input '%s'", tt.name, tt.expected, tt.input)
		})
	}
}

// TestValidateCredentialsEdgeCases tests edge cases for credential validation
func TestValidateCredentialsEdgeCases(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"empty username", "", "password", false},
		{"empty password", "username", "", false},
		{"both empty", "", "", false},
		{"max length username", strings.Repeat("a", 1000), "pass", true},
		{"max length password", "user", strings.Repeat("b", 1000), true},
		{"over max length username", strings.Repeat("a", 1001), "pass", false},
		{"over max length password", "user", strings.Repeat("b", 1001), false},
		{"both max length", strings.Repeat("a", 1000), strings.Repeat("b", 1000), true},
		{"newline in username", "user\nadmin", "pass", false},
		{"newline in password", "user", "pass\nsecret", false},
		{"command injection in username", "user;rm", "pass", false},
		{"command injection in password", "user", "pass;rm", false},
		{"special chars in username", "user$test", "pass", false},
		{"special chars in password", "user", "pass`inject", false},
		{"spaces in username", "user name", "pass", true}, // Space is not in dangerous chars
		{"spaces in password", "user", "pass word", true}, // Space is not in dangerous chars
		{"common username formats", "my-user_name.test@example.com", "pass123!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateCredentials(tt.username, tt.password)
			assert.Equal(t, tt.expected, result, "Test %s failed: expected %v for username '%s' and password '%s'", tt.name, tt.expected, tt.username, tt.password)
		})
	}
}

// TestValidateFilePathEdgeCases tests edge cases for file path validation
func TestValidateFilePathEdgeCases(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"empty path", "", true}, // Empty path doesn't contain traversal sequences
		{"simple path", "file.txt", true},
		{"relative path", "folder/file.txt", true},
		{"absolute path in allowed location", "/tmp/file.txt", true},
		{"absolute path in allowed location deep", "/tmp/folder/deep/file.txt", true},
		{"absolute path in var tmp", "/var/tmp/file.txt", true},
		{"path traversal forward slashes", "../../../etc/passwd", false},
		{"path traversal backslashes", "..\\..\\..\\windows\\system32", false},
		{"path traversal mixed slashes", "/../../", false},
		{"path traversal reverse", "/..", false},
		{"path traversal at start", "../file", false},
		{"path traversal at end", "file/../", false},
		{"null byte in path", "/tmp/file\x00", false},
		{"non-allowed absolute path", "/root/config.txt", false},
		{"non-allowed absolute path with allowed prefix substring", "/var/tmp.txt", false}, // Does not start with /var/tmp/
		{"allowed absolute path", "/var/tmp/", true},
		{"long path", "/tmp/" + strings.Repeat("a", 1000), true},
		{"long traversal path", "/tmp/" + strings.Repeat("../", 100) + "etc/passwd", false},
		{"encoded traversal", "/tmp/..%2Fetc%2Fpasswd", true}, // Should pass as not directly matching traversal
		{"dot dot file", "file..txt", true},                   // Not a traversal
		{"double dot at end", "backup..", true},               // Not a traversal
		{"current directory reference", "./file.txt", true},   // Not a traversal
		{"multiple dots", "file...txt", true},                 // Not a traversal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateFilePath(tt.path)
			assert.Equal(t, tt.expected, result, "Test %s failed: expected %v for path '%s'", tt.name, tt.expected, tt.path)
		})
	}
}

// TestValidateYAMLContentEdgeCases tests edge cases for YAML content validation
func TestValidateYAMLContentEdgeCases(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name     string
		content  string
		expected bool // true means no error (safe content), false means error (unsafe content)
	}{
		{"empty content", "", true},
		{"simple key value", "key: value", true},
		{"multiline YAML", "key:\n  subkey: value\n  another: thing", true},
		{"list in YAML", "list:\n  - item1\n  - item2\n  - item3", true},
		{"nested structure", "parent:\n  child:\n    grandchild: value", true},
		{"template tags", "{{ .Values.key }}", true}, // Safe template
		{"template with shell danger", "{{ .Values.key | shell }}", false},
		{"exec in template", "{{ exec \"command\" }}", false},
		{"eval in template", "{{ eval \"dangerous code\" }}", false},
		{"command in content", "command: ls -la", true}, // Just a key name, not actual command
		{"shell command pattern", "cmd | sh", false},
		{"bash command pattern", "script.sh | bash", false},
		{"zsh command pattern", "script.zsh | zsh", false},
		{"double ampersand", "cmd1 && cmd2", false},
		{"pipe operator", "ls -la | grep test", false},
		{"logical OR", "cmd1 || cmd2", false},
		{"semicolon separator", "cmd1; cmd2", false},
		{"backtick execution", "`whoami`", false},
		{"dollar paren execution", "$(whoami)", false},
		{"eval keyword", "eval dangerous_code", false},
		{"exec keyword", "exec /bin/bash", false},
		{"command keyword", "command -v bash", false},
		{"case insensitive danger", "EVAL malicious", false},
		{"lowercase danger", "exec /bin/sh", false},
		{"mixed case danger", "ExEc /bin/sh", false},
		{"whitespace around danger", "  exec  /bin/sh  ", false},
		{"comments with danger", "# This might execute: exec /bin/sh", true}, // Comments are not parsed as commands
		{"quotes around danger", "\"exec /bin/sh\"", true},                  // Inside quotes should be safe
		{"complex template danger", "{{ if eq .Values.enabled \"true\" }} {{ exec \"rm -rf /\" }} {{ end }}", false},
		{"large safe content", strings.Repeat("key: value\n", 1000), true},
		{"large dangerous content", strings.Repeat("key: value\n", 500) + "cmd | sh\n" + strings.Repeat("other: value\n", 500), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateYAMLContent(tt.content)
			if tt.expected {
				assert.NoError(t, err, "Test %s failed: expected no error for content '%s'", tt.name, tt.content[:min(len(tt.content), 100)])
			} else {
				assert.Error(t, err, "Test %s failed: expected error for content '%s'", tt.name, tt.content[:min(len(tt.content), 100)])
			}
		})
	}
}

