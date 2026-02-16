package services

import (
	"crypto/sha256"
	"fmt"
)

const (
	sensitiveDataPrefix = "[REDACTED:"
	sensitiveDataSuffix = "]"
)

type Sanitizer struct{}

func NewSanitizer() *Sanitizer {
	return &Sanitizer{}
}

func (s *Sanitizer) SanitizeForLog(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%s%x%s", sensitiveDataPrefix, hash[:8], sensitiveDataSuffix)
}

func SanitizeForLog(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%s%x%s", sensitiveDataPrefix, hash[:8], sensitiveDataSuffix)
}
