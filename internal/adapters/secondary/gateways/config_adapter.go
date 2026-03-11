package gateways

import (
	"fmt"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
)

// ConfigAdapter adapts config.Config to the SyncConfig interface
type ConfigAdapter struct {
	cfg    *config.Config
	logger output.Logger
}

// NewConfigAdapter creates a new ConfigAdapter
func NewConfigAdapter(cfg *config.Config, logger output.Logger) *ConfigAdapter {
	return &ConfigAdapter{cfg: cfg, logger: logger}
}

// StagingRegistryHost returns the staging registry host
func (a *ConfigAdapter) StagingRegistryHost() string {
	return a.cfg.Registry.Host
}

// StagingRegistryNamespace returns the staging registry namespace
func (a *ConfigAdapter) StagingRegistryNamespace() string {
	return a.cfg.Registry.Namespace
}

// StagingRegistryUsername returns the staging registry username
func (a *ConfigAdapter) StagingRegistryUsername() string {
	return a.cfg.Registry.Username
}

// StagingRegistryPassword returns the staging registry password
func (a *ConfigAdapter) StagingRegistryPassword() string {
	return a.cfg.Registry.Password
}

// DefaultArch returns the default architecture
func (a *ConfigAdapter) DefaultArch() string {
	return a.cfg.Registry.Arch
}

// DefaultOS returns the default OS
func (a *ConfigAdapter) DefaultOS() string {
	return a.cfg.Registry.Os
}

// GetDistributionTargets returns the distribution targets
func (a *ConfigAdapter) GetDistributionTargets(targets []string) []string {
	return a.cfg.GetDistributionTargets(targets)
}

// GetPrivateRegistry returns a private registry by name
func (a *ConfigAdapter) GetPrivateRegistry(name string) *output.PrivateRegistryConfig {
	reg := a.cfg.GetPrivateRegistryByName(name)
	if reg == nil {
		return nil
	}
	return &output.PrivateRegistryConfig{
		Name:     reg.Name,
		Host:     reg.Host,
		Username: reg.Username,
		Password: reg.Password,
	}
}

// BuildTargets builds distribution targets from target names
func (a *ConfigAdapter) BuildTargets(targetNames []string) []*value_objects.DistributionTarget {
	targets := make([]*value_objects.DistributionTarget, 0, len(targetNames))

	for _, name := range targetNames {
		if config.IsDockerTarget(name) {
			targets = append(targets, value_objects.NewDockerTarget())
		} else {
			reg := a.GetPrivateRegistry(name)
			if reg != nil {
				target, err := value_objects.NewRegistryTarget(reg.Name, reg.Host, reg.Username, reg.Password)
				if err != nil {
					if a.logger != nil {
						a.logger.Warn(fmt.Sprintf("failed to create registry target %q: %v", name, err))
					}
					continue
				}
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// Ensure ConfigAdapter implements the interfaces
var _ output.SyncConfig = (*ConfigAdapter)(nil)
var _ output.DistributionTargetBuilder = (*ConfigAdapter)(nil)
