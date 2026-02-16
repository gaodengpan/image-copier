package cli

import (
	"github.com/gaodengpan/image-copier/internal/adapters/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/filesystem"
	"github.com/gaodengpan/image-copier/internal/adapters/github"
	"github.com/gaodengpan/image-copier/internal/adapters/registry"
	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/sirupsen/logrus"
)

func NewPuller(config *core.Config, logger *logrus.Logger) *core.Puller {
	dockerClient := docker.NewExecDockerAdapter()
	registryClient := registry.NewSkopeoAdapter()
	githubClient := github.NewAPIAdapter(nil, config.GithubToken, config.GithubOwner, config.GithubRepo)
	fs := filesystem.NewOSAdapter()

	return core.NewPullerWithPorts(config, logger, dockerClient, registryClient, githubClient, fs)
}
