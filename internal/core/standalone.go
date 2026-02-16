package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/gaodengpan/image-copier/internal/domain/validators"
)

var imageValidator = validators.NewImageValidator()

func createTempFile() (string, error) {
	tmpFile, err := os.CreateTemp("", "image-copier-*.tar")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	return tmpPath, nil
}

func parseDockerImageOutput(output string, maxCacheSize int) map[string]bool {
	images := make(map[string]bool)
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && imageValidator.ValidateImageNameInput(line) && count < maxCacheSize {
			images[line] = true
			count++
		}
	}
	return images
}

func normalizeImageSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		return fmt.Sprintf("docker.io/%s", segment)
	}
	return segment
}

func hasTagOrDigest(s string) bool {
	if s == "" {
		return false
	}

	parts := strings.Split(s, "/")
	tailSegment := parts[len(parts)-1]

	if strings.Contains(tailSegment, "@") {
		return true
	}

	colonParts := strings.Split(tailSegment, ":")

	if len(colonParts) > 2 {
		return true
	}

	if len(colonParts) == 2 {
		return true
	}

	return false
}
