package services

import (
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

type ImageIDService struct{}

func NewImageIDService() *ImageIDService {
	return &ImageIDService{}
}

func (s *ImageIDService) BuildDestImageID(sourceID, registryHost, registryNamespace string) string {
	return value_objects.ParseImageID(sourceID).BuildDestImageID(registryHost, registryNamespace)
}

func (s *ImageIDService) NormalizeSourceID(imageID string) string {
	return value_objects.ParseImageID(imageID).Normalize()
}

// Ensure ImageIDService implements the output.ImageIDService interface
var _ output.ImageIDService = (*ImageIDService)(nil)
