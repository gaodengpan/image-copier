package value_objects

import (
	"fmt"
	"strings"
)

type RegistryEndpoint struct {
	host      string
	namespace string
	arch      string
	os        string
}

func NewRegistryEndpoint(host, namespace, arch, os string) *RegistryEndpoint {
	return &RegistryEndpoint{
		host:      host,
		namespace: namespace,
		arch:      arch,
		os:        os,
	}
}

func (r *RegistryEndpoint) Host() string      { return r.host }
func (r *RegistryEndpoint) Namespace() string { return r.namespace }
func (r *RegistryEndpoint) Arch() string      { return r.arch }
func (r *RegistryEndpoint) Os() string        { return r.os }

func (r *RegistryEndpoint) BuildDestImageID(sourceID string) string {
	imgID, err := NewImageID(sourceID)
	if err != nil {
		return sourceID
	}

	var tag, digest, imageName string

	digest = imgID.Digest()
	imageName = imgID.Name()
	tag = imgID.Tag()

	if r.host == "" {
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		const maxLen = 50
		maxBaseLen := maxLen
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

		normalized = normalized + tag + digest
		return normalized
	}

	if r.namespace == "" {
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		const maxLen = 50
		maxBaseLen := maxLen
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
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	const maxLen = 50
	maxBaseLen := maxLen
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
