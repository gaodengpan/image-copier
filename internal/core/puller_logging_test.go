package core

import (
	"context"
	"strings"
	"testing"
)

// TestCheckCredentialsNotLogged verifies that sensitive credentials are not logged in plain text
func TestCheckCredentialsNotLogged(t *testing.T) {
	// Setup puller with sensitive credentials and capture logs
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "SECRET_TOKEN_12345",
		GithubWorkflowID: "workflow.yaml",
		RegistryHost:     "registry.example.com",
		RegistryUsername: "secret_user",
		RegistryPassword: "secret_password",
	}

	puller, logHook := newTestPullerWithLogHook(config)

	// Attempt operations that might leak credentials in logs
	ctx := context.Background()
	imageName := "test/image:latest"

	_, err := puller.CheckLocalImageExists(ctx, imageName)
	if err != nil {
		t.Logf("Got expected error during test: %v", err)
	}

	// Verify that no sensitive information appears in logs
	secrets := []string{
		"SECRET_TOKEN_12345",
		"secret_user",
		"secret_password",
	}

	if logHook.HasSensitiveInfo(secrets...) {
		t.Errorf("FAILED: Sensitive credentials were logged in plain text")
		// Print the actual logs for debugging
		for _, entry := range logHook.GetEntries() {
			if containsAnySecret(entry.Message, secrets) {
				t.Errorf("Found secret in log: %s", entry.Message)
			}
		}
	}
}

// TestCheckErrorMessagesDoNotContainCredentials verifies that error messages don't expose sensitive information
func TestCheckErrorMessagesDoNotContainCredentials(t *testing.T) {
	// Setup puller with sensitive credentials
	config := &Config{
		GithubToken:      "sensitive_github_token_abc123",
		RegistryUsername: "admin_user",
		RegistryPassword: "super_secret_password",
		RegistryHost:     "registry.example.com",
	}

	puller, _ := newTestPullerWithLogHook(config)

	// Call a function that might expose credentials in error messages
	ctx := context.Background()
	invalidImage := "invalid$image:name" // This should trigger validation errors

	_, err := puller.CheckLocalImageExists(ctx, invalidImage)
	if err != nil {
		// Check if credentials appear in the error message
		secrets := []string{
			"sensitive_github_token_abc123",
			"admin_user",
			"super_secret_password",
		}

		if SanitizeErrorMessage(err, secrets...) {
			t.Errorf("FAILED: Sensitive credentials were exposed in error message: %s", err.Error())
		}
	} else {
		t.Log("Operation succeeded, no credential leakage detected in error message")
	}
}

// TestTriggerWorkflowDoesNotLogCredentials verifies that workflow triggering doesn't leak credentials in logs
func TestTriggerWorkflowDoesNotLogCredentials(t *testing.T) {
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "REAL_SECRET_TOKEN_XYZ789",
		GithubWorkflowID: "workflow.yaml",
	}

	puller, logHook := newTestPullerWithLogHook(config)

	// Attempt to trigger workflow (this might fail, but we're testing for credential leaks)
	ctx := context.Background()
	_, err := puller.triggerWorkflow(ctx, "source/image:tag", "dest/image:tag")

	// Check logs regardless of whether the operation succeeded
	secrets := []string{"REAL_SECRET_TOKEN_XYZ789"}

	if logHook.HasSensitiveInfo(secrets...) {
		t.Errorf("FAILED: Sensitive credentials appeared in workflow logs")
		// Print the actual logs for debugging
		for _, entry := range logHook.GetEntries() {
			if containsAnySecret(entry.Message, secrets) {
				t.Errorf("Found secret in log: %s", entry.Message)
			}
		}
	}

	if err != nil {
		// Even if there's an error, credentials shouldn't appear in the error message
		if SanitizeErrorMessage(err, secrets...) {
			t.Errorf("FAILED: Token leaked in workflow error: %s", err.Error())
		}
	}
}

// containsAnySecret checks if a string contains any of the provided secrets
func containsAnySecret(text string, secrets []string) bool {
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			return true
		}
	}
	return false
}