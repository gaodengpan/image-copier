package value_objects

import (
	sharederrors "github.com/gaodengpan/image-copier/internal/shared/errors"
)

// TargetType represents the type of distribution target
type TargetType string

const (
	TargetTypeDocker   TargetType = "docker"
	TargetTypeRegistry TargetType = "registry"
)

// DistributionTarget represents a target for image distribution
type DistributionTarget struct {
	name       string
	typ        TargetType
	host       string // Empty for docker target
	username   string
	password   string
	insecure   bool
	mirrorMode bool // Keep original image names
}

// NewDockerTarget creates a new Docker distribution target
func NewDockerTarget() *DistributionTarget {
	return &DistributionTarget{
		name: "docker",
		typ:  TargetTypeDocker,
	}
}

// NewRegistryTarget creates a new Registry distribution target
func NewRegistryTarget(name, host, username, password string, insecure, mirrorMode bool) (*DistributionTarget, error) {
	if name == "" {
		return nil, sharederrors.NewValidationError("name", "registry target name is required")
	}
	if host == "" {
		return nil, sharederrors.NewValidationError("host", "registry target host is required")
	}
	return &DistributionTarget{
		name:       name,
		typ:        TargetTypeRegistry,
		host:       host,
		username:   username,
		password:   password,
		insecure:   insecure,
		mirrorMode: mirrorMode,
	}, nil
}

// Name returns the target name
func (t *DistributionTarget) Name() string {
	return t.name
}

// Type returns the target type
func (t *DistributionTarget) Type() TargetType {
	return t.typ
}

// Host returns the target host (empty for docker)
func (t *DistributionTarget) Host() string {
	return t.host
}

// Username returns the target username
func (t *DistributionTarget) Username() string {
	return t.username
}

// Password returns the target password
func (t *DistributionTarget) Password() string {
	return t.password
}

// Insecure returns whether to skip TLS verification
func (t *DistributionTarget) Insecure() bool {
	return t.insecure
}

// MirrorMode returns whether to keep original image names
func (t *DistributionTarget) MirrorMode() bool {
	return t.mirrorMode
}

// String returns a string representation of the target
func (t *DistributionTarget) String() string {
	if t.typ == TargetTypeDocker {
		return "docker (local)"
	}
	return t.name + " (" + t.host + ")"
}
