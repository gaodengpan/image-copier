package encryption

import (
	"os"
	"testing"
)

func TestConfigEncryptor(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	// Create a new encryptor
	encryptor, err := NewConfigEncryptor()
	if err != nil {
		t.Fatalf("Failed to create config encryptor: %v", err)
	}

	testValue := "test value to encrypt"

	// Test encryption
	encrypted, err := encryptor.EncryptValue(testValue)
	if err != nil {
		t.Fatalf("Failed to encrypt value: %v", err)
	}

	// Verify it's properly formatted
	if !IsEncryptedValue(encrypted) {
		t.Errorf("Encrypted value doesn't have correct format: %s", encrypted)
	}

	// Test decryption to verify round trip
	decryptor := NewConfigDecryptor()
	decrypted, err := decryptor.DecryptValue(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt value: %v", err)
	}

	if decrypted != testValue {
		t.Errorf("Decrypted value '%s' doesn't match original '%s'", decrypted, testValue)
	}
}

func TestConfigEncryptorEmptyValue(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-for-unit-tests-32chars")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	// Create a new encryptor
	encryptor, err := NewConfigEncryptor()
	if err != nil {
		t.Fatalf("Failed to create config encryptor: %v", err)
	}

	// Test encryption of empty value
	_, err = encryptor.EncryptValue("")
	if err == nil {
		t.Error("Expected error when encrypting empty value, but got none")
	}

	encErr, ok := err.(*EncryptionError)
	if !ok {
		t.Errorf("Expected EncryptionError, got %T", err)
	} else if encErr.Message != "cannot encrypt empty value" {
		t.Errorf("Expected 'cannot encrypt empty value', got '%s'", encErr.Message)
	}
}

func TestIsSensitiveField(t *testing.T) {
	sensitiveFields := []string{"token", "password", "secret", "key", "access_key", "secret_key", "username"}
	nonSensitiveFields := []string{"host", "owner", "repo", "workflow_id", "namespace", "arch", "os", "log_level"}

	for _, field := range sensitiveFields {
		if !IsSensitiveField(field) {
			t.Errorf("Expected '%s' to be sensitive, but it wasn't", field)
		}
	}

	for _, field := range nonSensitiveFields {
		if IsSensitiveField(field) {
			t.Errorf("Expected '%s' to not be sensitive, but it was", field)
		}
	}
}
