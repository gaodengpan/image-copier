package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageValidator_ValidateImageNameInput_Valid(t *testing.T) {
	v := NewImageValidator()

	tests := []struct {
		name  string
		input string
	}{
		{"simple image", "nginx"},
		{"with tag", "nginx:latest"},
		{"with registry", "docker.io/library/nginx"},
		{"with registry and tag", "docker.io/library/nginx:latest"},
		{"with digest", "nginx@sha256:abc123def456"},
		{"complex path", "myregistry.com/namespace/myimage:v1.0.0"},
		{"multi-segment", "myregistry.com/namespace/subnamespace/myimage:latest"},
		{"alpine known", "alpine"},
		{"ubuntu known", "ubuntu"},
		{"redis known", "redis"},
		{"python known", "python"},
		{"golang known", "golang"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateImageNameInput(tt.input)
			assert.True(t, result, "expected %q to be valid", tt.input)
		})
	}
}

func TestImageValidator_ValidateImageNameInput_Invalid(t *testing.T) {
	v := NewImageValidator()

	tests := []struct {
		name  string
		input string
	}{
		{"shell injection semicolon", "nginx;rm -rf /"},
		{"shell injection backtick", "nginx`ls`"},
		{"shell injection dollar", "nginx$(whoami)"},
		{"shell injection pipe", "nginx|cat /etc/passwd"},
		{"newline", "nginx\nls"},
		{"carriage return", "nginx\rls"},
		{"empty", ""},
		{"path traversal", "../../../etc/passwd"},
		{"invalid tag", "imagetag"},
		{"ImageTagLatest typo", "ImageTagLatest"},
		{"nginxLatest typo", "nginxLatest"},
		{"centosDev typo", "centosDev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateImageNameInput(tt.input)
			assert.False(t, result, "expected %q to be invalid", tt.input)
		})
	}
}

func TestImageValidator_ValidateFilePath(t *testing.T) {
	v := NewImageValidator()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple path", "nginx", true},
		{"valid with tag", "nginx:latest", true},
		{"valid registry path", "docker.io/library/nginx:latest", true},
		{"path traversal forward", "../etc/passwd", false},
		{"path traversal backward", "..\\etc\\passwd", false},
		{"null byte", "nginx\x00", false},
		{"absolute path not allowed", "/etc/passwd", false},
		{"absolute path tmp allowed", "/tmp/test", true},
		{"absolute path vartmp allowed", "/var/tmp/test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_ValidateCredentials(t *testing.T) {
	v := NewImageValidator()

	tests := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"valid credentials", "user", "password", true},
		{"empty username", "", "password", false},
		{"empty password", "user", "", false},
		{"both empty", "", "", false},
		{"long credentials", "user", string(make([]byte, 2000)), false},
		{"special chars in password", "user", "p@ssw0rd!", true},
		{"shell chars in credentials", "user;rm", "pass", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateCredentials(tt.username, tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_IsValidImageName(t *testing.T) {
	v := NewImageValidator()

	assert.True(t, v.IsValidImageName("nginx:latest"))
	assert.False(t, v.IsValidImageName("nginx;rm -rf"))
}

func TestImageValidator_containsDangerousChars(t *testing.T) {
	v := NewImageValidator()

	tests := []struct {
		input    string
		expected bool
	}{
		{"nginx", false},
		{"nginx:latest", false},
		{"nginx;rm", true},
		{"nginx`ls`", true},
		{"nginx$(whoami)", true},
		{"nginx\nls", true},
		{"nginx\rls", true},
		{"nginx&ls", true},
		{"nginx|ls", true},
		{"nginx<ls", true},
		{"nginx>ls", true},
		{"nginx(ls)", true},
		{"nginx{ls}", true},
		{"nginx[ls]", true},
		{"nginx\"quote", true},
		{"nginx'single", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := v.containsDangerousChars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLooksLikeMistypedImageTag(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"nginx", false},
		{"latest", false},
		{"nginxLatest", true},
		{"ImageTagLatest", true},
		{"centosDev", true},
		{"ubuntuStable", true},
		{"myappEdge", true},
		{"normalTag", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := looksLikeMistypedImageTag(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
