package services

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/application/ports"
)

type ImageCheckerService struct {
	dockerClient   ports.DockerClient
	registryClient ports.RegistryClient
	logger         Logger
}

func NewImageCheckerService(
	dockerClient ports.DockerClient,
	registryClient ports.RegistryClient,
	logger Logger,
) *ImageCheckerService {
	return &ImageCheckerService{
		dockerClient:   dockerClient,
		registryClient: registryClient,
		logger:         logger,
	}
}

type CheckLocalResult struct {
	Exists bool
	Error  error
}

type CheckRemoteResult struct {
	Exists bool
	Error  error
}

func (s *ImageCheckerService) CheckLocal(ctx context.Context, imageID string) (CheckLocalResult, error) {
	exists, err := s.dockerClient.ImageExists(ctx, imageID)
	if err != nil {
		return CheckLocalResult{Exists: false, Error: err}, err
	}
	return CheckLocalResult{Exists: exists, Error: nil}, nil
}

func (s *ImageCheckerService) CheckRemote(ctx context.Context, imageID, username, password string) (CheckRemoteResult, error) {
	exists, err := s.registryClient.ImageExists(ctx, imageID, username, password)
	if err != nil {
		return CheckRemoteResult{Exists: false, Error: err}, err
	}
	return CheckRemoteResult{Exists: exists, Error: nil}, nil
}
