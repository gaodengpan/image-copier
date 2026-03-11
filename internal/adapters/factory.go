package adapters

import (
	"net/http"

	"github.com/gaodengpan/image-copier/internal/adapters/secondary/gateways"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/logging"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
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

func (f *AdapterFactory) CreateDockerClient() output.DockerClient {
	return gateways.NewExecDockerAdapter()
}

func (f *AdapterFactory) CreateRegistryClient() output.RegistryClient {
	return gateways.NewSkopeoAdapter()
}

func (f *AdapterFactory) CreateGitHubClient(owner, repo, token, workflowID string) output.GitHubClientWithRetry {
	return gateways.NewAPIAdapter(nil, token, owner, repo, workflowID)
}

func (f *AdapterFactory) CreateFileSystem() output.FileSystem {
	return gateways.NewOSAdapter()
}

func (f *AdapterFactory) CreateHTTPClient() output.HTTPClient {
	return &http.Client{}
}

func (f *AdapterFactory) CreateSystemClient() output.SystemClient {
	return system.NewSystemAdapter()
}

func (f *AdapterFactory) CreateImageIDService() *services.ImageIDService {
	return services.NewImageIDService()
}

// CreateLogger returns a Logger interface implementation
func (f *AdapterFactory) CreateLogger() output.Logger {
	return logging.NewLogrusLogger(f.logger)
}
