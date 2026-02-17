package value_objects

import (
	"fmt"
	"strings"
)

type ImageID struct {
	original   string
	normalized string
	registry   string
	name       string
	tag        string
	digest     string
}

func NewImageID(raw string) (*ImageID, error) {
	if raw == "" {
		return nil, fmt.Errorf("image ID cannot be empty")
	}

	normalized := normalizeImageID(raw)
	registry, name, tag, digest := parseImageParts(normalized)

	return &ImageID{
		original:   raw,
		normalized: normalized,
		registry:   registry,
		name:       name,
		tag:        tag,
		digest:     digest,
	}, nil
}

func (i *ImageID) String() string   { return i.normalized }
func (i *ImageID) Original() string { return i.original }
func (i *ImageID) Registry() string { return i.registry }
func (i *ImageID) Name() string     { return i.name }
func (i *ImageID) Tag() string      { return i.tag }
func (i *ImageID) Digest() string   { return i.digest }
func (i *ImageID) HasTag() bool     { return i.tag != "" }
func (i *ImageID) HasDigest() bool  { return i.digest != "" }
func (i *ImageID) IsOfficial() bool { return !strings.Contains(i.name, "/") }

func normalizeImageID(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		normalized = fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		normalized = normalizeImageSegment(segs[0]) + "/" + segs[1]
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

func normalizeImageSegment(segment string) string {
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
	if len(colonParts) > 2 || len(colonParts) == 2 {
		return true
	}
	return false
}

func parseImageParts(imageID string) (registry, name, tag, digest string) {
	digestIndex := strings.LastIndex(imageID, "@")
	if digestIndex != -1 {
		digest = imageID[digestIndex:]
		imageName := imageID[:digestIndex]
		imageID = imageName
	}

	tagIndex := strings.LastIndex(imageID, ":")
	if tagIndex != -1 {
		afterTag := imageID[tagIndex+1:]
		if !strings.Contains(afterTag, "/") {
			tag = imageID[tagIndex:]
			imageID = imageID[:tagIndex]
		}
	}

	slashIndex := strings.LastIndex(imageID, "/")
	if slashIndex != -1 {
		registry = imageID[:slashIndex]
		name = imageID[slashIndex+1:]
	} else {
		name = imageID
	}

	return
}
