package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the configuration for image-copier
type Config struct {
	Github struct {
		Owner string `mapstructure:"owner"`
		Repo  string `mapstructure:"repo"`
		Token string `mapstructure:"token"`
		WorkflowID string `mapstructure:"workflow_id"`
	} `mapstructure:"github"`

	Registry struct {
		Host     string `mapstructure:"host"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Namespace string `mapstructure:"namespace"`
		Arch     string `mapstructure:"arch"`
		Os       string `mapstructure:"os"`
	} `mapstructure:"registry"`

	Retry struct {
		MaxAttempts     string `mapstructure:"max_attempts"`
		InitialInterval string `mapstructure:"initial_interval"`
		MaxInterval     string `mapstructure:"max_interval"`
	} `mapstructure:"retry"`

	LogLevel string `mapstructure:"log_level"`
}

// configDir returns the XDG-compliant configuration directory.
func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "image-copier")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "image-copier")
}

// ConfigFilePath returns the full path to the config file.
func ConfigFilePath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// Load loads configuration from file or environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir())
	viper.AddConfigPath(".")

	// Set default values
	viper.SetDefault("registry.arch", "amd64")
	viper.SetDefault("registry.os", "linux")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("github.workflow_id", "image-copier-v2.yaml")

	// Bind environment variables
	viper.BindEnv("github.owner", "GITHUB_OWNER")
	viper.BindEnv("github.repo", "GITHUB_REPO")
	viper.BindEnv("github.token", "GITHUB_TOKEN")
	viper.BindEnv("registry.host", "REGISTRY_HOST")
	viper.BindEnv("registry.username", "REGISTRY_USERNAME")
	viper.BindEnv("registry.password", "REGISTRY_PASSWD")
	viper.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	viper.BindEnv("registry.arch", "REGISTRY_ARCH")
	viper.BindEnv("registry.os", "REGISTRY_OS")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; fallback to environment variables only
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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

func validateConfig(cfg *Config) error {
	return ValidateConfig(cfg)
}

// GetConfigPath returns the path to the config file if it exists.
func GetConfigPath() string {
	p := ConfigFilePath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
