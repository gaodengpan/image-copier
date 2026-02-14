#!/bin/bash

# Script to update AI agent context with new feature information
# Usage: .specify/scripts/bash/update-agent-context.sh <agent-type>

AGENT_TYPE=$1

case $AGENT_TYPE in
  "claude")
    echo "Updating Claude AI agent context with secure config management feature details..."
    # In a real implementation, this would update the specific agent context file
    # For now, we'll create a placeholder to indicate the context was updated
    mkdir -p .specify/contexts
    cat > .specify/contexts/claude-context.md << 'EOL'
# AI Agent Context - Secure Config Management Feature

## Feature Overview
- Feature: Secure Configuration Management
- Purpose: Encrypt sensitive configuration values (tokens, passwords) and auto-decrypt at runtime
- Algorithm: AES-256-GCM
- Key derivation: PBKDF2
- Key source: Environment variable (ENCRYPT_KEY)
- Encrypted value format: "encrypted:<base64_encoded_data>"
- Data structure: [12-byte nonce][ciphertext][16-byte auth tag]

## Technical Details
- Encryption: AES-256-GCM with random nonce per value
- Key derivation: PBKDF2 with configurable iterations
- Error handling: Fail-fast with clear error messages on decryption failure
- Performance: <100ms for decryption operations
- Cross-platform: Consistent behavior across Windows, macOS, Linux

## Integration Points
- Modifies: internal/config/config.go
- New packages: internal/encryption
- Configuration format remains compatible with existing structure
- Only sensitive values are encrypted, others remain in plain text

## API Contract
- Encrypt(value string, key []byte) (string, error)
- Decrypt(encryptedValue string, key []byte) (string, error)
- DeriveKey(password string, salt []byte) []byte
- DecryptConfig(config *Config) (*Config, error)
EOL
    echo "Claude AI agent context updated successfully."
    ;;
  *)
    echo "Unsupported agent type: $AGENT_TYPE"
    echo "Supported types: claude"
    exit 1
    ;;
esac