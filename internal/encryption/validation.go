package encryption

import (
	"encoding/base64"
	"strings"
)

// IsValidEncryptedFormat checks if a string is in the correct encrypted format
// The expected format is "encrypted:<base64_encoded_data>"
func IsValidEncryptedFormat(value string) bool {
	if value == "" {
		return false
	}

	// Check if the value starts with "encrypted:"
	if !strings.HasPrefix(value, "encrypted:") {
		return false
	}

	// Extract the base64 encoded part
	encodedPart := value[10:] // Skip "encrypted:" prefix

	// Check if there's content after the prefix
	if encodedPart == "" {
		return false
	}

	// Try to decode the base64 part to verify it's valid
	_, err := base64.StdEncoding.DecodeString(encodedPart)
	return err == nil
}

// IsEncryptedValue checks if a value is encrypted (starts with encrypted: prefix)
func IsEncryptedValue(value string) bool {
	return strings.HasPrefix(value, "encrypted:")
}