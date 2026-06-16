package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

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

// GetConfigPath returns the path to the config file if it exists.
func GetConfigPath() string {
	provider := NewViperConfigProvider()
	return provider.GetConfigPath()
}

// ConfigProvider interface to abstract configuration loading
type ConfigProvider interface {
	Load() (*Config, error)
	GetConfigPath() string
}

// DefaultConfigProvider returns a ConfigProvider with default configuration
func DefaultConfigProvider() ConfigProvider {
	return NewEncryptedViperConfigProvider()
}

// ViperConfigProvider implementation that maintains current functionality
type ViperConfigProvider struct {
	viper *viper.Viper
}

// NewViperConfigProvider creates a new instance of ViperConfigProvider
func NewViperConfigProvider() *ViperConfigProvider {
	v := viper.New()

	return &ViperConfigProvider{
		viper: v,
	}
}

// Load loads configuration from environment variables only (no file loading)
func (vp *ViperConfigProvider) Load() (*Config, error) {
	// Set default values
	vp.viper.SetDefault("registry.arch", "amd64")
	vp.viper.SetDefault("registry.os", "linux")
	vp.viper.SetDefault("log_level", "info")
	vp.viper.SetDefault("github.workflow_id", "image-copier-v2.yaml")

	// Bind environment variables (errors are ignored as BindEnv typically succeeds)
	_ = vp.viper.BindEnv("github.owner", "GITHUB_OWNER")
	_ = vp.viper.BindEnv("github.repo", "GITHUB_REPO")
	_ = vp.viper.BindEnv("github.token", "GITHUB_TOKEN")
	_ = vp.viper.BindEnv("github.workflow_id", "GITHUB_WORKFLOW_ID")
	_ = vp.viper.BindEnv("registry.host", "REGISTRY_HOST")
	_ = vp.viper.BindEnv("registry.username", "REGISTRY_USERNAME")
	_ = vp.viper.BindEnv("registry.password", "REGISTRY_PASSWD")
	_ = vp.viper.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	_ = vp.viper.BindEnv("registry.arch", "REGISTRY_ARCH")
	_ = vp.viper.BindEnv("registry.os", "REGISTRY_OS")
	_ = vp.viper.BindEnv("log_level", "LOG_LEVEL")

	vp.viper.SetConfigType("yaml")
	vp.viper.SetConfigName("config")
	vp.viper.AddConfigPath(configDir()) // user's config directory

	// Read config file
	if err := vp.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := vp.viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadWithPaths loads configuration with specific paths for testing
func (vp *ViperConfigProvider) LoadWithPaths(configPath string) (*Config, error) {
	// Set config paths
	vp.viper.SetConfigType("yaml")
	if configPath != "" {
		dir := filepath.Dir(configPath)
		filename := filepath.Base(configPath)
		ext := filepath.Ext(filename)

		// Remove extension to get the config name
		configName := filename[:len(filename)-len(ext)]

		vp.viper.SetConfigName(configName)
		vp.viper.AddConfigPath(dir)
	} else {
		// Only use user config directory to avoid loading from arbitrary paths
		vp.viper.SetConfigName("config")
		vp.viper.AddConfigPath(configDir())
	}

	// Read config file
	if err := vp.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; fallback to environment variables only
	}

	var cfg Config
	if err := vp.viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetConfigPath returns the path to the config file if it exists.
func (vp *ViperConfigProvider) GetConfigPath() string {
	p := ConfigFilePath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
