package output

import (
	"context"
	"io"
)

type RegistryClient interface {
	ImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	SaveImageToFile(ctx context.Context, imageID, imageTag, outputPath, username, password string) error
	// SaveImageToWriter saves a registry image to a writer (for streaming)
	SaveImageToWriter(ctx context.Context, imageID, imageTag string, writer io.Writer, username, password string) error
	CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	BuildDestImageID(sourceID, registryHost, registryNamespace string) string
}
