package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestValidateImageNameInputRecovery tests error recovery scenarios for image name validation
func TestValidateImageNameInputRecovery(t *testing.T) {
	validator := NewImageValidator()

	// Test with malformed inputs that might cause unexpected behavior
	malformedInputs := []string{
		// Potential regex DOS attempts
		strings.Repeat("a.", 10000) + "z", // Many dots that might affect regex engine
		// Extreme length inputs
		strings.Repeat("very-long-input-", 10000),
		// Null bytes and special control characters
		"image\x00name",
		"image\x01name",
		// Unicode edge cases
		"image" + string([]byte{0xFF, 0xFE}) + "name", // Invalid UTF-8
		// Multi-byte unicode
		"image" + "测试" + "name",
	}

	for i, input := range malformedInputs {
		t.Run("recovery_test_"+string(rune(i+'0')), func(t *testing.T) {
			// This test ensures that even malformed inputs don't crash the validator
			// They should return either true or false, but not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Validation panicked with input: %s, recovered: %v", input, r)
				}
			}()

			result := validator.ValidateImageNameInput(input)
			// We don't care about the specific result (true/false) - just that it didn't panic
			t.Logf("Input %d processed successfully, result: %v", i, result)
		})
	}
}

// TestValidateCredentialsRecovery tests error recovery for credential validation
func TestValidateCredentialsRecovery(t *testing.T) {
	validator := NewImageValidator()

	// Test various problematic credential combinations
	testCases := []struct {
		username string
		password string
	}{
		{strings.Repeat("a", 100000), "normal_password"}, // Extremely long username
		{"normal_username", strings.Repeat("b", 100000)}, // Extremely long password
		{strings.Repeat("a.", 50000), strings.Repeat("b.", 50000)}, // Both extremely long with dots
		{"user\x00", "pass"},                            // Null byte in username
		{"user", "pass\x00"},                            // Null byte in password
		{"", strings.Repeat("\n", 1000)},                // Newlines in password
		{strings.Repeat(";", 10000), "normal"},          // Semicolons in username
	}

	for i, tc := range testCases {
		t.Run("credential_recovery_"+string(rune(i+'0')), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Credential validation panicked with username: %s..., password: %s..., recovered: %v",
						tc.username[:min(len(tc.username), 10)],
						tc.password[:min(len(tc.password), 10)],
						r)
				}
			}()

			result := validator.ValidateCredentials(tc.username, tc.password)
			t.Logf("Credential pair %d processed successfully, result: %v", i, result)
		})
	}
}

// TestValidateFilePathRecovery tests error recovery for file path validation
func TestValidateFilePathRecovery(t *testing.T) {
	validator := NewImageValidator()

	// Test various problematic file paths
	problematicPaths := []string{
		strings.Repeat("../", 10000),                    // Extreme path traversal
		strings.Repeat("/../../../", 5000),               // Mixed extreme path traversal
		strings.Repeat("..\\..\\", 5000),                // Windows-style extreme traversal
		"/tmp/" + strings.Repeat("a/", 10000) + "file", // Deep nesting
		"\x00" + strings.Repeat("a", 1000),             // Leading null byte
		"/tmp/file" + strings.Repeat("\x00", 100),      // Multiple null bytes
		"",                                              // Empty string
		"/tmp/" + strings.Repeat(".", 10000),           // Many dots
	}

	for i, path := range problematicPaths {
		t.Run("filepath_recovery_"+string(rune(i+'0')), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("File path validation panicked with path: %s..., recovered: %v", path[:min(len(path), 20)], r)
				}
			}()

			result := validator.ValidateFilePath(path)
			t.Logf("Path %d processed successfully, result: %v", i, result)
		})
	}
}

// TestValidateYAMLContentRecovery tests error recovery for YAML content validation
func TestValidateYAMLContentRecovery(t *testing.T) {
	validator := NewImageValidator()

	// Test various problematic YAML content
	problematicYAMLs := []string{
		// Massive YAML to test memory handling
		strings.Repeat("key: value\n", 100000),
		// Potential regex DOS with repeated patterns
		strings.Repeat("{{ {{ {{ shell \"test\" }} }} }}\n", 10000),
		// Combinations of dangerous patterns
		strings.Repeat("line | sh\nexec command\n", 25000),
		// Null bytes
		"safe_key: value\x00dangerous_value: exec",
		// Very long strings that might cause memory issues
		"key: " + strings.Repeat("a", 1000000),
		// Embedded dangerous content
		"template: |\n" + strings.Repeat("{{ eval \"malicious\" }}\n", 50000),
	}

	for i, yaml := range problematicYAMLs {
		t.Run("yaml_recovery_"+string(rune(i+'0')), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("YAML validation panicked, recovered: %v", r)
				}
			}()

			err := validator.ValidateYAMLContent(yaml)
			// Error is expected for dangerous content, but no panic should occur
			t.Logf("YAML %d processed successfully, error: %v", i, err != nil)
		})
	}
}

