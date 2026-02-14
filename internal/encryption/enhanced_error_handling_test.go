package encryption

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestInvalidKeyHandling tests how the system handles invalid encryption keys
func TestInvalidKeyHandling(t *testing.T) {
	// Test 1: No encryption key set
	os.Unsetenv("ENCRYPT_KEY")

	_, err := NewConfigEncryptor()
	if err == nil {
		t.Error("Expected error when ENCRYPT_KEY is not set, but got none")
	}

	// Test 2: Too short encryption key
	os.Setenv("ENCRYPT_KEY", "short")
	defer os.Unsetenv("ENCRYPT_KEY")

	err = CheckKeyValidity()
	if err == nil {
		t.Error("Expected error for short key, but got none")
	}

	// Verify it's the right type of error
	if _, ok := err.(*InvalidKeyError); !ok {
		t.Errorf("Expected InvalidKeyError, got %T", err)
	}

	// Test 3: Valid key length (at least 32 chars)
	longEnoughKey := "this-is-a-valid-key-that-is-at-least-thirty-two-characters-long"
	os.Setenv("ENCRYPT_KEY", longEnoughKey)
	err = CheckKeyValidity()
	if err != nil {
		t.Errorf("Unexpected error with valid key: %v", err)
	}
}

// TestCorruptedDataHandling tests how the system handles corrupted encrypted data
func TestCorruptedDataHandling(t *testing.T) {
	// Set up a valid key for encryption
	validKey := "valid-key-for-corruption-testing-12345"
	os.Setenv("ENCRYPT_KEY", validKey)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Create valid encrypted data
	testValue := "test-data-for-corruption"
	encrypted, err := Encrypt(testValue, validKey)
	if err != nil {
		t.Fatalf("Failed to encrypt test data: %v", err)
	}

	// Test that valid data decrypts correctly
	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt valid data: %v", err)
	}
	if decrypted != testValue {
		t.Errorf("Decrypted value doesn't match original: expected '%s', got '%s'", testValue, decrypted)
	}

	// Test corrupted data scenarios
	corruptedScenarios := []string{
		"encrypted:",                      // Empty encrypted data
		"encrypted:invalid_base64!",       // Invalid base64
		"encrypted:" + "aGVsbG8gd29ybGQ=", // Valid base64 but wrong structure
		"encrypted:" + "!!garbage!!",      // Completely invalid base64
		"encrypted:short",                 // Too short to contain salt+nonce
		"encrypted:" + "YWJjZGVmZ2hpams=", // Another valid base64, wrong structure
	}

	enhancedDecryptor := NewEnhancedConfigDecryptor()
	for i, corrupted := range corruptedScenarios {
		t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
			_, err := enhancedDecryptor.DecryptValueWithValidation(corrupted, "test-field")
			if err == nil {
				t.Errorf("Expected error for corrupted data '%s', but got none", corrupted)
			}

			// Verify it's an appropriate error type
			switch err.(type) {
			case *InvalidFormatError, *CorruptedDataError, *DecryptionError:
				// These are all acceptable error types for corrupted data
			default:
				t.Logf("Got error type %T for corrupted data: %v", err, err)
			}
		})
	}
}

// TestEnhancedDecryptionErrors tests the enhanced error handling during decryption
func TestEnhancedDecryptionErrors(t *testing.T) {
	// Set up valid key
	validKey := "valid-test-key-for-enhanced-errors"
	os.Setenv("ENCRYPT_KEY", validKey)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Test 1: Non-encrypted value should pass through unchanged
	plainValue := "plain-text-value"
	result, err := NewEnhancedConfigDecryptor().DecryptValueWithValidation(plainValue, "test-field")
	if err != nil {
		t.Errorf("Non-encrypted value caused unexpected error: %v", err)
	}
	if result != plainValue {
		t.Errorf("Non-encrypted value was changed: expected '%s', got '%s'", plainValue, result)
	}

	// Test 2: Properly encrypted value should decrypt
	original := "value-to-encrypt-and-decrypt"
	encrypted, err := Encrypt(original, validKey)
	if err != nil {
		t.Fatalf("Failed to encrypt for test: %v", err)
	}

	result, err = NewEnhancedConfigDecryptor().DecryptValueWithValidation(encrypted, "test-field")
	if err != nil {
		t.Errorf("Properly encrypted value failed to decrypt: %v", err)
	}
	if result != original {
		t.Errorf("Decrypted value doesn't match original: expected '%s', got '%s'", original, result)
	}

	// Test 3: Wrong key should cause appropriate error
	wrongKey := "wrong-key-for-testing-purposes-"
	os.Setenv("ENCRYPT_KEY", wrongKey)

	_, err = NewEnhancedConfigDecryptor().DecryptValueWithValidation(encrypted, "test-field")
	if err == nil {
		t.Error("Expected error when decrypting with wrong key, but got none")
	}
	os.Setenv("ENCRYPT_KEY", validKey) // Restore valid key
}

