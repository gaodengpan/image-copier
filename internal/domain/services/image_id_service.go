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
