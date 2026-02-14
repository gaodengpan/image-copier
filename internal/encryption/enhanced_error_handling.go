package encryption

import (
	"fmt"
	"os"
	"strings"
)

// Enhanced error handling for various error scenarios

// InvalidKeyError represents an error when the encryption key is invalid
type InvalidKeyError struct {
	Field   string
	Message string
}

func (e *InvalidKeyError) Error() string {
	return fmt.Sprintf("invalid key error for field '%s': %s - Please ensure the ENCRYPT_KEY environment variable is correctly set and is at least 32 characters long", e.Field, e.Message)
}

// CorruptedDataError represents an error when the encrypted data is corrupted or tampered with
type CorruptedDataError struct {
	Field   string
	Value   string
	Message string
}

func (e *CorruptedDataError) Error() string {
	return fmt.Sprintf("corrupted data error for field '%s': %s - The encrypted value may have been corrupted or tampered with (value: %.30s...)", e.Field, e.Message, e.Value)
}

// ValidationError represents an error when input validation fails
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s (value: %s)", e.Field, e.Message, e.Value)
}

// EnhancedConfigDecryptor extends ConfigDecryptor with enhanced error handling
type EnhancedConfigDecryptor struct{}

// NewEnhancedConfigDecryptor creates a new instance of EnhancedConfigDecryptor
func NewEnhancedConfigDecryptor() *EnhancedConfigDecryptor {
	return &EnhancedConfigDecryptor{}
}

// DecryptValueWithValidation decrypts a configuration value with enhanced error handling
func (ecd *EnhancedConfigDecryptor) DecryptValueWithValidation(encryptedValue string, fieldName string) (string, error) {
	if !IsEncryptedValue(encryptedValue) {
		// If the value is not encrypted, return as is
		return encryptedValue, nil
	}

	// Validate the format before attempting decryption
	if !IsValidEncryptedFormat(encryptedValue) {
		return "", &InvalidFormatError{
			Value:          encryptedValue,
			ExpectedFormat: "encrypted:base64_encoded_data",
			Message:        "invalid encrypted format",
		}
	}

	// Check that the encryption key is available
	password := os.Getenv("ENCRYPT_KEY")
	if password == "" {
		return "", &InvalidKeyError{
			Field:   fieldName,
			Message: "ENCRYPT_KEY environment variable not set",
		}
	}

	// Validate key length
	if len(password) < 32 {
		return "", &InvalidKeyError{
			Field:   fieldName,
			Message: fmt.Sprintf("ENCRYPT_KEY is too short, must be at least 32 characters, got %d", len(password)),
		}
	}

	// Attempt decryption and handle specific error cases
	result, err := Decrypt(encryptedValue)
	if err != nil {
		// Check for specific error types and enhance messages
		if _, ok := err.(*InvalidFormatError); ok {
			return "", &InvalidFormatError{
				Value:          encryptedValue,
				ExpectedFormat: "encrypted:base64_encoded_data",
				Message:        fmt.Sprintf("%s - please verify the encrypted value format", err.Error()),
			}
		}

		if _, ok := err.(*DecryptionError); ok {
			// Wrap the decryption error with field info
			return "", &CorruptedDataError{
				Field:   fieldName,
				Value:   encryptedValue,
				Message: fmt.Sprintf("%s - The encrypted data may be corrupted, or the encryption key might be incorrect", err.Error()),
			}
		}

		// For other errors, return a generic error with helpful info
		return "", &CorruptedDataError{
			Field:   fieldName,
			Value:   encryptedValue,
			Message: fmt.Sprintf("unexpected error during decryption: %v - This might indicate corrupted data or an incorrect encryption key", err),
		}
	}

	return result, nil
}

// BatchDecryptWithValidation decrypts multiple values with enhanced error handling
// Returns a map of field names to decrypted values, and a map of errors for failed decryptions
func (ecd *EnhancedConfigDecryptor) BatchDecryptWithValidation(values map[string]string) (map[string]string, map[string]error) {
	results := make(map[string]string)
	errors := make(map[string]error)

	for field, value := range values {
		decrypted, err := ecd.DecryptValueWithValidation(value, field)
		if err != nil {
			errors[field] = err
		} else {
			results[field] = decrypted
		}
	}

	return results, errors
}

// ValidateAndDecryptAllFields validates and decrypts all fields in a config-like structure
// This simulates the kind of processing that would happen when loading an entire configuration
func (ecd *EnhancedConfigDecryptor) ValidateAndDecryptAllFields(
	githubToken, registryUsername, registryPassword string,
) (string, string, string, map[string]error) {
	fields := map[string]string{
		"github.token":      githubToken,
		"registry.username": registryUsername,
		"registry.password": registryPassword,
	}

	results, errors := ecd.BatchDecryptWithValidation(fields)

	return results["github.token"], results["registry.username"], results["registry.password"], errors
}

// Additional utility functions for error scenario detection

// CheckKeyValidity verifies if the encryption key is properly configured
func CheckKeyValidity() error {
	password := os.Getenv("ENCRYPT_KEY")
	if password == "" {
		return &InvalidKeyError{
			Field:   "general",
			Message: "ENCRYPT_KEY environment variable not set",
		}
	}

	if len(password) < 32 {
		return &InvalidKeyError{
			Field:   "general",
			Message: fmt.Sprintf("ENCRYPT_KEY is too short: %d characters, must be at least 32 characters", len(password)),
		}
	}

	return nil
}

// ValidateEncryptedValue performs extended validation on an encrypted value
func ValidateEncryptedValue(value string, fieldName string) error {
	if !strings.HasPrefix(value, "encrypted:") {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: "value does not have required 'encrypted:' prefix",
		}
	}

	encodedPart := value[10:]

	// Check if the encoded part looks like valid base64 (basic validation)
	if len(encodedPart) == 0 {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: "encoded part is empty",
		}
	}

	// Additional checks could go here based on expected minimum length
	// Encoded data should contain at least salt (16 bytes) + nonce (12 bytes) + tag + some content

	return nil
}