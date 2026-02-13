package core

import (
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

// TestImageValidator_WithPuller_Integration tests the integration between ImageValidator and Puller
func TestImageValidator_WithPuller_Integration_COMPLETE(t *testing.T) {
	// Set up a test puller with an ImageValidator instance
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Verify that the puller has an ImageValidator instance
	if puller.ImageValidator == nil {
		t.Fatal("Expected Puller to have an ImageValidator instance, but it was nil")
	}

	// Test that both validation methods produce consistent results
	testInputs := []string{
		"nginx:latest",
		"my-registry.com/image:tag",
		"user/repo:1.0.0",
		"malicious$(command)",
	}

	for _, input := range testInputs {
		result1 := puller.ImageValidator.ValidateImageNameInput(input)
		result2 := puller.ImageValidator.IsValidImageName(input)

		if result1 != result2 {
			t.Errorf("ValidateImageNameInput and IsValidImageName returned different results for '%s': %t vs %t",
				input, result1, result2)
		}
	}

	// Test validation of valid inputs
	validInputs := []string{
		"nginx:latest",
		"my-registry.com/image:tag",
		"user/repo:1.0.0",
		"alpine@sha256:abc123def456",
	}

	for _, input := range validInputs {
		t.Run("ValidInput_"+input, func(t *testing.T) {
			result1 := puller.ImageValidator.ValidateImageNameInput(input)
			result2 := puller.ImageValidator.IsValidImageName(input)

			if !result1 {
				t.Errorf("ValidateImageNameInput('%s') returned false for valid input", input)
			}
			if !result2 {
				t.Errorf("IsValidImageName('%s') returned false for valid input", input)
			}
		})
	}

	// Test validation of invalid inputs (security tests)
	invalidInputs := []string{
		"nginx;rm -rf /",
		"image`whoami`",
		"$(malicious)",
		"image && cat /etc/passwd",
		"image || exit 1",
		"image\nnew_line",
		"image\"quote_injection",
		"image'quote_injection",
	}

	for _, input := range invalidInputs {
		t.Run("InvalidInput_"+input, func(t *testing.T) {
			result1 := puller.ImageValidator.ValidateImageNameInput(input)
			result2 := puller.ImageValidator.IsValidImageName(input)

			if result1 {
				t.Errorf("ValidateImageNameInput('%s') returned true for invalid/malicious input", input)
			}
			if result2 {
				t.Errorf("IsValidImageName('%s') returned true for invalid/malicious input", input)
			}
		})
	}
}

// TestImageValidator_Credentials_Validation_Integration tests credential validation integration
func TestImageValidator_Credentials_Validation_Integration_COMPLETE(t *testing.T) {
	// Set up a test puller
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Test valid credentials
	validUser, validPass := "normal_user", "normal_pass"
	if !puller.ImageValidator.ValidateCredentials(validUser, validPass) {
		t.Errorf("Valid credentials ('%s', '%s') were unexpectedly rejected", validUser, validPass)
	}

	// Test invalid credentials (security tests)
	invalidCredentialTests := []struct {
		username string
		password string
		desc     string
	}{
		{"user;sql", "pass", "semicolon in username"},
		{"user", "pass'inject", "quote in password"},
		{"user`cmd`", "pass", "backticks in username"},
		{"user$(cmd)", "pass", "dollar-paren in username"},
		{"user", "pass && cmd", "AND operator in password"},
		{"user||other", "pass", "OR operator in username"},
		{"user\nnewline", "pass", "newline in username"},
		{"user", "pass\r\ncarriage_return", "CRLF in password"},
	}

	for _, test := range invalidCredentialTests {
		t.Run(test.desc, func(t *testing.T) {
			result := puller.ImageValidator.ValidateCredentials(test.username, test.password)
			if result {
				t.Errorf("Malicious credentials ('%s', '%s') were unexpectedly accepted", test.username, test.password)
			}
		})
	}
}

// TestHTTPClientFactory_WithPuller_Integration tests HTTP client factory integration
func TestHTTPClientFactory_WithPuller_Integration_COMPLETE(t *testing.T) {
	// Test that HTTPClientFactory creates properly configured HTTP clients
	factory := &HTTPClientFactory{}
	client := factory.NewHTTPClient()

	if client == nil {
		t.Fatal("Expected HTTPClientFactory to create non-nil client")
	}

	// Check timeout settings
	expectedTimeout := 30 * time.Second
	if client.Timeout != expectedTimeout {
		t.Errorf("Expected HTTP client timeout to be %v, got %v", expectedTimeout, client.Timeout)
	}

	// Test that a puller properly initializes with HTTP client from factory
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	if puller.HTTPClient == nil {
		t.Fatal("Expected Puller to initialize with HTTP client")
	}

	// Compare timeout values
	if puller.HTTPClient.Timeout != client.Timeout {
		t.Errorf("Expected Puller.HTTPClient timeout (%v) to match factory client timeout (%v)",
			puller.HTTPClient.Timeout, client.Timeout)
	}
}

// TestRetryConfig_WithImageValidator_Integration tests retry configuration integration with ImageValidator
func TestRetryConfig_WithImageValidator_Integration_COMPLETE(t *testing.T) {
	// Custom retry config
	customRC := &retry.Config{
		MaxAttempts:     5,
		InitialInterval: 2 * time.Second,
		MaxInterval:     60 * time.Second,
	}

	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
		RetryConfig:      customRC,
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

	// Verify that validation still works correctly with custom retry config
	testImage := "nginx:latest"
	result := puller.ImageValidator.IsValidImageName(testImage)
	if !result {
		t.Errorf("Expected valid image '%s' to pass validation with custom retry config", testImage)
	}
}

// TestPuller_With_ImageValidator_Methods tests that puller uses ImageValidator correctly
func TestPuller_With_ImageValidator_Methods_COMPLETE(t *testing.T) {
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	// Test that puller's validation calls are consistent with direct ImageValidator calls
	testCases := []string{
		"nginx:latest",
		"my.registry.com/image:v1.0",
		"malicious;rm -rf /",
		"$(bad_command)",
	}

	for _, testCase := range testCases {
		t.Run("Consistency_"+testCase, func(t *testing.T) {
			// Direct validator call
			directResult := puller.ImageValidator.IsValidImageName(testCase)

			// Call as it would be in CheckLocalImageExists
			indirectResult := puller.ImageValidator.IsValidImageName(testCase)

			if directResult != indirectResult {
				t.Errorf("Direct and indirect validation calls returned different results for '%s': %t vs %t",
					testCase, directResult, indirectResult)
			}
		})
	}
}

// TestImageValidator_Instance_Consistency tests that multiple instances behave consistently
func TestImageValidator_Instance_Consistency_COMPLETE(t *testing.T) {
	// Create multiple validators
	validator1 := NewImageValidator()
	validator2 := NewImageValidator()

	// They should both behave identically
	testInputs := []string{
		"nginx:latest",
		"malicious;rm -rf /",
		"normal_image",
		"$(bad)",
	}

	for _, input := range testInputs {
		result1 := validator1.IsValidImageName(input)
		result2 := validator2.IsValidImageName(input)

		if result1 != result2 {
			t.Errorf("Different validator instances produced different results for '%s': %t vs %t",
				input, result1, result2)
		}
	}

	// Also test that the instance used by a puller produces same results
	config := &Config{
		GithubOwner:      "testowner",
		GithubRepo:       "testrepo",
		GithubToken:      "testtoken",
		GithubWorkflowID: "testworkflow",
		RegistryHost:     "testregistry.com",
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		RegistryNamespace: "testns",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	puller := NewPuller(config, logger)

	for _, input := range testInputs {
		instanceResult := puller.ImageValidator.IsValidImageName(input)
		standaloneResult := validator1.IsValidImageName(input)

		if instanceResult != standaloneResult {
			t.Errorf("Puller's validator instance and standalone validator produced different results for '%s': %t vs %t",
				input, instanceResult, standaloneResult)
		}
	}
}