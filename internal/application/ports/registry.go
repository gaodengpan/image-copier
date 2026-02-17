package ports

import "context"

type RegistryClient interface {
	ImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	SaveImageToFile(ctx context.Context, imageID, imageTag, outputPath, username, password string) error
	CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	BuildDestImageID(sourceID, registryHost, registryNamespace string) string
}
