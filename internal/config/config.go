package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/spf13/viper"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field string
	Value string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s: invalid value %s", e.Field, e.Value)
}

// Config holds the configuration for image-copier
type Config struct {
	Github struct {
		Owner      string `mapstructure:"owner"`
		Repo       string `mapstructure:"repo"`
		Token      string `mapstructure:"token"`
		WorkflowID string `mapstructure:"workflow_id"`
	} `mapstructure:"github"`

	Registry struct {
		Host      string `mapstructure:"host"`
		Username  string `mapstructure:"username"`
		Password  string `mapstructure:"password"`
		Namespace string `mapstructure:"namespace"`
		Arch      string `mapstructure:"arch"`
		Os        string `mapstructure:"os"`
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

// LoadForTestingWithEnv loads configuration with environment variables for testing
func LoadForTestingWithEnv(env map[string]string) (*Config, error) {
	// Create a new viper instance for better test isolation
	v := viper.New()

	// Temporarily set environment variables
	originalEnv := make(map[string]string)
	for k, v := range env {
		originalEnv[k] = os.Getenv(k)
		os.Setenv(k, v)
	}

	// Defer restoring original environment values
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set default values
	v.SetDefault("registry.arch", "amd64")
	v.SetDefault("registry.os", "linux")
	v.SetDefault("log_level", "info")
	v.SetDefault("github.workflow_id", "image-copier-v2.yaml")

	// Bind environment variables
	v.BindEnv("github.owner", "GITHUB_OWNER")
	v.BindEnv("github.repo", "GITHUB_REPO")
	v.BindEnv("github.token", "GITHUB_TOKEN")
	v.BindEnv("github.workflow_id", "GITHUB_WORKFLOW_ID")
	v.BindEnv("registry.host", "REGISTRY_HOST")
	v.BindEnv("registry.username", "REGISTRY_USERNAME")
	v.BindEnv("registry.password", "REGISTRY_PASSWD")
	v.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	v.BindEnv("registry.arch", "REGISTRY_ARCH")
	v.BindEnv("registry.os", "REGISTRY_OS")
	v.BindEnv("log_level", "LOG_LEVEL")

	// Do NOT read any config file - only use environment variables and defaults
	// This ensures complete test isolation

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadForTesting loads configuration with better test isolation
func LoadForTesting() (*Config, error) {
	// Create a new viper instance for better test isolation
	v := viper.New()

	// Set default values
	v.SetDefault("registry.arch", "amd64")
	v.SetDefault("registry.os", "linux")
	v.SetDefault("log_level", "info")
	v.SetDefault("github.workflow_id", "image-copier-v2.yaml")

	// Bind environment variables
	v.BindEnv("github.owner", "GITHUB_OWNER")
	v.BindEnv("github.repo", "GITHUB_REPO")
	v.BindEnv("github.token", "GITHUB_TOKEN")
	v.BindEnv("github.workflow_id", "GITHUB_WORKFLOW_ID")
	v.BindEnv("registry.host", "REGISTRY_HOST")
	v.BindEnv("registry.username", "REGISTRY_USERNAME")
	v.BindEnv("registry.password", "REGISTRY_PASSWD")
	v.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	v.BindEnv("registry.arch", "REGISTRY_ARCH")
	v.BindEnv("registry.os", "REGISTRY_OS")
	v.BindEnv("log_level", "LOG_LEVEL")

	// Do NOT read any config file - only use environment variables and defaults
	// This ensures complete test isolation

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
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

// ParseRetryConfig converts string-based Retry config fields into a typed *retry.Config.
// Empty or invalid fields fall back to retry.DefaultConfig() values.
func (c *Config) ParseRetryConfig() *retry.Config {
	defaults := retry.DefaultConfig()

	maxAttempts := defaults.MaxAttempts
	if c.Retry.MaxAttempts != "" {
		if v, err := strconv.Atoi(c.Retry.MaxAttempts); err == nil && v > 0 {
			maxAttempts = v
		}
	}

	initialInterval := defaults.InitialInterval
	if c.Retry.InitialInterval != "" {
		if v, err := time.ParseDuration(c.Retry.InitialInterval); err == nil && v > 0 {
			initialInterval = v
		}
	}

	maxInterval := defaults.MaxInterval
	if c.Retry.MaxInterval != "" {
		if v, err := time.ParseDuration(c.Retry.MaxInterval); err == nil && v > 0 {
			maxInterval = v
		}
	}

	return &retry.Config{
		MaxAttempts:     maxAttempts,
		InitialInterval: initialInterval,
		MaxInterval:     maxInterval,
	}
}

// GetConfigPath returns the path to the config file if it exists.
func GetConfigPath() string {
	provider := NewViperConfigProvider()
	return provider.GetConfigPath()
}

// DefaultRetryConfig returns the default retry configuration values
func DefaultRetryConfig() *retry.Config {
	return retry.DefaultConfig()
}

// ConfigBuilder provides a fluent interface for building Config instances
type ConfigBuilder struct {
	config *Config
}

// NewConfigBuilder creates a new ConfigBuilder instance
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &Config{},
	}
}

// WithGithubOwner sets the GitHub owner
func (cb *ConfigBuilder) WithGithubOwner(owner string) *ConfigBuilder {
	cb.config.Github.Owner = owner
	return cb
}

// WithGithubRepo sets the GitHub repository
func (cb *ConfigBuilder) WithGithubRepo(repo string) *ConfigBuilder {
	cb.config.Github.Repo = repo
	return cb
}

// WithGithubToken sets the GitHub token
func (cb *ConfigBuilder) WithGithubToken(token string) *ConfigBuilder {
	cb.config.Github.Token = token
	return cb
}

// WithGithubWorkflowID sets the GitHub workflow ID
func (cb *ConfigBuilder) WithGithubWorkflowID(workflowID string) *ConfigBuilder {
	cb.config.Github.WorkflowID = workflowID
	return cb
}

// WithRegistryHost sets the registry host
func (cb *ConfigBuilder) WithRegistryHost(host string) *ConfigBuilder {
	cb.config.Registry.Host = host
	return cb
}

// WithRegistryUsername sets the registry username
func (cb *ConfigBuilder) WithRegistryUsername(username string) *ConfigBuilder {
	cb.config.Registry.Username = username
	return cb
}

// WithRegistryPassword sets the registry password
func (cb *ConfigBuilder) WithRegistryPassword(password string) *ConfigBuilder {
	cb.config.Registry.Password = password
	return cb
}

// WithRegistryNamespace sets the registry namespace
func (cb *ConfigBuilder) WithRegistryNamespace(namespace string) *ConfigBuilder {
	cb.config.Registry.Namespace = namespace
	return cb
}

// WithRegistryArch sets the registry architecture
func (cb *ConfigBuilder) WithRegistryArch(arch string) *ConfigBuilder {
	cb.config.Registry.Arch = arch
	return cb
}

// WithRegistryOs sets the registry OS
func (cb *ConfigBuilder) WithRegistryOs(os string) *ConfigBuilder {
	cb.config.Registry.Os = os
	return cb
}

// WithLogLevel sets the log level
func (cb *ConfigBuilder) WithLogLevel(logLevel string) *ConfigBuilder {
	cb.config.LogLevel = logLevel
	return cb
}

// WithRetryMaxAttempts sets the retry max attempts
func (cb *ConfigBuilder) WithRetryMaxAttempts(maxAttempts string) *ConfigBuilder {
	cb.config.Retry.MaxAttempts = maxAttempts
	return cb
}

// WithRetryInitialInterval sets the retry initial interval
func (cb *ConfigBuilder) WithRetryInitialInterval(interval string) *ConfigBuilder {
	cb.config.Retry.InitialInterval = interval
	return cb
}

// WithRetryMaxInterval sets the retry max interval
func (cb *ConfigBuilder) WithRetryMaxInterval(interval string) *ConfigBuilder {
	cb.config.Retry.MaxInterval = interval
	return cb
}

// WithRetryConfig sets the entire retry configuration
func (cb *ConfigBuilder) WithRetryConfig(retryMaxAttempts, retryInitialInterval, retryMaxInterval string) *ConfigBuilder {
	cb.config.Retry.MaxAttempts = retryMaxAttempts
	cb.config.Retry.InitialInterval = retryInitialInterval
	cb.config.Retry.MaxInterval = retryMaxInterval
	return cb
}

// Build returns the constructed Config
func (cb *ConfigBuilder) Build() *Config {
	return cb.config
}

// ConfigProvider interface to abstract configuration loading
type ConfigProvider interface {
	Load() (*Config, error)
	GetConfigPath() string
}

// ViperConfigProvider implementation that maintains current functionality
type ViperConfigProvider struct {
	viper *viper.Viper
}

// ConfigManager provides a centralized way to handle application configuration
// This implements the configuration management abstraction goal
type ConfigManager struct {
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

	// Bind environment variables
	vp.viper.BindEnv("github.owner", "GITHUB_OWNER")
	vp.viper.BindEnv("github.repo", "GITHUB_REPO")
	vp.viper.BindEnv("github.token", "GITHUB_TOKEN")
	vp.viper.BindEnv("github.workflow_id", "GITHUB_WORKFLOW_ID")
	vp.viper.BindEnv("registry.host", "REGISTRY_HOST")
	vp.viper.BindEnv("registry.username", "REGISTRY_USERNAME")
	vp.viper.BindEnv("registry.password", "REGISTRY_PASSWD")
	vp.viper.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	vp.viper.BindEnv("registry.arch", "REGISTRY_ARCH")
	vp.viper.BindEnv("registry.os", "REGISTRY_OS")
	vp.viper.BindEnv("log_level", "LOG_LEVEL")

	vp.viper.SetConfigType("yaml")
	vp.viper.SetConfigName("config")
	vp.viper.AddConfigPath(".")         // current directory
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
	if err := validateConfig(&cfg); err != nil {
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
		// Only add current directory to avoid global config interference
		vp.viper.SetConfigName("config")
		vp.viper.AddConfigPath(".")
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
	if err := validateConfig(&cfg); err != nil {
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

// LoadConfiguration loads the configuration from file
func (cm *ConfigManager) LoadConfiguration(configPath string) error {
	if configPath != "" {
		// Set the config file
		configDir := filepath.Dir(configPath)
		configName := filepath.Base(configPath)

		// Remove extension to get config name
		ext := filepath.Ext(configName)
		configName = configName[:len(configName)-len(ext)]

		cm.viper.SetConfigName(configName)
		cm.viper.AddConfigPath(configDir)
	} else {
		// Set default config paths
		cm.viper.SetConfigName("config")
		cm.viper.AddConfigPath(".")                          // current directory
		cm.viper.AddConfigPath("$HOME/.config/image-copier") // user's config directory
		cm.viper.AddConfigPath("/etc/image-copier/")         // system-wide config
	}

	if err := cm.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, but that's okay - we'll rely on env vars and defaults
	}

	return nil
}

// GetConfig returns the loaded configuration as a Config struct
func (cm *ConfigManager) GetConfig() (*Config, error) {
	var config Config
	err := cm.viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode configuration into struct: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadWithConfigPath loads configuration from a specific file path
func LoadWithConfigPath(configPath string) (*Config, error) {
	provider := NewViperConfigProvider()
	return provider.LoadWithPaths(configPath)
}

// DefaultConfigProvider returns a ConfigProvider with default configuration
func DefaultConfigProvider() ConfigProvider {
	return NewViperConfigProvider()
}
