package output

import "github.com/gaodengpan/image-copier/internal/domain/value_objects"

// PrivateRegistryConfig represents configuration for a private registry
type PrivateRegistryConfig struct {
	Name       string
	Host       string
	Username   string
	Password   string
	Insecure   bool
	MirrorMode bool // Keep original image names for mirror registries
}

// SyncConfig provides configuration needed for sync operations
// This interface decouples the application layer from infrastructure configuration
type SyncConfig interface {
	// Staging registry configuration
	StagingRegistryHost() string
	StagingRegistryNamespace() string
	StagingRegistryUsername() string
	StagingRegistryPassword() string

	// Default architecture and OS
	DefaultArch() string
	DefaultOS() string

	// Distribution targets
	GetDistributionTargets(targets []string) []string
	GetPrivateRegistry(name string) *PrivateRegistryConfig
}

// DistributionTargetBuilder builds distribution targets from target names
type DistributionTargetBuilder interface {
	BuildTargets(targetNames []string) []*value_objects.DistributionTarget
}
