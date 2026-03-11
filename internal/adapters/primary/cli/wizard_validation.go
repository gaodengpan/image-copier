package cli

import (
	"context"
	"fmt"
	"net/http"
)

// validateGitHubToken validates a GitHub token by making a simple API call
func validateGitHubToken(ctx context.Context, owner, repo, token string) error {
	if owner == "" || repo == "" {
		return fmt.Errorf("owner and repo are required for token validation")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found")
	}

	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
