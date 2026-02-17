package services

import (
	"context"
	"fmt"
	"time"

	"github.com/gaodengpan/image-copier/internal/application/ports"
)

type GitHubActionConfig struct {
	MaxRetries     int
	PollInterval   time.Duration
	FindRunTimeout time.Duration
}

type GitHubActionRequest struct {
	SourceID   string
	DestID     string
	Arch       string
	Os         string
	WorkflowID string
	Suffix     string
}

type GitHubActionResult struct {
	RunID      string
	Success    bool
	Conclusion string
	Duration   time.Duration
}

type GitHubActionService struct {
	githubClient ports.GitHubClient
	logger       Logger
	owner        string
	repo         string
	config       GitHubActionConfig
}

type Logger interface {
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

func NewGitHubActionService(
	githubClient ports.GitHubClient,
	logger Logger,
	owner, repo string,
) *GitHubActionService {
	return &GitHubActionService{
		githubClient: githubClient,
		logger:       logger,
		owner:        owner,
		repo:         repo,
		config: GitHubActionConfig{
			MaxRetries:     300,
			PollInterval:   2 * time.Second,
			FindRunTimeout: 60 * time.Second,
		},
	}
}

func (s *GitHubActionService) Execute(ctx context.Context, req GitHubActionRequest) (GitHubActionResult, error) {
	runID, err := s.Trigger(ctx, req)
	if err != nil {
		return GitHubActionResult{}, err
	}

	result, err := s.WaitForCompletion(ctx, runID)
	return result, err
}

func (s *GitHubActionService) Trigger(ctx context.Context, req GitHubActionRequest) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())
	req.Suffix = suffix

	inputs := map[string]string{
		"imageId":     req.SourceID,
		"destImageId": req.DestID,
		"suffix":      suffix,
		"arch":        req.Arch,
		"os":          req.Os,
	}

	runID, err := s.githubClient.TriggerWorkflow(ctx, s.owner, s.repo, req.WorkflowID, inputs)
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}

	s.logger.Infof("Triggered workflow run ID: %s", runID)
	return runID, nil
}

func (s *GitHubActionService) WaitForCompletion(ctx context.Context, runID string) (GitHubActionResult, error) {
	startTime := time.Now()

	for i := 0; i < s.config.MaxRetries; i++ {
		status, err := s.githubClient.GetWorkflowStatus(ctx, s.owner, s.repo, runID)
		if err != nil {
			s.logger.Debugf("Workflow status check failed: %v", err)
			select {
			case <-time.After(s.config.PollInterval):
				continue
			case <-ctx.Done():
				return GitHubActionResult{}, ctx.Err()
			}
		}

		if status == "completed" || status == "success" {
			return GitHubActionResult{
				RunID:      runID,
				Success:    true,
				Conclusion: status,
				Duration:   time.Since(startTime),
			}, nil
		}

		if status == "failed" || status == "cancelled" {
			return GitHubActionResult{
				RunID:      runID,
				Success:    false,
				Conclusion: status,
				Duration:   time.Since(startTime),
			}, fmt.Errorf("workflow failed with conclusion: %s", status)
		}

		select {
		case <-time.After(s.config.PollInterval):
		case <-ctx.Done():
			return GitHubActionResult{}, ctx.Err()
		}
	}

	return GitHubActionResult{}, fmt.Errorf("workflow timed out after %d attempts", s.config.MaxRetries)
}
