package gateways

import (
	"context"
	"fmt"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
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
	sourceImageID := s.registryClient.BuildDestImageID(
		opts.SourceImageID,
		opts.SourceRegistryHost,
		opts.SourceRegistryNS,
	)

	tmpPath, err := s.fileSystem.CreateTempFile("image-copier-*.tar")
	if err != nil {
		return err
	}

	cleanup := func() {
		_ = s.fileSystem.RemoveFile(tmpPath)
	}
	defer cleanup()

	if err := s.registryClient.SaveImageToFile(
		ctx,
		sourceImageID,
		opts.TargetImageTag,
		tmpPath,
		opts.SourceRegistryUsername,
		opts.SourceRegistryPassword,
	); err != nil {
		return fmt.Errorf("failed to save image to file: %w", err)
	}

	if err := s.dockerClient.LoadImage(ctx, tmpPath); err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}

	return nil
}

func (s *DockerSyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	sourceImageID := s.registryClient.BuildDestImageID(
		opts.SourceImageID,
		opts.SourceRegistryHost,
		opts.SourceRegistryNS,
	)
	return s.dockerClient.ImageExists(ctx, sourceImageID)
}

func (s *DockerSyncStrategy) Name() output.SyncTargetType {
	return output.SyncTargetDocker
}

var _ output.SyncTargetStrategy = (*DockerSyncStrategy)(nil)
