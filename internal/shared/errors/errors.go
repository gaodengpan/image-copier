package errors

import "fmt"

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
