package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gaodengpan/image-copier/internal/infrastructure/encryption"
	"github.com/spf13/viper"
)

// EncryptedViperConfigProvider extends ViperConfigProvider with decryption capabilities
type EncryptedViperConfigProvider struct {
	viper *viper.Viper
}

// NewEncryptedViperConfigProvider creates a new instance of EncryptedViperConfigProvider
func NewEncryptedViperConfigProvider() *EncryptedViperConfigProvider {
	v := viper.New()

	return &EncryptedViperConfigProvider{
		viper: v,
	}
}

// Load loads configuration and decrypts encrypted values
func (evp *EncryptedViperConfigProvider) Load() (*Config, error) {
	// Set default values
	evp.viper.SetDefault("registry.arch", "amd64")
	evp.viper.SetDefault("registry.os", "linux")
	evp.viper.SetDefault("log_level", "info")
	evp.viper.SetDefault("github.workflow_id", "image-copier-v2.yaml")

	// Bind environment variables (errors are ignored as BindEnv typically succeeds)
	_ = evp.viper.BindEnv("github.owner", "GITHUB_OWNER")
	_ = evp.viper.BindEnv("github.repo", "GITHUB_REPO")
	_ = evp.viper.BindEnv("github.token", "GITHUB_TOKEN")
	_ = evp.viper.BindEnv("github.workflow_id", "GITHUB_WORKFLOW_ID")
	_ = evp.viper.BindEnv("registry.host", "REGISTRY_HOST")
	_ = evp.viper.BindEnv("registry.username", "REGISTRY_USERNAME")
	_ = evp.viper.BindEnv("registry.password", "REGISTRY_PASSWD")
	_ = evp.viper.BindEnv("registry.namespace", "REGISTRY_NAMESPACE")
	_ = evp.viper.BindEnv("registry.arch", "REGISTRY_ARCH")
	_ = evp.viper.BindEnv("registry.os", "REGISTRY_OS")
	_ = evp.viper.BindEnv("log_level", "LOG_LEVEL")

	evp.viper.SetConfigType("yaml")
	evp.viper.SetConfigName("config")
	evp.viper.AddConfigPath(".")         // current directory
	evp.viper.AddConfigPath(configDir()) // user's config directory

	// Read config file
	if err := evp.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := evp.viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Decrypt sensitive values if they are encrypted
	decryptedCfg, err := evp.decryptConfig(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Validate required fields
	if err := ValidateConfig(decryptedCfg); err != nil {
		return nil, err
	}

	return decryptedCfg, nil
}

// generateDecryptionDiagnostics creates a detailed diagnostic message for decryption failures
// without exposing sensitive data
func generateDecryptionDiagnostics(field, value string) string {
	var diagnostics []string

	// Check if ENCRYPT_KEY is set
	if os.Getenv("ENCRYPT_KEY") == "" {
		diagnostics = append(diagnostics, "ENCRYPT_KEY environment variable is not set")
	} else {
		diagnostics = append(diagnostics, "ENCRYPT_KEY is set")
	}

	// Check value format without exposing content
	if value == "" {
		diagnostics = append(diagnostics, "field value is empty")
	} else if encryption.IsEncryptedValue(value) {
		diagnostics = append(diagnostics, "field value is in encrypted format")
	} else {
		diagnostics = append(diagnostics, "field value is in plaintext format")
	}

	// Add config file location
	if configPath := GetConfigPath(); configPath != "" {
		diagnostics = append(diagnostics, fmt.Sprintf("config file: %s", configPath))
	}

	return fmt.Sprintf("decryption failed for field '%s'. Diagnostics: %s. "+
		"Please verify ENCRYPT_KEY matches the key used for encryption. "+
		"If using plaintext, ensure the value doesn't start with 'encrypted:'",
		field, strings.Join(diagnostics, "; "))
}

// decryptConfig decrypts sensitive fields in the configuration
func (evp *EncryptedViperConfigProvider) decryptConfig(cfg *Config) (*Config, error) {
	decryptor := encryption.NewConfigDecryptor()

	// Decrypt GitHub token if it's encrypted
	decryptedToken, err := decryptor.DecryptValue(cfg.Github.Token)
	if err != nil {
		return nil, &encryption.DecryptionError{
			Message: generateDecryptionDiagnostics("github.token", cfg.Github.Token),
			Field:   "github.token",
			Cause:   err,
		}
	}
	cfg.Github.Token = decryptedToken

	// Decrypt registry username if it's encrypted
	decryptedUsername, err := decryptor.DecryptValue(cfg.Registry.Username)
	if err != nil {
		return nil, &encryption.DecryptionError{
			Message: generateDecryptionDiagnostics("registry.username", cfg.Registry.Username),
			Field:   "registry.username",
			Cause:   err,
		}
	}
	cfg.Registry.Username = decryptedUsername

	// Decrypt registry password if it's encrypted
	decryptedPassword, err := decryptor.DecryptValue(cfg.Registry.Password)
	if err != nil {
		return nil, &encryption.DecryptionError{
			Message: generateDecryptionDiagnostics("registry.password", cfg.Registry.Password),
			Field:   "registry.password",
			Cause:   err,
		}
	}
	cfg.Registry.Password = decryptedPassword

	return cfg, nil
}

// LoadWithPaths loads configuration with specific paths for testing
func (evp *EncryptedViperConfigProvider) LoadWithPaths(configPath string) (*Config, error) {
	// Set config paths
	evp.viper.SetConfigType("yaml")
	if configPath != "" {
		dir := filepath.Dir(configPath)
		filename := filepath.Base(configPath)
		ext := filepath.Ext(filename)

		// Remove extension to get the config name
		configName := filename[:len(filename)-len(ext)]

		evp.viper.SetConfigName(configName)
		evp.viper.AddConfigPath(dir)
	} else {
		// Only add current directory to avoid global config interference
		evp.viper.SetConfigName("config")
		evp.viper.AddConfigPath(".")
	}

	// Read config file
	if err := evp.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; fallback to environment variables only
	}

	var cfg Config
	if err := evp.viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Decrypt sensitive values if they are encrypted
	decryptedCfg, err := evp.decryptConfig(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Validate required fields
	if err := ValidateConfig(decryptedCfg); err != nil {
		return nil, err
	}

	return decryptedCfg, nil
}

// GetConfigPath returns the path to the config file if it exists.
func (evp *EncryptedViperConfigProvider) GetConfigPath() string {
	p := ConfigFilePath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// LoadEncryptedConfig loads configuration with decryption support
func LoadEncryptedConfig() (*Config, error) {
	provider := NewEncryptedViperConfigProvider()
	return provider.Load()
}

// SafeLoadEncryptedConfig loads configuration with enhanced error handling
// It returns a detailed error if decryption fails
func SafeLoadEncryptedConfig() (*Config, error) {
	cfg, err := LoadEncryptedConfig()
	if err != nil {
		// If it's a decryption error, provide additional guidance
		if de, ok := err.(*encryption.DecryptionError); ok {
			// Check if the error is due to missing encryption key
			envKey := os.Getenv("ENCRYPT_KEY")
			if envKey == "" {
				// If the ENCRYPT_KEY is not set, try to load with a plain config provider
				plainProvider := NewViperConfigProvider()
				if plainCfg, plainErr := plainProvider.Load(); plainErr == nil {
					return plainCfg, nil // Return plain config if decryption fails due to missing key
				}
			}
			return nil, fmt.Errorf("%s - Please ensure the ENCRYPT_KEY environment variable is correctly set and matches the key used for encryption", de.Error())
		}
		return nil, err
	}
	return cfg, nil
}