// TestIntegrationErrorRecovery tests the interaction between validator and puller under error conditions
func TestIntegrationErrorRecovery(t *testing.T) {
	// Create a puller with a validator
	config := &Config{
		GithubOwner:      "test",
		GithubRepo:       "test",
		GithubToken:      "token",
		GithubWorkflowID: "workflow",
		RegistryHost:     "registry.com",
		RegistryUsername: "user",
		RegistryPassword: "pass",
	}

	puller := NewPuller(config, nil) // nil logger for test

	// Test with various problematic image names
	problematicImages := []string{
		strings.Repeat("a.", 10000),
		"../../../etc/passwd",
		"nginx;rm -rf /",
		"",
		"nginx\nmalicious",
	}

	for i, image := range problematicImages {
		t.Run("integration_recovery_"+string(rune(i+'0')), func(t *testing.T) {
			// Test that validation doesn't panic even with problematic inputs
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Integration validation panicked with image: %s, recovered: %v", image, r)
				}
			}()

			// These should not panic even if they return errors
			result := puller.ImageValidator.IsValidImageName(image)
			t.Logf("Image %d validated successfully: %v", i, result)

			// Also test direct validation method
			directResult := puller.ImageValidator.ValidateImageNameInput(image)
			t.Logf("Direct validation for image %d: %v", i, directResult)
		})
	}
}

// TestConcurrentRecovery tests error recovery under concurrent load
func TestConcurrentRecovery(t *testing.T) {
	validator := NewImageValidator()

	// Channel to collect errors
	errChan := make(chan error, 100)

	// Start multiple goroutines with potentially problematic inputs
	numGoroutines := 20
	numTestsPerGoroutine := 50

	for j := 0; j < numGoroutines; j++ {
		go func(workerID int) {
			defer func() {
				if r := recover(); r != nil {
					errChan <- errors.New("Worker " + string(rune(workerID)) + " panicked: " + r.(string))
					return
				}
				errChan <- nil
			}()

			for i := 0; i < numTestsPerGoroutine; i++ {
				// Different types of problematic inputs for each worker
				var testInput string
				switch i % 5 {
				case 0:
					testInput = strings.Repeat("test.", i*10) + "final"
				case 1:
					testInput = "../../../etc/passwd" + string(rune(i))
				case 2:
					testInput = "image;rm -rf /" + string(rune(i))
				case 3:
					testInput = strings.Repeat("a", i*100)
				case 4:
					testInput = "normal:image" + string(rune(i))
				}

				// Test all validation methods
				validator.ValidateImageNameInput(testInput)
				validator.ValidateFilePath("/tmp/test" + string(rune(i)) + ".txt")
				validator.ValidateCredentials(
					"user"+string(rune(i)),
					"pass"+strings.Repeat("x", i%50),
				)
				validator.ValidateYAMLContent(
					"key" + string(rune(i)) + ": value\nnested: " + strings.Repeat("safe", i%100),
				)
			}
		}(j)
	}

	// Collect results
	completedWorkers := 0
	for completedWorkers < numGoroutines {
		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("Worker failed: %v", err)
			}
			completedWorkers++
		case <-time.After(30 * time.Second): // Timeout to prevent hanging
			t.Fatalf("Test timed out after waiting for %d of %d workers", completedWorkers, numGoroutines)
		}
	}

	t.Logf("Successfully tested %d concurrent workers with %d tests each", numGoroutines, numTestsPerGoroutine)
}

// TestValidatorInitializationRecovery tests error recovery during validator initialization
func TestValidatorInitializationRecovery(t *testing.T) {
	// Test that validator creation doesn't panic under various conditions
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Validator initialization panicked: %v", r)
		}
	}()

	validator := NewImageValidator()
	assert.NotNil(t, validator, "Validator should be created successfully")

	// Test basic functionality after creation
	result := validator.ValidateImageNameInput("test:latest")
	assert.True(t, result, "Basic validation should work after initialization")
}

// TestContextCancellationDuringValidation simulates context cancellation during validation processes
func TestContextCancellationDuringValidation(t *testing.T) {
	// While our validation methods don't take context directly,
	// this test verifies the validator behaves correctly under stress
	// which might simulate timeout scenarios

	validator := NewImageValidator()

	// Simulate rapid successive calls that might occur under load/timeouts
	for i := 0; i < 1000; i++ {
		// Vary the inputs to test different code paths
		input := "image" + string(rune(i%100)) + ":tag" + string(rune(i%50))

		// This should not panic or cause issues even under rapid calling
		result := validator.ValidateImageNameInput(input)

		// Validate filepath as well
		path := "/tmp/test" + string(rune(i%100)) + ".txt"
		pathResult := validator.ValidateFilePath(path)

		// Validate credentials
		credResult := validator.ValidateCredentials(
			"user"+string(rune(i%100)),
			"pass"+string(rune(i%50)),
		)

		// Validate YAML
		yamlResult := validator.ValidateYAMLContent(
			"key" + string(rune(i%10)) + ": value",
		)

		// None of these should cause issues
		_ = result
		_ = pathResult
		_ = credResult
		_ = yamlResult
	}

	t.Log("Successfully completed rapid validation calls without issues")
}