package encryption

import (
	"os"
	"testing"
	"time"
)

func BenchmarkEncryption(b *testing.B) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-performance-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "this is a test value for performance benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(testValue, "test-key-for-performance-tests-32chars")
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

func BenchmarkDecryption(b *testing.B) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-performance-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "this is a test value for performance benchmarking"
	encrypted, err := Encrypt(testValue, "test-key-for-performance-tests-32chars")
	if err != nil {
		b.Fatalf("Failed to encrypt for benchmark: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(encrypted)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}

func TestDecryptionPerformance(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-performance-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	testValue := "this is a test value for performance testing"
	encrypted, err := Encrypt(testValue, "test-key-for-performance-tests-32chars")
	if err != nil {
		t.Fatalf("Failed to encrypt for performance test: %v", err)
	}

	startTime := time.Now()
	decrypted, err := Decrypt(encrypted)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != testValue {
		t.Errorf("Decrypted value doesn't match original")
	}

	if duration.Milliseconds() > 100 {
		t.Errorf("Decryption took %dms, which exceeds the 100ms requirement", duration.Milliseconds())
	} else {
		t.Logf("Decryption completed in %dms, within the required 100ms", duration.Milliseconds())
	}
}

func TestMultipleDecryptionsPerformance(t *testing.T) {
	// Set up environment variable for testing
	originalKey := os.Getenv("ENCRYPT_KEY")
	os.Setenv("ENCRYPT_KEY", "test-key-for-performance-tests-32chars")
	defer os.Setenv("ENCRYPT_KEY", originalKey) // Restore original value

	const numOperations = 100
	testValues := make([]string, numOperations)
	encryptedValues := make([]string, numOperations)

	// Prepare test data
	for i := 0; i < numOperations; i++ {
		testValues[i] = "test value for performance testing " + string(rune(i+'0'))
		var err error
		encryptedValues[i], err = Encrypt(testValues[i], "test-key-for-performance-tests-32chars")
		if err != nil {
			t.Fatalf("Failed to encrypt for performance test: %v", err)
		}
	}

	startTime := time.Now()

	// Perform multiple decryptions
	for i := 0; i < numOperations; i++ {
		decrypted, err := Decrypt(encryptedValues[i])
		if err != nil {
			t.Fatalf("Decryption %d failed: %v", i, err)
		}
		if decrypted != testValues[i] {
			t.Errorf("Decrypted value %d doesn't match original", i)
		}
	}

	duration := time.Since(startTime)
	avgDuration := duration.Milliseconds() / int64(numOperations)

	if avgDuration > 100 {
		t.Errorf("Average decryption took %dms, which exceeds the 100ms requirement", avgDuration)
	} else {
		t.Logf("Average decryption completed in %dms, within the required 100ms", avgDuration)
	}
}
