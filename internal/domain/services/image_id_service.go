package services

import (
	"fmt"
	"strings"
)

const maxDestImageIDLen = 50

type ImageIDService struct{}

func NewImageIDService() *ImageIDService {
	return &ImageIDService{}
}

func (s *ImageIDService) BuildDestImageID(sourceID, registryHost, registryNamespace string) string {
	var tag, digest, imageName string

	digestIndex := strings.LastIndex(sourceID, "@")
	if digestIndex != -1 {
		digest = sourceID[digestIndex:]
		imageName = sourceID[:digestIndex]
	} else {
		imageName = sourceID
	}

	if digestIndex == -1 {
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:]
			imageName = imageName[:tagIndex]
		}
	} else {
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:]
			imageName = imageName[:tagIndex]
		}
	}

	if registryHost == "" {
		return s.normalizeImageName(imageName, tag, digest)
	}

	if registryNamespace == "" {
		normalized := s.normalizeImageName(imageName, tag, digest)
		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	normalized := s.normalizeImageName(imageName, tag, digest)
	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}

func (s *ImageIDService) normalizeImageName(imageName, tag, digest string) string {
	normalized := strings.ReplaceAll(imageName, "/", "_")
	normalized = strings.ReplaceAll(normalized, ":", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	maxBaseLen := maxDestImageIDLen
	if tag != "" {
		maxBaseLen -= len(tag)
	}
	if digest != "" {
		maxBaseLen -= len(digest)
	}
	if maxBaseLen < 0 {
		maxBaseLen = 0
	}
	if len(normalized) > maxBaseLen {
		normalized = normalized[:maxBaseLen]
	}

	normalized = strings.TrimRight(normalized, "_")
	return normalized + tag + digest
}

func (s *ImageIDService) NormalizeSourceID(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		normalized = fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		normalized = s.normalizeImageSegment(segs[0]) + "/" + segs[1]
	default:
		normalized = imageID
	}

	lastSlash := strings.LastIndex(normalized, "/")
	tail := normalized
	if lastSlash >= 0 {
		tail = normalized[lastSlash+1:]
	}
	if !s.hasTagOrDigest(tail) {
		normalized += ":latest"
	}

	return normalized
}

func (s *ImageIDService) normalizeImageSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		return "docker.io/" + segment
	}
	return segment
}

func (s *ImageIDService) hasTagOrDigest(str string) bool {
	if str == "" {
		return false
	}
	parts := strings.Split(str, "/")
	tailSegment := parts[len(parts)-1]
	if strings.Contains(tailSegment, "@") {
		return true
	}
	colonParts := strings.Split(tailSegment, ":")
	if len(colonParts) > 2 || len(colonParts) == 2 {
		return true
	}
	return false
}
