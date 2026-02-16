package value_objects

import (
	"fmt"
	"strings"
)

const maxNormalizedLen = 40

type RegistryConfig struct {
	host      string
	namespace string
	arch      string
	os        string
}

func NewRegistryConfig(host, namespace, arch, osType string) *RegistryConfig {
	return &RegistryConfig{
		host:      host,
		namespace: namespace,
		arch:      arch,
		os:        osType,
	}
}

func (r *RegistryConfig) Host() string {
	return r.host
}

func (r *RegistryConfig) Namespace() string {
	return r.namespace
}

func (r *RegistryConfig) Arch() string {
	return r.arch
}

func (r *RegistryConfig) Os() string {
	return r.os
}

func (r *RegistryConfig) BuildDestImageID(sourceID string) string {
	img, err := NewImageID(sourceID)
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

	if r.host == "" {
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

		if r.namespace == "" {
			return fmt.Sprintf("/%s", normalized)
		}
		return fmt.Sprintf("/%s/%s", r.namespace, normalized)
	}

	if r.namespace == "" {
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

		return fmt.Sprintf("%s/%s", r.host, normalized)
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

	return fmt.Sprintf("%s/%s/%s", r.host, r.namespace, normalized)
}
