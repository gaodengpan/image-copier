package encryption

import "fmt"

// EncryptionError represents an error during the encryption process
type EncryptionError struct {
	Message string
	Cause   error
}

func (e *EncryptionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("encryption error: %s (cause: %v)", e.Message, e.Cause)
	}
	return fmt.Sprintf("encryption error: %s", e.Message)
}

// DecryptionError represents an error during the decryption process
type DecryptionError struct {
	Message string
	Field   string
	Cause   error
}

func (e *DecryptionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("decryption error for field '%s': %s (cause: %v)", e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("decryption error for field '%s': %s", e.Field, e.Message)
}

// InvalidFormatError represents an error when the encrypted value format is invalid
type InvalidFormatError struct {
	Value          string
	ExpectedFormat string
	Message        string
}

func (e *InvalidFormatError) Error() string {
	return fmt.Sprintf("invalid format for encrypted value '%s': %s (expected: %s)", e.Value, e.Message, e.ExpectedFormat)
}
