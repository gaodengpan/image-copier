package main

import (
	"fmt"
	"os"

	"github.com/gaodengpan/image-copier/internal/encryption"
)

func main() {
	// Example usage of the secure configuration management
	fmt.Println("Secure Configuration Management Demo")

	// 1. Example of encrypting a value
	fmt.Println("\n1. Encrypting a configuration value:")

	// Set the encryption key in environment
	os.Setenv("ENCRYPT_KEY", "my-very-secure-key-that-is-at-least-32-chars")

	// Create an encryptor
	encryptor, err := encryption.NewConfigEncryptor()
	if err != nil {
		fmt.Printf("Error creating encryptor: %v\n", err)
		return
	}

	// Encrypt a sensitive value
	originalValue := "my-super-secret-token"
	encryptedValue, err := encryptor.EncryptValue(originalValue)
	if err != nil {
		fmt.Printf("Error encrypting value: %v\n", err)
		return
	}

	fmt.Printf("Original: %s\n", originalValue)
	fmt.Printf("Encrypted: %s\n", encryptedValue)

	// 2. Example of decrypting the value
	fmt.Println("\n2. Decrypting the value:")
	decryptor := encryption.NewConfigDecryptor()
	decryptedValue, err := decryptor.DecryptValue(encryptedValue)
	if err != nil {
		fmt.Printf("Error decrypting value: %v\n", err)
		return
	}

	fmt.Printf("Decrypted: %s\n", decryptedValue)

	// 3. Example of checking if a value is encrypted
	fmt.Println("\n3. Checking if a value is encrypted:")
	isEncrypted := encryption.IsEncryptedValue(encryptedValue)
	fmt.Printf("Is encrypted: %t\n", isEncrypted)

	isPlainText := encryption.IsEncryptedValue("plain-text-value")
	fmt.Printf("Plain text is encrypted: %t\n", isPlainText)

	// 4. Example of validating encrypted format
	fmt.Println("\n4. Validating encrypted format:")
	isValid := encryption.IsValidEncryptedFormat(encryptedValue)
	fmt.Printf("Valid format: %t\n", isValid)

	invalidFormat := encryption.IsValidEncryptedFormat("invalid-format")
	fmt.Printf("Invalid format is valid: %t\n", invalidFormat)

	fmt.Println("\nDemo completed successfully!")
}
