package services

import "strings"

type ImageNormalizer struct{}

func NewImageNormalizer() *ImageNormalizer {
	return &ImageNormalizer{}
}

func (n *ImageNormalizer) Normalize(sourceID string) string {
	return normalize(sourceID)
}

func Normalize(sourceID string) string {
	return normalize(sourceID)
}

func normalize(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		normalized = "docker.io/library/" + imageID
	case 2:
		normalized = normalizeSegment(segs[0]) + "/" + segs[1]
	default:
		normalized = imageID
	}

	lastSlash := strings.LastIndex(normalized, "/")
	tail := normalized
	if lastSlash >= 0 {
		tail = normalized[lastSlash+1:]
	}
	if !hasTagOrDigest(tail) {
		normalized += ":latest"
	}

	return normalized
}

func normalizeSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		return "docker.io/" + segment
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
