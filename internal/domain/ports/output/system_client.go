package output

import "context"

type SystemClient interface {
	CommandExists(ctx context.Context, cmd string) (bool, error)
	DockerRunning(ctx context.Context) (bool, error)
}
