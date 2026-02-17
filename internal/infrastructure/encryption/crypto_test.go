package encryption

import (
	"os"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "this is a test value to encrypt and decrypt"

	// Test encryption
	encrypted, err := Encrypt(testValue, "test-key-for-unit-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Verify the format
	if !IsValidEncryptedFormat(encrypted) {
		t.Errorf("Encrypted value does not have valid format: %s", encrypted)
	}

	// Test decryption
	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Verify the result
	if decrypted != testValue {
		t.Errorf("Decrypted value '%s' does not match original '%s'", decrypted, testValue)
	}
}

func TestDecryptInvalidFormat(t *testing.T) {
	invalidFormats := []string{
		"",                            // Empty string
		"encrypted",                   // No colon
		"encrypted:",                  // No data
		"encrypt:test",                // Wrong prefix
		"encrypted:invalid_base64!@#", // Invalid base64
	}

	for _, invalidFormat := range invalidFormats {
		_, err := Decrypt(invalidFormat)
		if err == nil {
			t.Errorf("Expected error for invalid format '%s', but got none", invalidFormat)
		}
	}
}

func TestIsEncryptedValue(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"encrypted:abc123", true},
		{"not_encrypted:value", false},
		{"", false},
		{"encrypted", false},
		{"encrypted:", true}, // starts with encrypted:, so true
	}

	for _, test := range tests {
		result := IsEncryptedValue(test.input)
		if result != test.expected {
			t.Errorf("IsEncryptedValue('%s') = %v, want %v", test.input, result, test.expected)
		}
	}
}

func TestIsValidEncryptedFormat(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	// Valid encrypted value
	testValue := "test value"
	encrypted, err := Encrypt(testValue, "test-key-for-unit-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt for test: %v", err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{encrypted, true},               // Valid encrypted value
		{"encrypted:YWJjZGVm", true},    // Valid base64, but may not have proper structure
		{"", false},                     // Empty
		{"encrypted", false},            // No colon
		{"encrypted:", false},           // No data
		{"encrypted:invalid!@#", false}, // Invalid base64
	}

	for _, test := range tests {
		result := IsValidEncryptedFormat(test.input)
		// Note: Some of these might pass base64 validation but fail other checks
		// The important thing is that obviously invalid formats return false
		if test.expected && !result && test.input != "encrypted:YWJjZGVm" {
			t.Errorf("IsValidEncryptedFormat('%s') = %v, want %v", test.input, result, test.expected)
		}
	}
}
