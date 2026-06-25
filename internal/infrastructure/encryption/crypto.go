package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
)

// Constants for encryption
const (
	// SaltLength is the length of salt in bytes
	SaltLength = 16
	// NonceLength is the length of nonce in bytes for AES-GCM
	NonceLength = 12
	// IterationCount is the number of iterations for PBKDF2
	IterationCount = 10000
	// KeyLength is the length of the derived key in bytes
	KeyLength = 32 // for AES-256
)

// Encrypt encrypts a string using AES-256-GCM algorithm
// Returns a string in the format "encrypted:<base64_encoded_encrypted_data>"
// The encoded data contains [salt][nonce][ciphertext][authentication_tag]
func Encrypt(plaintext string, password string) (string, error) {
	// Derive a key from the password
	salt := make([]byte, SaltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return "", &EncryptionError{
			Message: "failed to generate salt",
			Cause:   err,
		}
	}

	key := DeriveKey(password, salt, IterationCount, KeyLength)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", &EncryptionError{
			Message: "failed to create cipher",
			Cause:   err,
		}
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", &EncryptionError{
			Message: "failed to create GCM",
			Cause:   err,
		}
	}

	// Generate random nonce
	nonce := make([]byte, NonceLength)
	_, err = rand.Read(nonce)
	if err != nil {
		return "", &EncryptionError{
			Message: "failed to generate nonce",
			Cause:   err,
		}
	}

	// Encrypt the plaintext
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine salt, nonce, and ciphertext into one byte slice
	result := make([]byte, SaltLength+NonceLength+len(ciphertext))
	copy(result[0:SaltLength], salt)
	copy(result[SaltLength:SaltLength+NonceLength], nonce)
	copy(result[SaltLength+NonceLength:], ciphertext)

	// Encode to base64 and add prefix
	encoded := base64.StdEncoding.EncodeToString(result)
	return "encrypted:" + encoded, nil
}

// Decrypt decrypts a string that was encrypted using the Encrypt function
// The input should be in the format "encrypted:<base64_encoded_encrypted_data>"
func Decrypt(encryptedText string) (string, error) {
	// Validate format
	if len(encryptedText) < 11 || encryptedText[:10] != "encrypted:" {
		return "", &InvalidFormatError{
			Value:          encryptedText,
			ExpectedFormat: "encrypted:<base64_encoded_data>",
			Message:        "must start with 'encrypted:'",
		}
	}

	// Extract base64 encoded data
	encoded := encryptedText[10:]

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", &DecryptionError{
			Message: "failed to decode base64",
			Field:   "encrypted_value",
			Cause:   err,
		}
	}

	// Check minimum length: salt(16) + nonce(12) + tag(16) = 44
	if len(data) < SaltLength+NonceLength+16 {
		return "", &DecryptionError{
			Message: "encrypted data too short",
			Field:   "encrypted_value",
		}
	}

	// Extract salt, nonce, and ciphertext
	salt := data[0:SaltLength]
	nonce := data[SaltLength : SaltLength+NonceLength]
	ciphertext := data[SaltLength+NonceLength:]

	// Derive key using the same parameters as encryption
	password, err := getPasswordFromEnv()
	if err != nil {
		return "", &DecryptionError{
			Message: "failed to get password for decryption",
			Field:   "encrypted_value",
			Cause:   err,
		}
	}

	key := DeriveKey(password, salt, IterationCount, KeyLength)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", &DecryptionError{
			Message: "failed to create cipher",
			Field:   "encrypted_value",
			Cause:   err,
		}
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", &DecryptionError{
			Message: "failed to create GCM",
			Field:   "encrypted_value",
			Cause:   err,
		}
	}

	// Decrypt the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", &DecryptionError{
			Message: "failed to decrypt",
			Field:   "encrypted_value",
			Cause:   err,
		}
	}

	return string(plaintext), nil
}

// getPasswordFromEnv retrieves the encryption key from environment variables
func getPasswordFromEnv() (string, error) {
	password := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	if password == "" {
		return "", errors.New("IMAGE_COPIER_ENCRYPT_KEY environment variable not set")
	}
	return password, nil
}
