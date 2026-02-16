package services

import (
	"fmt"
	"strings"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

const maxNormalizedLen = 40

type ImageBuilder struct{}

func NewImageBuilder() *ImageBuilder {
	return &ImageBuilder{}
}

func (b *ImageBuilder) BuildDestImageID(registryHost, registryNamespace, sourceID string) string {
	img, err := value_objects.NewImageID(sourceID)
	if err != nil {
		return ""
	}

	sourceNormalized := img.String()

	var tag, digest, imageName string

	digestIndex := strings.LastIndex(sourceNormalized, "@")
	if digestIndex != -1 {
		digest = sourceNormalized[digestIndex:]
		imageName = sourceNormalized[:digestIndex]
	} else {
		imageName = sourceNormalized
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
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		maxBaseLen := maxNormalizedLen
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
		normalized = normalized + tag + digest

		if registryNamespace == "" {
			return fmt.Sprintf("/%s", normalized)
		}
		return fmt.Sprintf("/%s/%s", registryNamespace, normalized)
	}

	if registryNamespace == "" {
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		maxBaseLen := maxNormalizedLen
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
		normalized = normalized + tag + digest

		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	normalized := strings.ReplaceAll(imageName, "/", "_")
	normalized = strings.ReplaceAll(normalized, ":", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	maxBaseLen := maxNormalizedLen
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
	normalized = normalized + tag + digest

	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}
