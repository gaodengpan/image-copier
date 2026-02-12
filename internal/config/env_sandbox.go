package config

import (
	"os"
)

// EnvSandbox provides utilities for managing environment variables in tests
type EnvSandbox struct {
	originalValues map[string]string
}

// NewEnvSandbox creates a new environment sandbox
func NewEnvSandbox() *EnvSandbox {
	return &EnvSandbox{
		originalValues: make(map[string]string),
	}
}

// SetEnv sets an environment variable and remembers the original value
func (es *EnvSandbox) SetEnv(key, value string) {
	if es.originalValues[key] == "" {
		es.originalValues[key] = os.Getenv(key)
	}
	os.Setenv(key, value)
}

// UnsetEnv unsets an environment variable
func (es *EnvSandbox) UnsetEnv(key string) {
	os.Unsetenv(key)
}

// Restore restores all original environment variable values
func (es *EnvSandbox) Restore() {
	for key, value := range es.originalValues {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}