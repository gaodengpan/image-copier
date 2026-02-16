package core

import "errors"

type PullStage int

const (
	StageCheckLocal PullStage = iota
	StageCheckRegistry
	StageTriggerWorkflow
	StageWaitWorkflow
	StageDownload
	StageLoad
)

type Config struct {
	GithubOwner       string
	GithubRepo        string
	GithubToken       string
	GithubWorkflowID  string
	RegistryHost      string
	RegistryUsername  string
	RegistryPassword  string
	RegistryNamespace string
	RegistryArch      string
	RegistryOs        string
	Force             bool
	RetryConfig       interface{}
	DryRun            bool
}

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay int
	MaxDelay     int
}

func NormalizeSourceID(sourceID string) string {
	if len(sourceID) > MaxNormalizedLen {
		return sourceID[:MaxNormalizedLen] + "..."
	}
	return sourceID
}

var (
	ErrSkipped = errors.New("image skipped")
	ErrDryRun  = errors.New("dry run mode")
)
