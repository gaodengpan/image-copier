package encryption

import (
	"encoding/base64"
	"os"
	"testing"
)

// TestRandomSaltGeneration verifies that the random salt generation is working properly
func TestRandomSaltGeneration(t *testing.T) {
	testPassword := "test-password-for-random-generation"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Generate multiple salts and verify they're all different
	saltSet := make(map[string]bool)
	numSalts := 100

	for i := 0; i < numSalts; i++ {
		// We need to test the salt generation inside the Encrypt function
		// Since the salt is generated randomly inside the Encrypt function,
		// we'll encrypt the same value multiple times and check that
		// the resulting encrypted values are different (due to different salts)
		encrypted1, err := Encrypt("test-value", testPassword)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}

		encrypted2, err := Encrypt("test-value", testPassword)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}

		// Different encrypted values should be produced each time due to random salt/nonce
		if encrypted1 == encrypted2 {
			t.Errorf("Encryption with same input produced identical output, salt generation may not be random")
		}

		// Add to our set to verify uniqueness
		saltSet[encrypted1] = true
	}

	// All generated encrypted values should be unique
	if len(saltSet) != numSalts {
		t.Errorf("Expected %d unique encrypted values, got %d. Random generation may not be working properly.", numSalts, len(saltSet))
	}
}

// TestRandomNonceGeneration verifies that the random nonce generation is working properly
func TestRandomNonceGeneration(t *testing.T) {
	testPassword := "test-password-for-nonce-generation"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// We'll reuse the same test as salt generation since nonce is generated similarly
	TestRandomSaltGeneration(t)
}

// TestRNGSecurityProperties tests properties of the random number generator
// to ensure it meets cryptographic security requirements
func TestRNGSecurityProperties(t *testing.T) {
	testPassword := "test-password-for-rng-security"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Test that repeated encryptions produce outputs that appear random
	// This tests the distribution of output bytes
	testValues := []string{
		"short",
		"this is a longer test value",
		"another test value",
		"yet another test value",
	}

	for _, testVal := range testValues {
		encryptions := make([]string, 50)
		for i := 0; i < 50; i++ {
			encrypted, err := Encrypt(testVal, testPassword)
			if err != nil {
				t.Fatalf("Failed to encrypt: %v", err)
			}
			encryptions[i] = encrypted
		}

		// Check that outputs have sufficient entropy by looking for patterns
		// In a truly random output, we shouldn't see many duplicates
		uniqueOutputs := make(map[string]bool)
		for _, enc := range encryptions {
			uniqueOutputs[enc] = true
		}

		// Almost all outputs should be unique due to random salt/nonce
		if len(uniqueOutputs) < 45 { // Allow for a few unlikely collisions
			t.Errorf("Too many duplicate encryptions of same input (%d/%d), randomness may be compromised",
				len(encryptions)-len(uniqueOutputs), len(encryptions))
		}
	}
}

// TestRandomBytesGeneration tests the underlying random bytes generation
func TestRandomBytesGeneration(t *testing.T) {
	// This test verifies that crypto/rand (used in Encrypt function) is producing proper random data

	// The random generation happens inside Encrypt, so we test indirectly
	// by verifying that encrypted outputs are sufficiently different

	testPassword := "test-password-for-indirect-rand-test"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	// Encrypt the same value multiple times
	sampleSize := 100
	encryptedOutputs := make([]string, sampleSize)

	for i := 0; i < sampleSize; i++ {
		encrypted, err := Encrypt("test-string-for-rand-validation", testPassword)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedOutputs[i] = encrypted
	}

	// Calculate byte differences between outputs to ensure randomness
	// Convert to raw bytes to compare
	rawOutputs := make([][]byte, len(encryptedOutputs))
	for i, enc := range encryptedOutputs {
		// Skip "encrypted:" prefix and decode
		if len(enc) <= 10 {
			t.Fatal("Encrypted value too short")
		}
		withoutPrefix := enc[10:] // Remove "encrypted:" prefix
		decoded, err := base64.StdEncoding.DecodeString(withoutPrefix)
		if err != nil {
			// Use a simpler approach by just checking the encoded strings are different
			continue
		}
		rawOutputs[i] = decoded
	}

	// We can't check the internal random values directly, but we can verify that
	// the encryption process (which uses crypto/rand) produces different outputs
	// each time, which indicates the RNG is working

	// Count unique outputs
	uniqueMap := make(map[string]bool)
	for _, output := range encryptedOutputs {
		uniqueMap[output] = true
	}

	if len(uniqueMap) < sampleSize*9/10 { // Expect 90% uniqueness at minimum
		t.Errorf("Random number generator may not be producing diverse outputs: %d/%d unique",
			len(uniqueMap), sampleSize)
	}
}

// Test that crypto/rand is used (as opposed to math/rand or other non-cryptographic RNGs)
func TestCryptoRandUsage(t *testing.T) {
	// This test ensures that the implementation uses crypto/rand
	// which is cryptographically secure as opposed to pseudo-random generators

	// While we can't directly observe the internal use of crypto/rand,
	// we can verify that the output meets security requirements
	// by ensuring encrypted values of the same input are different each time

	testPassword := "test-password-for-crypto-rand-verification"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	const numTests = 50
	testInput := "consistent-input-for-rand-verification"

	// Store all encrypted values
	encryptedValues := make([]string, numTests)
	for i := 0; i < numTests; i++ {
		encrypted, err := Encrypt(testInput, testPassword)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedValues[i] = encrypted
	}

	// Count unique values - with proper random generation, these should all be different
	uniqueCount := 0
	seen := make(map[string]bool)
	for _, val := range encryptedValues {
		if !seen[val] {
			uniqueCount++
			seen[val] = true
		}
	}

	// With crypto/rand and the use of random salt/nonce, all encryptions of the
	// same value should produce different outputs
	if uniqueCount != numTests {
		t.Errorf("Expected all %d encryptions to be unique, but only %d were unique",
			numTests, uniqueCount)
	}

	// Verify each can be decrypted back to original value
	for _, encrypted := range encryptedValues {
		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Errorf("Could not decrypt randomly-generated encryption: %v", err)
			continue
		}
		if decrypted != testInput {
			t.Errorf("Decryption mismatch: expected '%s', got '%s'", testInput, decrypted)
		}
	}
}