package encryption

// ConfigDecryptor handles decryption of configuration values
type ConfigDecryptor struct{}

// NewConfigDecryptor creates a new instance of ConfigDecryptor
func NewConfigDecryptor() *ConfigDecryptor {
	return &ConfigDecryptor{}
}

// DecryptValue decrypts a single configuration value if it's encrypted
func (cd *ConfigDecryptor) DecryptValue(encryptedValue string) (string, error) {
	if !IsEncryptedValue(encryptedValue) {
		// If the value is not encrypted, return as is
		return encryptedValue, nil
	}

	return Decrypt(encryptedValue)
}
