package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
	"github.com/gaodengpan/image-copier/pkg/retry"
)

const (
	GitHubAPIVersion    = "2022-11-28"
	GitHubMediaType     = "application/vnd.github+json"
	WorkflowPollTimeout = 30 * time.Second
)

type APIAdapter struct {
	httpClient *http.Client
	token      string
	owner      string
	repo       string
	workflowID string
}

func NewAPIAdapter(httpClient *http.Client, token, owner, repo, workflowID string) *APIAdapter {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &APIAdapter{
		httpClient: httpClient,
		token:      token,
		owner:      owner,
		repo:       repo,
		workflowID: workflowID,
	}
}

func (a *APIAdapter) TriggerWorkflowSimple(ctx context.Context, imageID, destImageID, arch, osType string) (string, error) {
	return a.TriggerWorkflow(ctx, a.owner, a.repo, a.workflowID, map[string]string{
		"imageId":     imageID,
		"destImageId": destImageID,
		"arch":        arch,
		"os":          osType,
	})
}

func (a *APIAdapter) WaitForWorkflowSimple(ctx context.Context, runID string) error {
	return a.WaitForWorkflow(ctx, a.owner, a.repo, runID)
}

func (a *APIAdapter) TriggerWorkflowWithRetry(ctx context.Context, imageID, destImageID, arch, osType string) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())

	runID, err := a.TriggerWorkflowSimple(ctx, imageID, destImageID, arch, osType)
	if err == nil {
		return runID, nil
	}

	err = retry.Retry(ctx, retry.DefaultConfig(), func() error {
		_, err := a.TriggerWorkflowSimple(ctx, imageID, destImageID, arch, osType)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}

	return a.findWorkflowRunID(ctx, a.owner, a.repo, a.workflowID, imageID, destImageID, suffix)
}

func (a *APIAdapter) TriggerWorkflow(ctx context.Context, owner, repo, workflowID string, inputs map[string]string) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())

	data := map[string]interface{}{
		"ref": "master",
		"inputs": map[string]string{
			"imageId":     inputs["imageId"],
			"destImageId": inputs["destImageId"],
			"suffix":      suffix,
			"arch":        inputs["arch"],
			"os":          inputs["os"],
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", errors.NewGitHubError("TriggerWorkflow", "failed to marshal data", 0, err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches",
		owner, repo, workflowID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", errors.NewGitHubError("TriggerWorkflow", "failed to create request", 0, err)
	}

	req.Header.Set("Accept", GitHubMediaType)
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", errors.NewGitHubError("TriggerWorkflow", "failed to send request", 0, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return a.findWorkflowRunID(ctx, owner, repo, workflowID, inputs["imageId"], inputs["destImageId"], suffix)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", errors.NewGitHubError("TriggerWorkflow", "retryable error", resp.StatusCode, nil)
	}

	return "", errors.NewGitHubError("TriggerWorkflow", fmt.Sprintf("unexpected status: %d", resp.StatusCode), resp.StatusCode, nil)
}

func (a *APIAdapter) GetWorkflowStatus(ctx context.Context, owner, repo, runID string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s",
		owner, repo, runID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", errors.NewGitHubError("GetWorkflowStatus", "failed to create request", 0, err)
	}

	req.Header.Set("Accept", GitHubMediaType)
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", errors.NewGitHubError("GetWorkflowStatus", "failed to send request", 0, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return "", errors.NewGitHubError("GetWorkflowStatus", "retryable error", resp.StatusCode, nil)
		}
		return "", errors.NewGitHubError("GetWorkflowStatus", fmt.Sprintf("unexpected status: %d", resp.StatusCode), resp.StatusCode, nil)
	}

	var result struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", errors.NewGitHubError("GetWorkflowStatus", "failed to decode response", resp.StatusCode, err)
	}

	if result.Status == "completed" {
		return result.Conclusion, nil
	}

	return result.Status, nil
}

func (a *APIAdapter) WaitForWorkflow(ctx context.Context, owner, repo, runID string) error {
	const (
		maxRetries   = 300
		pollInterval = 2 * time.Second
	)

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}

		conclusion, err := a.GetWorkflowStatus(ctx, owner, repo, runID)
		if err != nil {
			return err
		}

		if conclusion == "success" {
			return nil
		}
		if conclusion != "" && conclusion != "in_progress" && conclusion != "queued" {
			return fmt.Errorf("workflow failed with conclusion: %s", conclusion)
		}
	}

	return fmt.Errorf("workflow timed out after %d attempts", maxRetries)
}

func (a *APIAdapter) FindWorkflowRunID(ctx context.Context, owner, repo, workflowID, sourceID, destImageID, suffix string) (string, error) {
	return a.findWorkflowRunID(ctx, owner, repo, workflowID, sourceID, destImageID, suffix)
}

func (a *APIAdapter) findWorkflowRunID(ctx context.Context, owner, repo, workflowID, sourceID, destImageID, suffix string) (string, error) {
	expectedName := fmt.Sprintf("copy %s to %s%s", sourceID, destImageID, suffix)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs",
		owner, repo, workflowID)

	timeoutCtx, cancel := context.WithTimeout(ctx, WorkflowPollTimeout)
	defer cancel()

	ticker := time.NewTimer(0)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", errors.NewGitHubError("findWorkflowRunID", "workflow run not found after timeout", 0, nil)
		case <-ticker.C:
			req, err := http.NewRequestWithContext(timeoutCtx, "GET", url, nil)
			if err != nil {
				ticker.Reset(time.Second)
				continue
			}

			req.Header.Set("Accept", GitHubMediaType)
			req.Header.Set("Authorization", "Bearer "+a.token)
			req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)

			resp, err := a.httpClient.Do(req)
			if err != nil {
				ticker.Reset(time.Second)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var result struct {
					WorkflowRuns []struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					} `json:"workflow_runs"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					ticker.Reset(time.Second)
					continue
				}

				for _, run := range result.WorkflowRuns {
					if run.Name == expectedName {
						return fmt.Sprintf("%d", run.ID), nil
					}
				}
			}

			ticker.Reset(time.Second)
		}
	}
}

var _ ports.GitHubClient = (*APIAdapter)(nil)
