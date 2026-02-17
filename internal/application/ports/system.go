package ports

import "context"

type SystemClient interface {
	CommandExists(ctx context.Context, cmd string) (bool, error)
	DockerRunning(ctx context.Context) (bool, error)
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	RunWithOutput(ctx context.Context, name string, args ...string) (string, string, error)
}
