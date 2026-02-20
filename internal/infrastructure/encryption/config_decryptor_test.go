package encryption

import (
	"os"
	"testing"
)

func TestConfigDecryptor(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "test value to encrypt and decrypt"

	// Encrypt the value first
	encrypted, err := Encrypt(testValue, "test-key-for-unit-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Create a new decryptor
	decryptor := NewConfigDecryptor()

	// Test decryption
	decrypted, err := decryptor.DecryptValue(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt value: %v", err)
	}

	if decrypted != testValue {
		t.Errorf("Decrypted value '%s' doesn't match original '%s'", decrypted, testValue)
	}

	// Test decrypting a non-encrypted value (should return as-is)
	plainText := "this is plain text"
	result, err := decryptor.DecryptValue(plainText)
	if err != nil {
		t.Fatalf("Failed to decrypt plain text: %v", err)
	}

	if result != plainText {
		t.Errorf("Plain text value changed: got '%s', want '%s'", result, plainText)
	}
}

func TestConfigDecryptorInvalidValue(t *testing.T) {
	// Create a new decryptor
	decryptor := NewConfigDecryptor()

	// Test with non-encrypted values - these should return as-is, not error
	nonEncryptedValues := []string{
		"",             // Empty string
		"normal text",  // Regular text
		"encrypt:test", // Text starting with encrypt but not encrypted:
	}

	for _, value := range nonEncryptedValues {
		result, err := decryptor.DecryptValue(value)
		if err != nil {
			t.Errorf("Unexpected error for non-encrypted value '%s': %v", value, err)
		}
		if result != value {
			t.Errorf("Non-encrypted value changed: got '%s', want '%s'", result, value)
		}
	}

	// Now test with properly prefixed but invalid encrypted values - these should error
	invalidEncryptedFormats := []string{
		"encrypted:",                  // No data after prefix
		"encrypted:invalid_base64!@#", // Invalid base64 after prefix
	}

	for _, value := range invalidEncryptedFormats {
		_, err := decryptor.DecryptValue(value)
		if err == nil {
			t.Errorf("Expected error for invalid encrypted format '%s', but got none", value)
		}
	}
}

func TestConfigDecryptorWrongKey(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "test value to encrypt and decrypt"

	// Encrypt the value with one key
	encrypted, err := Encrypt(testValue, "test-key-for-unit-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Temporarily change the key
	os.Setenv("ENCRYPT_KEY", "different-test-key-for-unit-tests")

	// Create a new decryptor (which will use the new key)
	decryptor := NewConfigDecryptor()

	// Test decryption with wrong key - should fail
	_, err = decryptor.DecryptValue(encrypted)
	if err == nil {
		t.Error("Expected error when decrypting with wrong key, but got none")
	}
}
