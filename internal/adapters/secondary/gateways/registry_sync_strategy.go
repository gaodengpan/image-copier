package gateways

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
)

type RegistrySyncStrategy struct {
	registryClient output.RegistryClient
	validator      *validators.ImageValidator
	commandRunner  func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewRegistrySyncStrategy(registryClient output.RegistryClient) *RegistrySyncStrategy {
	return &RegistrySyncStrategy{
		registryClient: registryClient,
		validator:      validators.NewImageValidator(),
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
	}
}

func (s *RegistrySyncStrategy) SyncFromRegistry(ctx context.Context, opts output.SyncTargetOptions) error {
	sourceImageID := s.registryClient.BuildDestImageID(
		opts.SourceImageID,
		opts.SourceRegistryHost,
		opts.SourceRegistryNS,
	)

	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID)

	srcCreds := fmt.Sprintf("%s:%s", opts.SourceRegistryUsername, opts.SourceRegistryPassword)
	destCreds := fmt.Sprintf("%s:%s", opts.TargetRegistryUsername, opts.TargetRegistryPassword)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := s.commandRunner(ctx, SkopeoCommand,
		"copy",
		"--src-creds="+srcCreds,
		"--dest-creds="+destCreds,
		"docker://"+sourceImageID,
		"docker://"+targetImageID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewRegistryError("SyncFromRegistry",
			fmt.Sprintf("failed to sync image from %s to %s: %s", sourceImageID, targetImageID, string(output)), err)
	}

	return nil
}

func (s *RegistrySyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID)
	return s.registryClient.CheckImageExists(ctx, targetImageID, opts.TargetRegistryUsername, opts.TargetRegistryPassword)
}

func (s *RegistrySyncStrategy) Name() output.SyncTargetType {
	return output.SyncTargetRegistry
}

func (s *RegistrySyncStrategy) buildTargetImageID(registryHost, sourceImageID string) string {
	return s.registryClient.BuildDestImageID(sourceImageID, registryHost, "")
}

var _ output.SyncTargetStrategy = (*RegistrySyncStrategy)(nil)
