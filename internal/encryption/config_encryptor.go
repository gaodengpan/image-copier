package encryption

import (
	"os"
)

// ConfigEncryptor handles encryption of configuration values
type ConfigEncryptor struct {
	password string
}

// NewConfigEncryptor creates a new instance of ConfigEncryptor
func NewConfigEncryptor() (*ConfigEncryptor, error) {
	password := os.Getenv("ENCRYPT_KEY")
	if password == "" {
		return nil, &EncryptionError{
			Message: "ENCRYPT_KEY environment variable not set",
		}
	}

	return &ConfigEncryptor{
		password: password,
	}, nil
}

// EncryptValue encrypts a single configuration value
func (ce *ConfigEncryptor) EncryptValue(plaintext string) (string, error) {
	if plaintext == "" {
		return "", &EncryptionError{
			Message: "cannot encrypt empty value",
		}
	}

	return Encrypt(plaintext, ce.password)
}

// EncryptMapValues encrypts all sensitive values in a map
func (ce *ConfigEncryptor) EncryptMapValues(data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range data {
		if strValue, ok := value.(string); ok && IsSensitiveField(key) {
			encrypted, err := ce.EncryptValue(strValue)
			if err != nil {
				return nil, err
			}
			result[key] = encrypted
		} else {
			result[key] = value
		}
	}

	return result, nil
}

// EncryptSliceValues encrypts all sensitive values in a slice
func (ce *ConfigEncryptor) EncryptSliceValues(data []interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(data))

	for i, value := range data {
		if strValue, ok := value.(string); ok {
			// We don't have field names for slices, so we check if the value looks like it should be encrypted
			// For now, we won't encrypt values in slices without context
			result[i] = strValue
		} else {
			result[i] = value
		}
	}

	return result, nil
}

// IsSensitiveField determines if a field should be encrypted based on the field name
func IsSensitiveField(fieldName string) bool {
	switch fieldName {
	case "token", "password", "secret", "key", "access_key", "secret_key", "username":
		return true
	default:
		return false
	}
}
