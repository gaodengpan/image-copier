package adapters

import (
	"net/http"

	"github.com/gaodengpan/image-copier/internal/adapters/secondary/gateways"
	"github.com/gaodengpan/image-copier/internal/domain/ports"
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
	return gateways.NewExecDockerAdapter()
}

func (f *AdapterFactory) CreateRegistryClient() ports.RegistryClient {
	return gateways.NewSkopeoAdapter()
}

func (f *AdapterFactory) CreateGitHubClient(owner, repo, token, workflowID string) ports.GitHubClientWithRetry {
	return gateways.NewAPIAdapter(nil, token, owner, repo, workflowID)
}

func (f *AdapterFactory) CreateFileSystem() ports.FileSystem {
	return gateways.NewOSAdapter()
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
