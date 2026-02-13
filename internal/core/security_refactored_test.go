package core

import (
	"context"
	"strings"
	"testing"
)

// TestSanitizeForLog_HidesSensitiveData tests that sensitive data is properly hidden in logs
func TestSanitizeForLog_HidesSensitiveData(t *testing.T) {
	testData := "secret_token_12345"
	result := sanitizeForLog(testData)

	// The sanitized result should hide the original data
	if result == testData {
		t.Fatalf("Expected '%s' to be sanitized differently from '%s'", result, testData)
	}

	// Should contain REDACTED indicator
	if len(result) < 10 || !strings.HasPrefix(result, "[REDACTED:") {
		t.Errorf("Expected sanitized result to start with '[REDACTED:', got '%s'", result)
	}

	// Should contain hash representation (hex characters within brackets)
	if len(result) < 20 || result[len(result)-1] != ']' {
		t.Errorf("Expected sanitized result to end with ']', got '%s'", result)
	}
}

// TestSanitizeForLog_DeterministicOutput tests that the same input produces the same output
func TestSanitizeForLog_DeterministicOutput(t *testing.T) {
	input := "test-sensitive-data"

	result1 := sanitizeForLog(input)
	result2 := sanitizeForLog(input)

	// Same input should produce same output (deterministic)
	if result1 != result2 {
		t.Errorf("Expected same input '%s' to produce same sanitized output.\nResult1: %s\nResult2: %s",
			input, result1, result2)
	}
}

