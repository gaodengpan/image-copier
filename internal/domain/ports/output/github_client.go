package output

import "context"

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
