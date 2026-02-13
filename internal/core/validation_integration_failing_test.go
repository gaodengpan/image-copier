package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

// TestImageValidator_WithPuller_Integration tests that ImageValidator properly integrates with Puller
func TestImageValidator_WithPuller_Integration_FAILING(t *testing.T) {
	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "testuser",
		RegistryPassword:  "testpass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Verify that the puller has an ImageValidator instance
	if puller.ImageValidator == nil {
		t.Fatal("Expected Puller to have an ImageValidator instance, but it was nil")
	}

	// Test that validation works through the integrated components
	testImage := "nginx:latest"
	result := puller.ImageValidator.IsValidImageName(testImage)
	if !result {
		t.Errorf("Expected valid image '%s' to pass validation through integrated components", testImage)
	}
}

// TestImageValidator_PullerIntegration_CheckLocalImageExists tests validation integration in CheckLocalImageExists
func TestImageValidator_PullerIntegration_CheckLocalImageExists_FAILING(t *testing.T) {
	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "testuser",
		RegistryPassword:  "testpass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Test that CheckLocalImageExists properly validates input through ImageValidator
	_, err := puller.CheckLocalImageExists(context.Background(), "valid-image:tag")

	// Since we expect an error due to missing docker command (not validation),
	// we just verify that validation didn't reject the valid input
	// The absence of a validation error means the input passed through ImageValidator
	if err != nil {
		if strings.Contains(err.Error(), "invalid image name") {
			t.Error("CheckLocalImageExists rejected a valid image name, indicating integration issue with ImageValidator")
		}
		// Other errors are expected due to environment (missing docker, etc.)
	}
}

// TestImageValidator_PullerIntegration_PullSingle_WithValidation tests validation in PullSingle method
func TestImageValidator_PullerIntegration_PullSingle_WithValidation_FAILING(t *testing.T) {
	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "testuser",
		RegistryPassword:  "testpass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Test that PullSingle properly validates input through ImageValidator
	err := puller.PullSingle(context.Background(), "valid-image:tag")

	// We expect an error here (due to workflow not actually existing), but it shouldn't be a validation error
	if err != nil && strings.Contains(err.Error(), "invalid image name") {
		t.Error("PullSingle rejected a valid image name, indicating integration issue with ImageValidator")
	}
}

// TestImageValidator_PullerIntegration_Credentials_Validation tests credential validation integration
func TestImageValidator_PullerIntegration_Credentials_Validation_FAILING(t *testing.T) {
	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "normal_user",
		RegistryPassword:  "normal_pass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Test that credential validation properly rejects malicious credentials
	if puller.ImageValidator.ValidateCredentials("user;rm -rf /", "pass") {
		t.Error("Expected credential validation to reject malicious username with semicolon")
	}

	if puller.ImageValidator.ValidateCredentials("user", "pass\nnew_line") {
		t.Error("Expected credential validation to reject malicious password with newline")
	}
}

// TestImageValidator_HTTPClientFactory_Integration tests the HTTP client factory integration
func TestImageValidator_HTTPClientFactory_Integration_FAILING(t *testing.T) {
	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "testuser",
		RegistryPassword:  "testpass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Verify that HTTP client is properly configured
	if puller.HTTPClient == nil {
		t.Fatal("Expected Puller to have an HTTP client instance")
	}

	// Verify timeout configuration
	expectedTimeout := 30 * time.Second
	if puller.HTTPClient.Timeout != expectedTimeout {
		t.Errorf("Expected HTTP client timeout to be %v, got %v", expectedTimeout, puller.HTTPClient.Timeout)
	}
}

// TestRetryConfig_Integration_WithValidator tests retry configuration integration
func TestRetryConfig_Integration_WithValidator_FAILING(t *testing.T) {
	// Custom retry config
	customRC := &retry.Config{
		MaxAttempts:     5,
		InitialInterval: 2 * time.Second,
		MaxInterval:     60 * time.Second,
	}

	config := &Config{
		GithubOwner:       "testowner",
		GithubRepo:        "testrepo",
		GithubToken:       "testtoken",
		GithubWorkflowID:  "testworkflow",
		RegistryHost:      "testregistry.com",
		RegistryUsername:  "testuser",
		RegistryPassword:  "testpass",
		RegistryNamespace: "testns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
		RetryConfig:       customRC,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Verify that the puller uses the custom retry config
	if puller.RetryConfig.MaxAttempts != customRC.MaxAttempts {
		t.Errorf("Expected MaxAttempts to be %d, got %d", customRC.MaxAttempts, puller.RetryConfig.MaxAttempts)
	}

	// Verify that ImageValidator is still properly set despite custom retry config
	if puller.ImageValidator == nil {
		t.Error("Expected ImageValidator to be set when using custom retry config")
	}
}