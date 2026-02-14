package encryption

import (
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

// DeriveKey derives a cryptographic key from a password using PBKDF2
// This function uses SHA-256 hash function with a specified salt and iteration count
func DeriveKey(password string, salt []byte, iterations int, keyLen int) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, keyLen, sha256.New)
}