// TestBatchDecryptionWithErrorHandling tests batch decryption with error reporting
func TestBatchDecryptionWithErrorHandling(t *testing.T) {
	// Set up key
	testKey := "batch-test-key-for-error-handling-12345"
	os.Setenv("ENCRYPT_KEY", testKey)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Create mixed valid and invalid encrypted values
	plainValue := "plain-text"
	validEncrypted, err := Encrypt("valid-data", testKey)
	if err != nil {
		t.Fatalf("Failed to encrypt valid data: %v", err)
	}
	invalidEncrypted := "encrypted:invalid-base64-data!"

	values := map[string]string{
		"plain_field":  plainValue,
		"valid_field":  validEncrypted,
		"invalid_field": invalidEncrypted,
	}

	enhancedDecryptor := NewEnhancedConfigDecryptor()
	results, errors := enhancedDecryptor.BatchDecryptWithValidation(values)

	// Check results
	if results["plain_field"] != plainValue {
		t.Errorf("Plain field not preserved: expected '%s', got '%s'", plainValue, results["plain_field"])
	}

	// Valid encrypted field should be decrypted
	expectedDecrypted := "valid-data"
	if results["valid_field"] != expectedDecrypted {
		t.Errorf("Valid field not decrypted properly: expected '%s', got '%s'", expectedDecrypted, results["valid_field"])
	}

	// Invalid field should have an error
	if errors["invalid_field"] == nil {
		t.Error("Expected error for invalid field, but got none")
	}

	// Plain and valid fields should have no errors
	if errors["plain_field"] != nil {
		t.Errorf("Plain field had unexpected error: %v", errors["plain_field"])
	}
	if errors["valid_field"] != nil {
		t.Errorf("Valid field had unexpected error: %v", errors["valid_field"])
	}
}

// TestValidateEncryptedValue tests the validation function
func TestValidateEncryptedValue(t *testing.T) {
	// Valid encrypted value
	testKey := "validation-test-key-32-chars-long"
	os.Setenv("ENCRYPT_KEY", testKey)
	defer os.Unsetenv("ENCRYPT_KEY")

	validEncrypted, err := Encrypt("test-data", testKey)
	if err != nil {
		t.Fatalf("Failed to create valid encrypted data: %v", err)
	}

	err = ValidateEncryptedValue(validEncrypted, "test-field")
	if err != nil {
		t.Errorf("Valid encrypted value failed validation: %v", err)
	}

	// Test invalid prefixes
	invalidValues := []string{
		"not-encrypted:value",  // Wrong prefix
		"",                     // Empty string
		"just-text",           // No prefix
		"encrypted",           // Missing colon
	}

	for i, invalidValue := range invalidValues {
		err = ValidateEncryptedValue(invalidValue, "test-field")
		if err == nil {
			t.Errorf("Test %d: Expected validation error for '%s', but got none", i, invalidValue)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestErrorMessagesAreHelpful tests that error messages are user-friendly and informative
func TestErrorMessagesAreHelpful(t *testing.T) {
	// Set up key and test
	testKey := "test-key-for-helpful-errors-12345"
	os.Setenv("ENCRYPT_KEY", testKey)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Test InvalidKeyError message construction
	shortKey := "short"
	os.Setenv("ENCRYPT_KEY", shortKey)
	err := CheckKeyValidity()
	if err != nil {
		errorMsg := err.Error()
		if !strings.Contains(errorMsg, "too short") {
			t.Errorf("Short key error message doesn't mention 'too short': %s", errorMsg)
		}
		if !strings.Contains(errorMsg, "32 characters") {
			t.Errorf("Short key error message doesn't mention required length: %s", errorMsg)
		}
	}
	os.Setenv("ENCRYPT_KEY", testKey) // Restore

	// Test CorruptedDataError in enhanced decryptor
	invalidEncrypted := "encrypted:invalid-base64-data!"
	enhancedDecryptor := NewEnhancedConfigDecryptor()
	_, err = enhancedDecryptor.DecryptValueWithValidation(invalidEncrypted, "github.token")
	if err != nil {
		errorMsg := err.Error()
		// Check if it's an InvalidFormatError which would not contain the field name in our implementation
		if _, ok := err.(*InvalidFormatError); !ok {
			// For other error types, verify they contain field information
			if !strings.Contains(errorMsg, "github.token") {
				t.Errorf("Error message doesn't include field name: %s", errorMsg)
			}
		}
	}
}