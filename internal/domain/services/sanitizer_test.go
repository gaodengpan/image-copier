package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectPrefix  bool
		expectSameLen bool
	}{
		{"simple string", "nginx", true, true},
		{"complex string", "my-registry.com/namespace/image:tag", true, true},
		{"empty string", "", true, true},
		{"long string", "very-long-image-name-that-exceeds-normal-length", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			assert.True(t, len(result) > 0)
			assert.Contains(t, result, sensitiveDataPrefix)
			assert.Contains(t, result, sensitiveDataSuffix)
		})
	}
}

func TestSanitizer_SanitizeForLog(t *testing.T) {
	s := NewSanitizer()
	result := s.SanitizeForLog("test-input")
	assert.Contains(t, result, sensitiveDataPrefix)
	assert.Contains(t, result, sensitiveDataSuffix)
}

func TestSanitizeForLog_Collisions(t *testing.T) {
	result1 := SanitizeForLog("password123")
	result2 := SanitizeForLog("password123")
	assert.Equal(t, result1, result2, "same input should produce same hash")

	result3 := SanitizeForLog("different")
	assert.NotEqual(t, result1, result3, "different input should produce different hash")
}

func TestSanitizeForLog_Format(t *testing.T) {
	result := SanitizeForLog("test")
	assert.Equal(t, len(result), 27, "prefix(10) + 8 hex + suffix(1) = 19, but sha256 hash is 8 bytes = 16 hex chars")
}
