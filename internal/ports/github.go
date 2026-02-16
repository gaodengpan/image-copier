package ports

import "context"

type GitHubClient interface {
	TriggerWorkflow(ctx context.Context, owner, repo, workflowID string, inputs map[string]string) (string, error)
	GetWorkflowStatus(ctx context.Context, owner, repo, runID string) (string, error)
}
