package validators

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	imageValidationPattern = `^[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$|^([a-zA-Z0-9._-]+:[0-9]+/[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*)(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$`
	imgShellChars          = "$`\"'\\;&|()<>()[]{}"
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

type ImageValidator struct {
	imageValidationRegex *regexp.Regexp
}

func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		imageValidationRegex: imageValidationRegex,
	}
}

func (v *ImageValidator) ValidateImageNameInput(name string) bool {
	if v.containsDangerousChars(name) {
		return false
	}

	if !v.imageValidationRegex.MatchString(name) {
		return false
	}

	return v.additionalValidation(name)
}

func (v *ImageValidator) ValidateFilePath(path string) bool {
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

func (v *ImageValidator) ValidateYAMLContent(content string) error {
	if strings.Contains(content, "{{") && strings.Contains(content, "shell") {
		return &ValidationError{Message: "unsafe template construct detected"}
	}

	dangerousPatterns := []string{
		"| sh", "| bash", "| zsh",
		"&", "&&", "||", ";", "`", "$(",
		"eval ", "exec ", "command ",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(strings.ToLower(content), strings.ToLower(pattern)) {
			return &ValidationError{Message: "unsafe content detected: " + pattern}
		}
	}

	return nil
}

func (v *ImageValidator) ValidateCredentials(username, password string) bool {
	if v.containsDangerousChars(username) || v.containsDangerousChars(password) {
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

func (v *ImageValidator) IsValidImageName(name string) bool {
	return v.ValidateImageNameInput(name)
}

func (v *ImageValidator) containsDangerousChars(input string) bool {
	if strings.ContainsAny(input, "\n\r") {
		return true
	}
	return strings.ContainsAny(input, imgShellChars)
}

func (v *ImageValidator) additionalValidation(name string) bool {
	if !v.ValidateFilePath(name) {
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

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
