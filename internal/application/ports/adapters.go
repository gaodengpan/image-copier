package ports

import (
	"context"
	"net/http"
)

type GitHubClient interface {
	TriggerWorkflow(ctx context.Context, owner, repo, workflowID string, inputs map[string]string) (string, error)
	GetWorkflowStatus(ctx context.Context, owner, repo, runID string) (string, error)
	WaitForWorkflow(ctx context.Context, owner, repo, runID string) error
	FindWorkflowRunID(ctx context.Context, owner, repo, workflowID, sourceID, destImageID, suffix string) (string, error)
}

type GitHubClientWithRetry interface {
	GitHubClient
	TriggerWorkflowWithRetry(ctx context.Context, imageID, destImageID, arch, osType string) (string, error)
	WaitForWorkflowSimple(ctx context.Context, runID string) error
}

type RegistryClient interface {
	ImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	SaveImageToFile(ctx context.Context, imageID, imageTag, outputPath, username, password string) error
	CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error)
	BuildDestImageID(sourceID, registryHost, registryNamespace string) string
}

type DockerClient interface {
	ImageExists(ctx context.Context, imageID string) (bool, error)
	ListImages(ctx context.Context) ([]string, error)
	LoadImage(ctx context.Context, tarPath string) error
}

type FileSystem interface {
	CreateTempFile(pattern string) (string, error)
	RemoveFile(path string) error
}

type SystemClient interface {
	CommandExists(ctx context.Context, cmd string) (bool, error)
	DockerRunning(ctx context.Context) (bool, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
