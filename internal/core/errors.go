package core

import (
	"errors"
	"fmt"
)

// EnhancedError represents an enhanced error with additional context
type EnhancedError struct {
	Code          string                 `json:"code"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Recommendation string                `json:"recommendation,omitempty"`
	Cause         error                  `json:"-"` // 不序列化底层错误
}

// Error implements the error interface
func (e *EnhancedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (e *EnhancedError) Unwrap() error {
	return e.Cause
}

// NewEnhancedError creates a new EnhancedError
func NewEnhancedError(code, message string) *EnhancedError {
	return &EnhancedError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with additional context
func WrapError(cause error, code, message string) *EnhancedError {
	return &EnhancedError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the EnhancedError
func (e *EnhancedError) WithDetail(key string, value interface{}) *EnhancedError {
	e.Details[key] = value
	return e
}

// WithRecommendation adds a recommendation to the EnhancedError
func (e *EnhancedError) WithRecommendation(recommendation string) *EnhancedError {
	e.Recommendation = recommendation
	return e
}

// Error codes for the application
const (
	ErrCodeInvalidImageName   = "INVALID_IMAGE_NAME"
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeCommandFailed      = "COMMAND_FAILED"
	ErrCodeConfigNotFound     = "CONFIG_NOT_FOUND"
	ErrCodeEncryptionFailed   = "ENCRYPTION_FAILED"
	ErrCodeDecryptionFailed   = "DECRYPTION_FAILED"
	ErrCodeNetworkError       = "NETWORK_ERROR"
	ErrCodeAuthentication     = "AUTHENTICATION_FAILED"
	ErrCodeAuthorization      = "AUTHORIZATION_FAILED"
	ErrCodeImageNotFound      = "IMAGE_NOT_FOUND"
	ErrCodeInvalidReference   = "INVALID_REFERENCE"
	ErrCodeInsufficientPrivileges = "INSUFFICIENT_PRIVILEGES"
)

// Helper functions to create common errors
var (
	ErrInvalidImageName = errors.New("invalid image name")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrCommandFailed = errors.New("command failed")
	ErrConfigNotFound = errors.New("configuration file not found")
	ErrEncryptionFailed = errors.New("encryption process failed")
	ErrDecryptionFailed = errors.New("decryption process failed")
	ErrNetworkError = errors.New("network error occurred")
	ErrAuthentication = errors.New("authentication failed")
	ErrAuthorization = errors.New("authorization failed")
	ErrImageNotFound = errors.New("image not found")
	ErrInvalidReference = errors.New("invalid image reference format")
	ErrInsufficientPrivileges = errors.New("insufficient privileges to access image")
)