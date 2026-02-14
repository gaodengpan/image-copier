package encryption

import (
	"os"
	"testing"
)

// BenchmarkEncrypt benchmarks the Encrypt function
func BenchmarkEncrypt(b *testing.B) {
	testPassword := "benchmark-test-password-for-encryption"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	testData := "test-data-for-encryption-performance-benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(testData, testPassword)
		if err != nil {
			b.Fatalf("Encrypt failed during benchmark: %v", err)
		}
	}
}

// BenchmarkDecrypt benchmarks the Decrypt function
func BenchmarkDecrypt(b *testing.B) {
	testPassword := "benchmark-test-password-for-decryption"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	testData := "test-data-for-decryption-performance-benchmark"
	encryptedData, err := Encrypt(testData, testPassword)
	if err != nil {
		b.Fatalf("Failed to prepare encrypted data: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(encryptedData)
		if err != nil {
			b.Fatalf("Decrypt failed during benchmark: %v", err)
		}
	}
}

// BenchmarkEncryptDecryptCycle benchmarks a complete encrypt-decrypt cycle
func BenchmarkEncryptDecryptCycle(b *testing.B) {
	testPassword := "benchmark-test-password-for-cycle"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	testData := "test-data-for-encrypt-decrypt-cycle-performance-benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Encrypt
		encrypted, err := Encrypt(testData, testPassword)
		if err != nil {
			b.Fatalf("Encrypt failed during benchmark: %v", err)
		}

		// Decrypt
		decrypted, err := Decrypt(encrypted)
		if err != nil {
			b.Fatalf("Decrypt failed during benchmark: %v", err)
		}

		// Verify correctness
		if decrypted != testData {
			b.Fatalf("Data mismatch: expected '%s', got '%s'", testData, decrypted)
		}
	}
}

// BenchmarkConfigEncryptor_EncryptValue benchmarks the ConfigEncryptor.EncryptValue method
func BenchmarkConfigEncryptor_EncryptValue(b *testing.B) {
	testPassword := "benchmark-test-password-for-config-encryptor"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	encryptor, err := NewConfigEncryptor()
	if err != nil {
		b.Fatalf("Failed to create config encryptor: %v", err)
	}

	testData := "test-data-for-config-encryptor-benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.EncryptValue(testData)
		if err != nil {
			b.Fatalf("ConfigEncryptor.EncryptValue failed during benchmark: %v", err)
		}
	}
}

// BenchmarkConfigDecryptor_DecryptValue benchmarks the ConfigDecryptor.DecryptValue method
func BenchmarkConfigDecryptor_DecryptValue(b *testing.B) {
	testPassword := "benchmark-test-password-for-config-decryptor"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	encryptor, err := NewConfigEncryptor()
	if err != nil {
		b.Fatalf("Failed to create config encryptor: %v", err)
	}

	decryptor := NewConfigDecryptor()

	testData := "test-data-for-config-decryptor-benchmark"
	encryptedData, err := encryptor.EncryptValue(testData)
	if err != nil {
		b.Fatalf("Failed to prepare encrypted data: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decryptor.DecryptValue(encryptedData)
		if err != nil {
			b.Fatalf("ConfigDecryptor.DecryptValue failed during benchmark: %v", err)
		}
	}
}

// BenchmarkBatchEncryption benchmarks batch encryption of multiple values
func BenchmarkBatchEncryption(b *testing.B) {
	testPassword := "benchmark-test-password-for-batch-encryption"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	encryptor, err := NewConfigEncryptor()
	if err != nil {
		b.Fatalf("Failed to create config encryptor: %v", err)
	}

	testData := []string{
		"first-value-to-encrypt",
		"second-value-to-encrypt",
		"third-value-to-encrypt",
		"fourth-value-to-encrypt",
		"fifth-value-to-encrypt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, data := range testData {
			_, err := encryptor.EncryptValue(data)
			if err != nil {
				b.Fatalf("Batch encryption failed during benchmark: %v", err)
			}
		}
	}
}

// BenchmarkBatchDecryption benchmarks batch decryption of multiple values
func BenchmarkBatchDecryption(b *testing.B) {
	testPassword := "benchmark-test-password-for-batch-decryption"
	os.Setenv("ENCRYPT_KEY", testPassword)
	defer os.Unsetenv("ENCRYPT_KEY")

	encryptor, err := NewConfigEncryptor()
	if err != nil {
		b.Fatalf("Failed to create config encryptor: %v", err)
	}

	decryptor := NewConfigDecryptor()

	testData := []string{
		"first-value-to-decrypt",
		"second-value-to-decrypt",
		"third-value-to-decrypt",
		"fourth-value-to-decrypt",
		"fifth-value-to-decrypt",
	}

	// Prepare encrypted data
	encryptedData := make([]string, len(testData))
	for i, data := range testData {
		encrypted, err := encryptor.EncryptValue(data)
		if err != nil {
			b.Fatalf("Failed to prepare encrypted data: %v", err)
		}
		encryptedData[i] = encrypted
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, encrypted := range encryptedData {
			decrypted, err := decryptor.DecryptValue(encrypted)
			if err != nil {
				b.Fatalf("Batch decryption failed during benchmark: %v", err)
			}

			if decrypted != testData[j] {
				b.Fatalf("Data mismatch in batch: expected '%s', got '%s'", testData[j], decrypted)
			}
		}
	}
}