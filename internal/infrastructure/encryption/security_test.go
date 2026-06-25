package encryption

import (
	"os"
	"testing"
)

func TestEncryptionDecryptionRoundTrip(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-for-security-tests-32chars")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	testValues := []string{
		"simple text",
		"special chars: !@#$%^&*()",
		"unicode: 你好世界 🌍",
		"long text: this is a much longer text to test encryption with various content and ensure it works properly",
		"",                           // empty string
		"1234567890",                 // numbers
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ", // uppercase
		"abcdefghijklmnopqrstuvwxyz", // lowercase
	}

	for _, testValue := range testValues {
		// Encrypt the value
		encrypted, err := Encrypt(testValue, "test-key-for-security-tests-32chars")
		if err != nil {
			t.Errorf("Failed to encrypt '%s': %v", testValue, err)
			continue
		}

		// Verify it's in correct format
		if !IsEncryptedValue(encrypted) {
			t.Errorf("Encrypted value '%s' for input '%s' is not in correct format", encrypted, testValue)
			continue
		}

		// Verify it's different from original (basic security check)
		if encrypted == testValue {
			t.Errorf("Encrypted value is identical to original for '%s', indicating possible encryption failure", testValue)
			continue
		}

		// Decrypt the value
		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Errorf("Failed to decrypt value '%s' for input '%s': %v", encrypted, testValue, err)
			continue
		}

		// Verify decryption result matches original
		if decrypted != testValue {
			t.Errorf("Decryption result '%s' does not match original '%s'", decrypted, testValue)
			continue
		}
	}
}

func TestDifferentInputsProduceDifferentOutputs(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-for-security-tests-32chars")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	testValues := []string{
		"first value",
		"second value",
		"third value",
		"different value",
		"another value",
	}

	encryptedValues := make(map[string]string)

	for _, testValue := range testValues {
		encrypted, err := Encrypt(testValue, "test-key-for-security-tests-32chars")
		if err != nil {
			t.Fatalf("Failed to encrypt '%s': %v", testValue, err)
		}

		// Check if this encrypted value was produced by a different input
		for original, existingEncrypted := range encryptedValues {
			if existingEncrypted == encrypted && original != testValue {
				t.Errorf("Same encrypted output '%s' for different inputs '%s' and '%s'", encrypted, original, testValue)
			}
		}

		encryptedValues[testValue] = encrypted
	}
}

func TestDecryptionWithWrongKey(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-first-32chars-length-test")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	testValue := "test value for wrong key test"

	// Encrypt with one key
	encrypted, err := Encrypt(testValue, "test-key-first-32chars-length-test")
	if err != nil {
		t.Fatalf("Failed to encrypt with first key: %v", err)
	}

	// Temporarily set a different key
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "different-key-32chars-test-value")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	// Attempt to decrypt with different key (should fail)
	_, err = Decrypt(encrypted)
	if err == nil {
		t.Error("Expected decryption to fail with wrong key, but it succeeded")
	}
}

func TestTamperingDetection(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	_ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", "test-key-for-tampering-tests-32chars")
	defer func() { _ = os.Setenv("IMAGE_COPIER_ENCRYPT_KEY", originalKey) }() // Restore original value

	testValue := "test value for tampering detection"
	encrypted, err := Encrypt(testValue, "test-key-for-tampering-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Tamper with the encrypted string (change one character)
	if len(encrypted) > 15 { // Make sure it's long enough
		tampered := encrypted[:10] + "X" + encrypted[11:] // Change one character in the middle

		// Attempt to decrypt tampered string (should fail)
		_, err := Decrypt(tampered)
		if err == nil {
			t.Error("Expected decryption to fail with tampered data, but it succeeded")
		}
	}
}

func TestSecurityConstants(t *testing.T) {
	// Verify that our encryption constants are appropriate for security
	if SaltLength < 16 {
		t.Errorf("Salt length %d may be too short for security; 16 bytes minimum recommended", SaltLength)
	}

	if NonceLength != 12 {
		t.Errorf("Nonce length %d is not the standard 12 bytes for AES-GCM", NonceLength)
	}

	if IterationCount < 10000 {
		t.Errorf("PBKDF2 iteration count %d may be too low; 10000+ recommended", IterationCount)
	}

	if KeyLength != 32 {
		t.Errorf("Key length %d is not 32 bytes (256-bit) as required for AES-256", KeyLength)
	}
}
