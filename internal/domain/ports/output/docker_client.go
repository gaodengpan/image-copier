package output

import (
	"context"
	"io"
)

type DockerClient interface {
	ImageExists(ctx context.Context, imageID string) (bool, error)
	ListImages(ctx context.Context) ([]string, error)
	LoadImage(ctx context.Context, tarPath string) error
	// LoadImageFromReader loads a Docker image from a reader (for streaming)
	LoadImageFromReader(ctx context.Context, reader io.Reader) error
}
