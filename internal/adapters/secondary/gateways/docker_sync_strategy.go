package gateways

import (
	"context"
	"fmt"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

type DockerSyncStrategy struct {
	dockerClient   output.DockerClient
	registryClient output.RegistryClient
	fileSystem     output.FileSystem
}

func NewDockerSyncStrategy(
	dockerClient output.DockerClient,
	registryClient output.RegistryClient,
	fileSystem output.FileSystem,
) *DockerSyncStrategy {
	return &DockerSyncStrategy{
		dockerClient:   dockerClient,
		registryClient: registryClient,
		fileSystem:     fileSystem,
	}
}

func (s *DockerSyncStrategy) SyncFromRegistry(ctx context.Context, opts output.SyncTargetOptions) error {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})

	// For cross-VM scenarios (like lima), we still need to use a temp file
	// because io.Pipe only works within the same process
	tmpPath, err := s.fileSystem.CreateTempFile("image-copier-*.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := func() {
		_ = s.fileSystem.RemoveFile(tmpPath)
	}
	defer cleanup()

	if err := s.registryClient.SaveImageToFile(ctx, output.RegistrySaveOptions{
		ImageID:    sourceImageID,
		ImageTag:   opts.TargetImageTag,
		OutputPath: tmpPath,
		Username:   opts.SourceRegistryUsername,
		Password:   opts.SourceRegistryPassword,
	}); err != nil {
		return fmt.Errorf("failed to save image to file: %w", err)
	}

	if err := s.dockerClient.LoadImage(ctx, tmpPath); err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}

	return nil
}

func (s *DockerSyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})
	return s.dockerClient.ImageExists(ctx, sourceImageID)
}

func (s *DockerSyncStrategy) Name() output.SyncTargetType {
	return output.SyncTargetDocker
}

// Distribute implements DistributionStrategy interface
func (s *DockerSyncStrategy) Distribute(ctx context.Context, opts output.DistributionOptions) error {
	syncOpts := output.SyncTargetOptions{
		SourceImageID:          opts.SourceImageID,
		SourceRegistryHost:     opts.SourceRegistryHost,
		SourceRegistryNS:       opts.SourceRegistryNS,
		SourceRegistryUsername: opts.SourceRegistryUser,
		SourceRegistryPassword: opts.SourceRegistryPass,
		TargetImageTag:         opts.SourceImageID,
	}
	return s.SyncFromRegistry(ctx, syncOpts)
}

// ExistsInDistributionTarget checks if the image exists in the distribution target
// This method implements DistributionStrategy interface
func (s *DockerSyncStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	syncOpts := output.SyncTargetOptions{
		SourceImageID:          opts.SourceImageID,
		SourceRegistryHost:     opts.SourceRegistryHost,
		SourceRegistryNS:       opts.SourceRegistryNS,
		SourceRegistryUsername: opts.SourceRegistryUser,
		SourceRegistryPassword: opts.SourceRegistryPass,
	}
	return s.ExistsInTarget(ctx, syncOpts)
}

// TargetType implements DistributionStrategy interface
func (s *DockerSyncStrategy) TargetType() value_objects.TargetType {
	return value_objects.TargetTypeDocker
}

var _ output.SyncTargetStrategy = (*DockerSyncStrategy)(nil)
var _ output.DistributionStrategy = (*DockerSyncStrategy)(nil)
