package main

import (
	"fmt"
	"os"

	"github.com/gaodengpan/image-copier/internal/encryption"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: encrypt <value_to_encrypt>")
		fmt.Println("Note: ENCRYPT_KEY environment variable must be set")
		os.Exit(1)
	}

	valueToEncrypt := os.Args[1]

	// Create a new config encryptor
	encryptor, err := encryption.NewConfigEncryptor()
	if err != nil {
		fmt.Printf("Error creating encryptor: %v\n", err)
		os.Exit(1)
	}

	// Encrypt the value
	encryptedValue, err := encryptor.EncryptValue(valueToEncrypt)
	if err != nil {
		fmt.Printf("Error encrypting value: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(encryptedValue)
}