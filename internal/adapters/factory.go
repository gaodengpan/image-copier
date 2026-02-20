package adapters

import (
	"net/http"

	dockeradapter "github.com/gaodengpan/image-copier/internal/adapters/secondary/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/filesystem"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/github"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/registry"
	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/infrastructure/system"
	"github.com/sirupsen/logrus"
)

type AdapterFactory struct {
	logger *logrus.Logger
}

func NewAdapterFactory(logger *logrus.Logger) *AdapterFactory {
	return &AdapterFactory{
		logger: logger,
	}
}

func (f *AdapterFactory) CreateDockerClient() ports.DockerClient {
	return dockeradapter.NewExecDockerAdapter()
}

func (f *AdapterFactory) CreateRegistryClient() ports.RegistryClient {
	return registry.NewSkopeoAdapter()
}

func (f *AdapterFactory) CreateGitHubClient(owner, repo, token, workflowID string) ports.GitHubClientWithRetry {
	return github.NewAPIAdapter(nil, token, owner, repo, workflowID)
}

func (f *AdapterFactory) CreateFileSystem() ports.FileSystem {
	return filesystem.NewOSAdapter()
}

func (f *AdapterFactory) CreateHTTPClient() ports.HTTPClient {
	return &http.Client{}
}

func (f *AdapterFactory) CreateSystemClient() ports.SystemClient {
	return system.NewSystemAdapter()
}

func (f *AdapterFactory) CreateImageIDService() *services.ImageIDService {
	return services.NewImageIDService()
}

func (f *AdapterFactory) CreateLogrusLogger() *logrus.Logger {
	return f.logger
}
