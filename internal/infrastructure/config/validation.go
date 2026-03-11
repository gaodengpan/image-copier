package config

import "fmt"

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field string
	Value string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s: invalid value %s", e.Field, e.Value)
}

// ValidateConfig validates the configuration
func ValidateConfig(cfg *Config) error {
	if cfg.Github.Owner == "" {
		return fmt.Errorf("github owner is required")
	}
	if cfg.Github.Repo == "" {
		return fmt.Errorf("github repo is required")
	}
	if cfg.Github.Token == "" {
		return fmt.Errorf("github token is required")
	}
	if cfg.Registry.Host == "" {
		return fmt.Errorf("registry host is required")
	}
	if cfg.Registry.Username == "" {
		return fmt.Errorf("registry username is required")
	}
	if cfg.Registry.Password == "" {
		return fmt.Errorf("registry password is required")
	}
	return nil
}
