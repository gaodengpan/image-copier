package sanitizer

import (
	"strings"
	"testing"
)

func TestErrorOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "filter SKOPEO_SRC_USER",
			input:    "error: SKOPEO_SRC_USER=admin failed",
			expected: "error: SKOPEO_SRC_USER=*** failed",
		},
		{
			name:     "filter SKOPEO_SRC_PASS",
			input:    "error: SKOPEO_SRC_PASS=secret123 failed",
			expected: "error: SKOPEO_SRC_PASS=*** failed",
		},
		{
			name:     "filter URL credentials",
			input:    "connecting to user:pass@registry.example.com",
			expected: "connecting to ***:***@registry.example.com",
		},
		{
			name:     "filter Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer ***",
		},
		{
			name:     "filter Basic auth",
			input:    "Authorization: Basic dXNlcjpwYXNz",
			expected: "Authorization: Basic ***",
		},
		{
			name:     "no sensitive info",
			input:    "image not found: nginx:latest",
			expected: "image not found: nginx:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrorOutput(tt.input)
			if result != tt.expected {
				t.Errorf("ErrorOutput() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short output unchanged",
			input:    "short",
			maxLen:   100,
			expected: "short",
		},
		{
			name:     "long output truncated",
			input:    strings.Repeat("a", 200),
			maxLen:   100,
			expected: strings.Repeat("a", 100) + "... (truncated)",
		},
		{
			name:     "exact length unchanged",
			input:    "exact",
			maxLen:   5,
			expected: "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateOutput(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("TruncateOutput() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	// Test combined sanitization
	input := "SKOPEO_SRC_PASS=secret failed with very long error message..."
	result := SanitizeError(input, 50)

	// Should filter credentials
	if strings.Contains(result, "secret") {
		t.Error("SanitizeError should filter credentials")
	}

	// Should be truncated (maxLen + "... (truncated)" suffix)
	if len(result) > 65 {
		t.Errorf("SanitizeError should truncate, got length %d", len(result))
	}
}
