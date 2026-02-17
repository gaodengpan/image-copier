package services

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
)

type ImageDownloaderService struct {
	dockerClient   ports.DockerClient
	registryClient ports.RegistryClient
	fileSystem     ports.FileSystem
	logger         Logger
	imageValidator *validators.ImageValidator
}

func NewImageDownloaderService(
	dockerClient ports.DockerClient,
	registryClient ports.RegistryClient,
	fileSystem ports.FileSystem,
	logger Logger,
) *ImageDownloaderService {
	return &ImageDownloaderService{
		dockerClient:   dockerClient,
		registryClient: registryClient,
		fileSystem:     fileSystem,
		logger:         logger,
		imageValidator: validators.NewImageValidator(),
	}
}

func (s *ImageDownloaderService) DownloadAndLoad(ctx context.Context, registryImageID, userImageTag, username, password string) error {
	if !s.imageValidator.IsValidImageName(registryImageID) {
		return ErrInvalidImageName
	}

	tmpPath, err := s.fileSystem.CreateTempFile("image-copier-*.tar")
	if err != nil {
		return err
	}

	cleanup := func() {
		if err := s.fileSystem.RemoveFile(tmpPath); err != nil {
			s.logger.Debugf("Failed to remove temp file %s: %v", tmpPath, err)
		}
	}
	defer cleanup()

	if err := s.registryClient.SaveImageToFile(ctx, registryImageID, userImageTag, tmpPath, username, password); err != nil {
		return err
	}

	if err := s.dockerClient.LoadImage(ctx, tmpPath); err != nil {
		return err
	}

	return nil
}

var ErrInvalidImageName = &DownloadError{Message: "invalid image name"}

type DownloadError struct {
	Message string
}

func (e *DownloadError) Error() string {
	return e.Message
}
