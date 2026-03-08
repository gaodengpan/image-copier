package value_objects

import (
	"fmt"
	"strings"
)

const maxDestImageIDLen = 50

type ImageID struct {
	raw    string
	repo   string
	tag    string
	digest string
}

func ParseImageID(raw string) *ImageID {
	id := &ImageID{raw: raw}

	digestIndex := strings.LastIndex(raw, "@")
	if digestIndex != -1 {
		id.digest = raw[digestIndex:]
		id.repo = raw[:digestIndex]
	} else {
		id.repo = raw
	}

	tagIndex := strings.LastIndex(id.repo, ":")
	if tagIndex != -1 && !strings.Contains(id.repo[:tagIndex], "/") {
		id.tag = id.repo[tagIndex:]
		id.repo = id.repo[:tagIndex]
	} else if digestIndex == -1 {
		tagIndex = strings.LastIndex(id.repo, ":")
		if tagIndex != -1 {
			id.tag = id.repo[tagIndex:]
			id.repo = id.repo[:tagIndex]
		}
	}

	return id
}

func (id *ImageID) Raw() string {
	return id.raw
}

func (id *ImageID) Registry() string {
	// Extract registry from the raw image ID
	// Format: [registry/]repo[:tag][@digest]
	parts := strings.SplitN(id.raw, "/", 2)
	if len(parts) == 2 {
		// Check if first part looks like a registry (contains . or : or is localhost)
		first := parts[0]
		if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
			return first
		}
	}
	// Default registry for Docker Hub images
	return "docker.io"
}

func (id *ImageID) Repository() string {
	return id.repo
}

func (id *ImageID) Tag() string {
	return id.tag
}

func (id *ImageID) Digest() string {
	return id.digest
}

func (id *ImageID) HasTagOrDigest() bool {
	if id.tag != "" || id.digest != "" {
		return true
	}
	return id.hasTagOrDigest(id.repo)
}

func (id *ImageID) hasTagOrDigest(str string) bool {
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

func (id *ImageID) BuildDestImageID(registryHost, registryNamespace string) string {
	tag := id.tag
	digest := id.digest
	imageName := id.repo

	if registryHost == "" {
		return id.normalizeImageName(imageName, tag, digest)
	}

	if registryNamespace == "" {
		normalized := id.normalizeImageName(imageName, tag, digest)
		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	normalized := id.normalizeImageName(imageName, tag, digest)
	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}

func (id *ImageID) normalizeImageName(imageName, tag, digest string) string {
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

func (id *ImageID) Normalize() string {
	segs := strings.Split(id.raw, "/")

	var normalized string
	switch len(segs) {
	case 1:
		normalized = fmt.Sprintf("docker.io/library/%s", id.raw)
	case 2:
		normalized = id.normalizeImageSegment(segs[0]) + "/" + segs[1]
	default:
		normalized = id.raw
	}

	lastSlash := strings.LastIndex(normalized, "/")
	tail := normalized
	if lastSlash >= 0 {
		tail = normalized[lastSlash+1:]
	}
	if !id.hasTagOrDigest(tail) {
		normalized += ":latest"
	}

	return normalized
}

func (id *ImageID) normalizeImageSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		return "docker.io/" + segment
	}
	return segment
}
