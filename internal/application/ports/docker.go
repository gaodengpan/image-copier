package ports

import "context"

type DockerClient interface {
	ImageExists(ctx context.Context, imageID string) (bool, error)
	ListImages(ctx context.Context) ([]string, error)
	LoadImage(ctx context.Context, tarPath string) error
}
