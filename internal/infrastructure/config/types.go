package config

import (
	"strconv"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
)

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

	// Timeout configuration
	Timeout TimeoutConfig `mapstructure:"timeout"`

	PrivateRegistries []PrivateRegistry `mapstructure:"private_registries"`

	// Distribution configuration
	Distribution Distribution `mapstructure:"distribution"`

	LogLevel string `mapstructure:"log_level"`
	Force    bool
	DryRun   bool
}

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	// ImageCheck is the timeout for checking image existence (default: 30s)
	ImageCheck string `mapstructure:"image_check"`
	// SyncOperation is the timeout for sync/copy operations (default: 10m)
	SyncOperation string `mapstructure:"sync_operation"`
	// WorkflowPoll is the interval for polling GitHub workflow status (default: 2s)
	WorkflowPoll string `mapstructure:"workflow_poll"`
	// WorkflowMaxPoll is the maximum number of polls for workflow status (default: 300)
	WorkflowMaxPoll string `mapstructure:"workflow_max_poll"`
}

// DefaultTimeoutConfig returns the default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ImageCheck:      "30s",
		SyncOperation:   "10m",
		WorkflowPoll:    "2s",
		WorkflowMaxPoll: "300",
	}
}

// ParseImageCheck returns the parsed image check timeout
func (t *TimeoutConfig) ParseImageCheck() time.Duration {
	if t.ImageCheck == "" {
		return 30 * time.Second
	}
	if d, err := time.ParseDuration(t.ImageCheck); err == nil {
		return d
	}
	return 30 * time.Second
}

// ParseSyncOperation returns the parsed sync operation timeout
func (t *TimeoutConfig) ParseSyncOperation() time.Duration {
	if t.SyncOperation == "" {
		return 10 * time.Minute
	}
	if d, err := time.ParseDuration(t.SyncOperation); err == nil {
		return d
	}
	return 10 * time.Minute
}

// ParseWorkflowPoll returns the parsed workflow poll interval
func (t *TimeoutConfig) ParseWorkflowPoll() time.Duration {
	if t.WorkflowPoll == "" {
		return 2 * time.Second
	}
	if d, err := time.ParseDuration(t.WorkflowPoll); err == nil {
		return d
	}
	return 2 * time.Second
}

// ParseWorkflowMaxPoll returns the parsed maximum poll count
func (t *TimeoutConfig) ParseWorkflowMaxPoll() int {
	if t.WorkflowMaxPoll == "" {
		return 300
	}
	if v, err := strconv.Atoi(t.WorkflowMaxPoll); err == nil && v > 0 {
		return v
	}
	return 300
}

// Distribution holds distribution target configuration
type Distribution struct {
	DefaultTargets []string `mapstructure:"default_targets"`
}

// PrivateRegistry holds configuration for a private registry
type PrivateRegistry struct {
	Name       string `mapstructure:"name"`
	Host       string `mapstructure:"host"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Insecure   bool   `mapstructure:"insecure"`
	MirrorMode bool   `mapstructure:"mirror_mode"` // Keep original image names for mirror registries
}

// GetPrivateRegistryByName returns a private registry by name
func (c *Config) GetPrivateRegistryByName(name string) *PrivateRegistry {
	for i := range c.PrivateRegistries {
		if c.PrivateRegistries[i].Name == name {
			return &c.PrivateRegistries[i]
		}
	}
	return nil
}

// GetDistributionTargets returns the list of distribution targets
// If the provided targets list is not empty, it returns that list.
// Otherwise, it returns the default targets from the configuration.
func (c *Config) GetDistributionTargets(targets []string) []string {
	if len(targets) > 0 {
		return targets
	}
	return c.Distribution.DefaultTargets
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

// IsDockerTarget returns true if the target name is "docker"
func IsDockerTarget(targetName string) bool {
	return targetName == "docker"
}
