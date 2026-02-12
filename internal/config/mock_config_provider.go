package config

// MockConfigProvider is a mock implementation of ConfigProvider for testing
type MockConfigProvider struct {
	ConfigToReturn *Config
	ErrorToReturn  error
}

// Load returns the configured config or error for testing
func (m *MockConfigProvider) Load() (*Config, error) {
	return m.ConfigToReturn, m.ErrorToReturn
}

// GetConfigPath returns an empty string for testing
func (m *MockConfigProvider) GetConfigPath() string {
	return ""
}