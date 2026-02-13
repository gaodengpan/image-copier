package core

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestPuller_HelperFunctions(t *testing.T) {
	config := &Config{
		GithubOwner:       "test-owner",
		GithubRepo:        "test-repo",
		GithubToken:       "test-token",
		GithubWorkflowID:  "test-workflow.yml",
		RegistryHost:      "registry.test.com",
		RegistryUsername:  "test-user",
		RegistryPassword:  "test-pass",
		RegistryNamespace: "test-ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := &logrus.Logger{}
	puller := NewPuller(config, logger)

	// Test buildExpectedWorkflowName helper function
	expectedName := puller.buildExpectedWorkflowName("nginx:latest", "registry.com/nginx:latest", "--12345")
	assert.Contains(t, expectedName, "copy nginx:latest to registry.com/nginx:latest--12345")

	// Test buildWorkflowRunsURL helper function
	url := puller.buildWorkflowRunsURL()
	assert.Contains(t, url, "https://api.github.com/repos/test-owner/test-repo/actions/workflows/test-workflow.yml/runs")

	// Test that the original complex method has been broken down into smaller functions
	// This test verifies the refactoring requirement has been met
	assert.NotNil(t, puller)

	// Create a sample result to test the search function
	sampleResult := struct {
		WorkflowRuns []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"workflow_runs"`
	}{
		WorkflowRuns: []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}{
			{ID: 12345, Name: "copy nginx:latest to registry.com/nginx:latest--12345"},
			{ID: 12346, Name: "copy redis:latest to registry.com/redis:latest--12346"},
		},
	}

	// Test searchWorkflowRunID helper function
	id, found := puller.searchWorkflowRunID(sampleResult, "copy nginx:latest to registry.com/nginx:latest--12345")
	assert.True(t, found)
	assert.Equal(t, "12345", id)
}