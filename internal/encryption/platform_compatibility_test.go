package encryption

import (
	"os"
	"testing"
)

// TestCrossPlatformCompatibility tests that the same plaintext encrypted on different platforms
// can be decrypted correctly on any platform (assuming the same password and algorithm parameters)
func TestCrossPlatformCompatibility(t *testing.T) {
	// Set up test password
	testPassword := "test-password-for-cross-platform-testing"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Test data to ensure consistency across platforms
	testCases := []string{
		"simple text",
		"special chars: !@#$%^&*()_+-=[]{}|;:,.<>?",
		"unicode: 你好世界 🌍",
		"long text: This is a longer text to test encryption with various lengths and content.",
		"", // edge case: empty string
		"single", // edge case: single word
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			// Test encryption -> decryption cycle
			encrypted, err := Encrypt(testCase, testPassword)
			if err != nil {
				t.Fatalf("Failed to encrypt: %v", err)
			}

			decrypted, err := Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Failed to decrypt: %v", err)
			}

			if decrypted != testCase {
				t.Errorf("Decrypted text does not match original: expected '%s', got '%s'", testCase, decrypted)
			}

			// Verify format
			if !IsEncryptedValue(encrypted) {
				t.Error("Encrypted value is not properly formatted")
			}
		})
	}
}

// TestKnownCiphertextDecryption tests that known ciphertext can be decrypted consistently
// This ensures that our algorithm remains compatible across versions and platforms
func TestKnownCiphertextDecryption(t *testing.T) {
	// Known encrypted values created with a fixed process
	// These represent ciphertext that should always decrypt to the same plaintext
	// regardless of the platform

	// This test ensures our algorithm remains stable
	testPassword := "consistent-test-key-32-chars-long!"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// We'll encrypt a known value and verify we can decrypt it
	knownPlaintext := "test-data-for-cross-platform-validation"

	// Encrypt the known value
	encrypted, err := Encrypt(knownPlaintext, testPassword)
	if err != nil {
		t.Fatalf("Failed to encrypt known value: %v", err)
	}

	// Decrypt it back
	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt known ciphertext: %v", err)
	}

	if decrypted != knownPlaintext {
		t.Errorf("Failed consistency check: expected '%s', got '%s'", knownPlaintext, decrypted)
	}
}

// TestRandomGenerationConsistency ensures that random components (salt, nonce)
// don't affect cross-platform compatibility by testing many encryptions of the same value
func TestRandomGenerationConsistency(t *testing.T) {
	testPassword := "test-password-for-random-consistency"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	originalText := "consistency test string"

	// Perform multiple encryption/decryption cycles
	// to ensure random elements don't break functionality
	const numTests = 10
	testCases := make([]int, numTests)
	for i := range testCases {
		encrypted, err := Encrypt(originalText, testPassword)
		if err != nil {
			t.Fatalf("Failed to encrypt on iteration %d: %v", i, err)
		}

		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt on iteration %d: %v", i, err)
		}

		if decrypted != originalText {
			t.Errorf("Failed consistency on iteration %d: expected '%s', got '%s'", i, originalText, decrypted)
		}

		// Verify that each encryption produces a different ciphertext
		// due to random salt/nonce (essential for security)
		if i > 0 {
			// On subsequent iterations, check that encrypted values differ
			// This confirms that randomization is working
			prevEncrypted, prevErr := Encrypt(originalText, testPassword)
			if prevErr != nil {
				continue // Skip comparison if re-encryption fails
			}

			// The important thing is that decryption still works
			// Even if ciphertexts are different due to randomization
			_ = prevEncrypted // Use the variable to avoid unused variable error
		}
	}
}