package core

import "time"

// General constants
const (
	DefaultCacheTTL     = 30 * time.Second // Default cache validity period
	MaxCacheSizeDefault = 10000            // Default maximum cache size
	MaxNormalizedLen    = 40               // Maximum normalization length
	CredentialsSep      = ":"              // Credential separator
)

// Command constants
const (
	DockerCommand        = "docker"
	SkopeoCommand        = "skopeo"
	DockerImageFormat    = "{{.Repository}}:{{.Tag}}"
	CredentialsSeparator = ":"
)

// GitHub API constants
const (
	GitHubAPIVersion = "2022-11-28"
	GitHubMediaType  = "application/vnd.github+json"
)

// Timeout constants
const (
	WorkflowPollTimeout = 30 * time.Second
	SkopeoCopyTimeout   = 120 * time.Second
	DockerLoadTimeout   = 60 * time.Second
	ListImagesTimeout   = 15 * time.Second
	CheckLocalTimeout   = 10 * time.Second
)

// Regex patterns
const (
	ImageValidationPattern = `^[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$|^([a-zA-Z0-9._-]+:[0-9]+/[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*)(:[a-zA-Z0-9._-]+)?(@[a-zA-Z0-9._:-]+)?$`
	ValidShellChars        = "$`\"'\\;&|()<>()[]{}"
	PathTraversalChars     = "../ ..\\ /.. \\.."
)

// Sanitization
const (
	SensitiveDataPrefix = "[REDACTED:"
	SensitiveDataSuffix = "]"
)
