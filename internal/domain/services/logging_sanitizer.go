package services

import (
	"crypto/sha256"
	"fmt"
)

func SanitizeForLog(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%s%x%s", "[REDACTED:", hash[:8], "]")
}
