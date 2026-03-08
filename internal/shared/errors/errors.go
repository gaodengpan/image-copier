package errors

import "fmt"

// AdapterError represents errors from the adapter layer (external services)
type AdapterError struct {
	Adapter   string
	Operation string
	Message   string
	Cause     error
}

func (e *AdapterError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s adapter error during %s: %s (cause: %v)", e.Adapter, e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s adapter error during %s: %s", e.Adapter, e.Operation, e.Message)
}

func (e *AdapterError) Unwrap() error {
	return e.Cause
}

func NewAdapterError(adapter, operation, message string, cause error) *AdapterError {
	return &AdapterError{
		Adapter:   adapter,
		Operation: operation,
		Message:   message,
		Cause:     cause,
	}
}

// DomainError represents errors from the domain layer (business rule violations)
type DomainError struct {
	Entity    string
	Operation string
	Message   string
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("%s domain error during %s: %s", e.Entity, e.Operation, e.Message)
}

func NewDomainError(entity, operation, message string) *DomainError {
	return &DomainError{
		Entity:    entity,
		Operation: operation,
		Message:   message,
	}
}

// ValidationError represents errors from input validation
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

type DockerError struct {
	*AdapterError
}

func NewDockerError(operation, message string, cause error) *DockerError {
	return &DockerError{
		AdapterError: NewAdapterError("docker", operation, message, cause),
	}
}

type RegistryError struct {
	*AdapterError
}

func NewRegistryError(operation, message string, cause error) *RegistryError {
	return &RegistryError{
		AdapterError: NewAdapterError("registry", operation, message, cause),
	}
}

type GitHubError struct {
	*AdapterError
	StatusCode int
}

func (e *GitHubError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("github adapter error during %s: %s (status: %d, cause: %v)", e.Operation, e.Message, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("github adapter error during %s: %s (status: %d)", e.Operation, e.Message, e.StatusCode)
}

func NewGitHubError(operation, message string, statusCode int, cause error) *GitHubError {
	return &GitHubError{
		AdapterError: NewAdapterError("github", operation, message, cause),
		StatusCode:   statusCode,
	}
}

type FileSystemError struct {
	*AdapterError
}

func NewFileSystemError(operation, message string, cause error) *FileSystemError {
	return &FileSystemError{
		AdapterError: NewAdapterError("filesystem", operation, message, cause),
	}
}
