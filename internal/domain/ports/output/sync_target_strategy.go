package output

import "context"

type SyncTargetType string

const (
	SyncTargetDocker   SyncTargetType = "docker"
	SyncTargetRegistry SyncTargetType = "registry"
)

type SyncTargetOptions struct {
	SourceImageID  string
	TargetImageTag string

	SourceRegistryHost     string
	SourceRegistryUsername string
	SourceRegistryPassword string
	SourceRegistryNS       string

	TargetRegistryHost     string
	TargetRegistryUsername string
	TargetRegistryPassword string
}

type SyncTargetStrategy interface {
	SyncFromRegistry(ctx context.Context, opts SyncTargetOptions) error
	ExistsInTarget(ctx context.Context, opts SyncTargetOptions) (bool, error)
	Name() SyncTargetType
}
