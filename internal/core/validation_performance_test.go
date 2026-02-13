package core

import (
	"testing"
	"time"
)

// BenchmarkValidateImageNameInput tests the performance of image name validation
func BenchmarkValidateImageNameInput(b *testing.B) {
	validator := NewImageValidator()
	testCases := []string{
		"nginx:latest",
		"docker.io/library/nginx:latest",
		"example.com/myapp:v1.2.3",
		"nginx@sha256:abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def123",
		"nginx;rm -rf /",
		"../../../etc/passwd",
		"normal-image:tag",
		"very-long-image-name-with-many-segments-and-tags:very-long-tag-name-that-exceeds-normal-length",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			validator.ValidateImageNameInput(testCase)
		}
	}
}

// BenchmarkValidateCredentials tests the performance of credential validation
func BenchmarkValidateCredentials(b *testing.B) {
	validator := NewImageValidator()
	usernames := []string{"user1", "admin", "test_user", "user;malicious", ""}
	passwords := []string{"password123", "secret!@#", "longpasswordwithspecialchars!@#$%^&*()", "pass;rm -rf /", ""}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, username := range usernames {
			for _, password := range passwords {
				validator.ValidateCredentials(username, password)
			}
		}
	}
}

// BenchmarkValidateFilePath tests the performance of file path validation
func BenchmarkValidateFilePath(b *testing.B) {
	validator := NewImageValidator()
	paths := []string{
		"/tmp/file.txt",
		"/var/tmp/another_file.log",
		"../../../etc/passwd",
		"..\\..\\windows\\system32",
		"/normal/path/to/file",
		"/tmp/file\x00",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			validator.ValidateFilePath(path)
		}
	}
}

// BenchmarkValidateYAMLContent tests the performance of YAML content validation
func BenchmarkValidateYAMLContent(b *testing.B) {
	validator := NewImageValidator()
	yamlContents := []string{
		"key: value",
		"list:\n  - item1\n  - item2",
		"{{ shell \"rm -rf /\" }}",
		"cmd | sh",
		"safe: yaml\nwith: content",
		"{{ eval \"malicious_code\" }}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, content := range yamlContents {
			validator.ValidateYAMLContent(content)
		}
	}
}

// TestValidateImageNameInputStress performs stress testing with large numbers of inputs
func TestValidateImageNameInputStress(t *testing.T) {
	validator := NewImageValidator()

	// Create a large number of test cases
	testInputs := make([]string, 10000)
	for i := 0; i < 5000; i++ {
		testInputs[i] = "nginx:latest"                          // Valid
		testInputs[i+5000] = "nginx;" + string(rune(i%65536)) // Invalid - potential injection
	}

	startTime := time.Now()
	for _, input := range testInputs {
		validator.ValidateImageNameInput(input)
	}
	duration := time.Since(startTime)

	// Ensure processing doesn't take too long (less than 1 second for 10k inputs)
	if duration > time.Second {
		t.Errorf("Stress test took too long: %v for %d inputs", duration, len(testInputs))
	}

	t.Logf("Processed %d inputs in %v", len(testInputs), duration)
}

// TestValidateCredentialsStress performs stress testing on credential validation
func TestValidateCredentialsStress(t *testing.T) {
	validator := NewImageValidator()

	// Create test credentials
	usernames := make([]string, 1000)
	passwords := make([]string, 1000)

	for i := 0; i < 500; i++ {
		usernames[i] = "user" + string(rune(i))
		passwords[i] = "password" + string(rune(i))
		usernames[i+500] = "user" + string(rune(i)) + ";malicious"
		passwords[i+500] = "pass" + string(rune(i)) + ";rm -rf /"
	}

	startTime := time.Now()
	for i := 0; i < 1000; i++ {
		validator.ValidateCredentials(usernames[i], passwords[i])
	}
	duration := time.Since(startTime)

	// Ensure processing doesn't take too long
	if duration > 500*time.Millisecond {
		t.Errorf("Credential stress test took too long: %v for %d inputs", duration, 1000)
	}

	t.Logf("Processed %d credential pairs in %v", 1000, duration)
}

// TestConcurrentValidation tests validation under concurrent load
func TestConcurrentValidation(t *testing.T) {
	validator := NewImageValidator()

	// Channel to collect results
	results := make(chan bool, 100)

	// Spawn multiple goroutines
	for j := 0; j < 10; j++ {
		go func(workerID int) {
			for i := 0; i < 100; i++ {
				testInput := "worker" + string(rune(workerID)) + "input" + string(rune(i))

				// Test multiple validation methods concurrently
				imgResult := validator.ValidateImageNameInput(testInput)
				fileResult := validator.ValidateFilePath("/tmp/test" + string(rune(i)) + ".txt")
				credResult := validator.ValidateCredentials("user"+string(rune(i)), "pass"+string(rune(i)))

				// Send a composite result to ensure all validations passed
				results <- (imgResult && fileResult && credResult)
			}
		}(j)
	}

	// Collect results
	totalResults := 0
	for totalResults < 1000 {
		select {
		case <-results:
			totalResults++
		case <-time.After(10 * time.Second): // Timeout to prevent hanging
			t.Fatal("Test timed out - possible deadlock or excessive runtime")
		}
	}

	t.Logf("Successfully completed %d concurrent validation operations", totalResults)
}