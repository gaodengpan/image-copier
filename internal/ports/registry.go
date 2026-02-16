package ports

import "context"

type RegistryClient interface {
	ImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	CopyImage(ctx context.Context, source, dest, username, password string) error
	CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error)
}