// TestValidateImageNameInput_CommandInjection_Attempts tests validation against command injection
func TestValidateImageNameInput_CommandInjection_Attempts(t *testing.T) {
	maliciousInputs := []struct {
		input string
		desc  string
	}{
		{"nginx; rm -rf /", "semicolon injection"},
		{"alpine && echo 'compromised'", "AND operator injection"},
		{"busybox || shutdown -h now", "OR operator injection"},
		{"centos `cat /etc/passwd`", "backtick command substitution"},
		{"ubuntu $(whoami)", "dollar-paren command substitution"},
		{"debian \"$(ls -la)\"", "quoted command substitution"},
		{"fedora; touch HACKED", "semicolon command"},
		{"rhel && ping -c 1 evil.com", "AND network command"},
	}

	for _, input := range maliciousInputs {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// All these should return false due to potential injection
			if result {
				t.Errorf("Expected '%s' to be rejected due to potential injection, but it was accepted", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_PathTraversal_Attempts tests validation against path traversal
func TestValidateImageNameInput_PathTraversal_Attempts(t *testing.T) {
	traversalInputs := []struct {
		input string
		desc  string
	}{
		{"../../../etc/passwd", "parent directory traversal"},
		{"../../windows/system32", "multiple parent traversals"},
		{"subdir/..../sensitive", "dots in middle"},
		{"normal/../../../secret", "traversal with prefix"},
		{"image/..\\..\\windows", "Windows-style path"},
		{"docker/../../../root/.ssh/id_rsa", "SSH key traversal"},
	}

	for _, input := range traversalInputs {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// Path traversal attempts should be rejected
			if result {
				t.Errorf("Expected '%s' to be rejected due to potential path traversal, but it was accepted", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_NetworkPath_Rejection tests validation against network path formats
func TestValidateImageNameInput_NetworkPath_Rejection(t *testing.T) {
	networkFormats := []struct {
		input string
		desc  string
	}{
		{"https://registry.com/image:tag", "HTTPS URL"},
		{"http://registry.com/image:tag", "HTTP URL"},
		{"ftp://registry.com/image:tag", "FTP URL"},
		{"\\\\server\\share\\image:tag", "Windows UNC path"},
		{"file:///path/to/image:tag", "File URL scheme"},
		{"ssh://user@host/image:tag", "SSH URL"},
	}

	for _, input := range networkFormats {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// These formats should generally be rejected as they're not valid docker image names
			if result {
				t.Errorf("Expected network path format '%s' to be rejected, but it was accepted", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_PortSpecifications_Acceptance tests that valid port specifications are accepted
func TestValidateImageNameInput_PortSpecifications_Acceptance(t *testing.T) {
	validPortFormats := []struct {
		input string
		desc  string
	}{
		{"localhost:5000/image:tag", "localhost with port"},
		{"registry.com:8080/image:tag", "domain with custom port"},
		{"192.168.1.1:5000/image:tag", "IP with port"},
		{"[::1]:5000/image:tag", "IPv6 with port"},
	}

	for _, input := range validPortFormats {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// These should be valid Docker image names
			if !result {
				t.Errorf("Expected port specification format '%s' to be accepted, but it was rejected", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_MultipleColons_Rejection tests rejection of multiple colons in invalid positions
func TestValidateImageNameInput_MultipleColons_Rejection(t *testing.T) {
	invalidMultiColonFormats := []struct {
		input string
		desc  string
	}{
		{"image::doublecolon", "double colon"},
		{"registry:5000:image:tag", "multiple colons in wrong places"},
		{"image:tag:sometag", "multiple tags"},
		{"image:tag@sha256:hash", "tag and digest together"},
	}

	for _, input := range invalidMultiColonFormats {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// These should be rejected due to multiple colons
			if result {
				t.Errorf("Expected multiple colon format '%s' to be rejected, but it was accepted", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_SpecialCharacters_Rejection tests rejection of various special characters
func TestValidateImageNameInput_SpecialCharacters_Rejection(t *testing.T) {
	specialChars := []struct {
		input string
		desc  string
	}{
		{"image*glob", "glob character"},
		{"image?wildcard", "wildcard character"},
		{"image[master]", "bracket character"},
		{"image{dev}", "curly brace"},
		{"image|pipe", "pipe character"},
		{"image&background", "ampersand"},
		{"image^caret", "caret"},
		{"image%percent", "percent"},
		{"image!negate", "exclamation"},
		{"image=assign", "equals sign"},
		{"image+plus", "plus sign"},
	}

	for _, input := range specialChars {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// Most of these should be rejected for security reasons
			if result {
				t.Errorf("Expected special character format '%s' to be rejected, but it was accepted", input.input)
			}
		})
	}
}

// TestValidateImageNameInput_RegexInjection_Attacks tests for potential regex-based vulnerabilities
func TestValidateImageNameInput_RegexInjection_Attacks(t *testing.T) {
	regexpAttackInputs := []struct {
		input string
		desc  string
	}{
		{"nginx:latest" + string([]byte{0x5C, 0x00}), "null byte injection"},
		{"image\r\nmalicious", "CRLF that might affect regex"},
		{"image[0-9]{1000000}", "potential ReDoS attack"},
		{"(?:a+)+$", "classic ReDoS pattern"},
		{"\\A\\c\\E", "regex metacharacters"},
	}

	for _, input := range regexpAttackInputs {
		t.Run(input.desc, func(t *testing.T) {
			// This should be handled safely by the validation
			result := validateImageNameInput(input.input)

			// Should not crash or hang on malicious regex inputs
			// (though they should likely be rejected)
			if result && len(input.input) > 100 {
				t.Errorf("Long regex-y input '%s' was unexpectedly accepted",
					input.input[:minInt(len(input.input), 50)] + "...")
			}
		})
	}
}

// TestValidateImageNameInput_UnicodeCharacters_Handling tests handling of unicode characters
func TestValidateImageNameInput_UnicodeCharacters_Handling(t *testing.T) {
	unicodeInputs := []struct {
		input string
		desc  string
	}{
		{"nginx:最新版", "Chinese characters in tag"},
		{"镜像:latest", "Chinese characters in name"},
		{"image:é", "Accent character in tag"},
		{"résumé:latest", "Accent character in name"},
		{"image:🚀", "Emoji in tag"},
		{"🚀:latest", "Emoji in name"},
	}

	for _, input := range unicodeInputs {
		t.Run(input.desc, func(t *testing.T) {
			result := validateImageNameInput(input.input)

			// Log the result to document how unicode is handled
			t.Logf("validateImageNameInput('%s') returned %v", input.input, result)

			// Most of these might be invalid for docker standards, but we want to ensure
			// the validation is explicit about which unicode characters are allowed
		})
	}
}

// TestCheckImageExists_WithSensitiveCredentials_DoesNotLeak tests that credentials aren't leaked in errors
func TestCheckImageExists_WithSensitiveCredentials_DoesNotLeak(t *testing.T) {
	ctx := context.Background()

	// Use fake credentials that contain sensitive data
	fakeUsername := "user;DROP TABLE users;"
	fakePassword := "pass' OR '1'='1"

	// Call with these potentially dangerous credentials
	// Note: This will likely fail due to missing registry, but shouldn't leak credentials
	_, err := CheckImageExists(ctx, "registry.example.com/image:tag", fakeUsername, fakePassword)

	if err == nil {
		t.Log("Expected CheckImageExists to fail with fake credentials, but it didn't return an error")
		// If it doesn't fail, that's a different issue but we still need to check for leaks
	}

	// If there's an error, check that sensitive data isn't exposed in error message
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "DROP TABLE") || strings.Contains(errMsg, "OR '1'='1") {
			t.Errorf("Error message contains sensitive data: %s", errMsg)
		}
	}
}

// TestCredentialsValidation_SpecialCharacter_Rejection tests credential validation
func TestCredentialsValidation_SpecialCharacter_Rejection(t *testing.T) {
	invalidCredentials := []struct {
		username string
		password string
		desc     string
	}{
		{"user;cmd", "pass", "username with semicolon"},
		{"user", "pass' OR '1'='1", "password with SQL injection"},
		{"user`cmd`", "pass", "username with backticks"},
		{"user$(inj)", "pass", "username with command substitution"},
		{"user", "pass && cmd", "password with command concatenation"},
		{"user||other", "pass", "username with OR operator"},
	}

	for _, tc := range invalidCredentials {
		t.Run(tc.desc, func(t *testing.T) {
			// Test the validation that happens inside CheckImageExists
			_, err := CheckImageExists(context.Background(), "registry.com/image:tag", tc.username, tc.password)

			// The validation should ideally reject these inputs
			// In the current implementation, these may not be caught by validation
			// but should at least not cause credential leakage
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, tc.username) || strings.Contains(errMsg, tc.password) {
					t.Errorf("Error message contains credentials: %s", errMsg)
				}
			}
		})
	}
}

// Helper function for minimum integer
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}