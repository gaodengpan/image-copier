package value_objects

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	imageValidationPattern = `^[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$|^([a-zA-Z0-9._-]+:[0-9]+/[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*)(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$`
	validShellChars        = "$`\"'\\;&|()<>()[]{}"
)

var (
	imageValidationRegex = regexp.MustCompile(imageValidationPattern)
	knownSimpleNames     = []string{
		"alpine", "busybox", "centos", "debian", "fedora", "ubuntu", "nginx", "redis",
		"mysql", "postgres", "mongo", "node", "python", "java", "golang", "ruby",
		"php", "httpd", "mariadb", "wordpress", "jenkins", "gitlab", "elasticsearch",
		"kibana", "rabbitmq", "memcached", "tomcat", "wildfly", "glassfish", "oraclelinux",
	}
)

type ImageID struct {
	value      string
	normalized string
	hasTag     bool
	hasDigest  bool
	tag        string
	digest     string
}

func NewImageID(source string) (*ImageID, error) {
	normalized := normalizeSourceID(source)

	img := &ImageID{
		value:      source,
		normalized: normalized,
		hasTag:     hasTagOrDigest(normalized),
		hasDigest:  strings.Contains(normalized, "@"),
	}

	if img.hasDigest {
		digestIndex := strings.LastIndex(normalized, "@")
		if digestIndex != -1 {
			img.digest = normalized[digestIndex:]
		}
	}

	if img.hasTag {
		tagIndex := strings.LastIndex(normalized, ":")
		if tagIndex != -1 && (len(normalized) <= tagIndex || normalized[tagIndex+1] != '/') {
			img.tag = normalized[tagIndex:]
		}
	}

	return img, nil
}

func (i *ImageID) String() string {
	return i.normalized
}

func (i *ImageID) Original() string {
	return i.value
}

func (i *ImageID) HasTag() bool {
	return i.hasTag
}

func (i *ImageID) HasDigest() bool {
	return i.hasDigest
}

func (i *ImageID) Tag() string {
	return i.tag
}

func (i *ImageID) Digest() string {
	return i.digest
}

func (i *ImageID) ImageName() string {
	name := i.normalized

	if idx := strings.LastIndex(name, "@"); idx != -1 {
		name = name[:idx]
	}
	if idx := strings.LastIndex(name, ":"); idx != -1 && !strings.Contains(name[:idx], "/") {
		name = name[:idx]
	}

	return name
}

func normalizeSourceID(imageID string) string {
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

type ImageIDValidator struct{}

func NewImageIDValidator() *ImageIDValidator {
	return &ImageIDValidator{}
}

func (v *ImageIDValidator) ValidateInput(name string) bool {
	if containsDangerousChars(name) {
		return false
	}

	if !imageValidationRegex.MatchString(name) {
		return false
	}

	return v.additionalValidation(name)
}

func (v *ImageIDValidator) Validate(name string) bool {
	return v.ValidateInput(name)
}

func (v *ImageIDValidator) ValidateCredentials(username, password string) bool {
	if containsDangerousChars(username) || containsDangerousChars(password) {
		return false
	}

	if len(username) == 0 || len(password) == 0 {
		return false
	}

	if len(username) > 1000 || len(password) > 1000 {
		return false
	}

	return true
}

func (v *ImageIDValidator) additionalValidation(name string) bool {
	if !validateFilePath(name) {
		return false
	}

	atIndex := strings.Index(name, "@")
	if atIndex != -1 {
		afterAt := name[atIndex+1:]
		colonIndex := strings.Index(afterAt, ":")
		if colonIndex != -1 {
			algorithm := afterAt[:colonIndex]
			hash := afterAt[colonIndex+1:]
			if len(algorithm) < 3 {
				return false
			}
			if algorithm == "sha256" {
				for _, r := range hash {
					if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
						return false
					}
				}
			}
		} else {
			if len(afterAt) < 5 {
				return false
			}
		}
	}

	if name == "imagetag" {
		return false
	}

	if !strings.Contains(name, ":") && !strings.Contains(name, "@") {
		parts := strings.Split(name, "/")
		if len(parts) == 1 {
			isKnownSimple := false
			for _, knownName := range knownSimpleNames {
				if name == knownName {
					isKnownSimple = true
					break
				}
			}

			if !isKnownSimple && len(parts[0]) > 5 {
				if looksLikeMistypedImageTag(parts[0]) {
					return false
				}
			}
		}
	} else if !strings.Contains(name, "/") && strings.Contains(name, ":") && !strings.Contains(name, "@") {
		if name == "imagetag" {
			return false
		}
	}

	return true
}

func validateFilePath(path string) bool {
	if strings.Contains(path, "../") || strings.Contains(path, "..\\") ||
		strings.Contains(path, "/..") || strings.Contains(path, "\\..") {
		return false
	}

	if strings.Contains(path, "\x00") {
		return false
	}

	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		allowedPrefixes := []string{"/tmp/", "/var/tmp/"}
		isAllowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return false
		}
	}

	return true
}

func containsDangerousChars(input string) bool {
	if strings.ContainsAny(input, "\n\r") {
		return true
	}
	return strings.ContainsAny(input, validShellChars)
}

func looksLikeMistypedImageTag(s string) bool {
	for i, r := range s {
		if i > 0 && i < len(s)-1 && unicode.IsLower(r) && unicode.IsUpper(rune(s[i+1])) {
			return true
		}

		remaining := s[i:]
		if strings.HasPrefix(strings.ToLower(remaining), "latest") ||
			strings.HasPrefix(strings.ToLower(remaining), "alpine") ||
			strings.HasPrefix(strings.ToLower(remaining), "dev") ||
			strings.HasPrefix(strings.ToLower(remaining), "prod") ||
			strings.HasPrefix(strings.ToLower(remaining), "test") {
			if i > 2 {
				return true
			}
		}
	}

	if strings.Contains(s, "latest") || strings.Contains(s, "stable") || strings.Contains(s, "edge") {
		if s == "imagetag" {
			return true
		}
	}

	return false
}
