package config

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

// WithPrivateRegistry adds a private registry to the configuration
func (cb *ConfigBuilder) WithPrivateRegistry(registry PrivateRegistry) *ConfigBuilder {
	cb.config.PrivateRegistries = append(cb.config.PrivateRegistries, registry)
	return cb
}

// WithPrivateRegistries sets all private registries
func (cb *ConfigBuilder) WithPrivateRegistries(registries []PrivateRegistry) *ConfigBuilder {
	cb.config.PrivateRegistries = registries
	return cb
}

// WithDistribution sets the distribution configuration
func (cb *ConfigBuilder) WithDistribution(defaultTargets []string) *ConfigBuilder {
	cb.config.Distribution.DefaultTargets = defaultTargets
	return cb
}

// Build returns the constructed Config
func (cb *ConfigBuilder) Build() *Config {
	return cb.config
}