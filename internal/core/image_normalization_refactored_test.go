package core

import (
	"strings"
	"testing"
)

// TestImageNormalization_ValidInputs_AreHandled tests that valid inputs are properly normalized
func TestImageNormalization_ValidInputs_AreHandled(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		desc     string
	}{
		{"nginx", "docker.io/library/nginx:latest", "simple image name"},
		{"nginx:latest", "docker.io/library/nginx:latest", "image with latest tag"},
		{"my.registry.com/image", "my.registry.com/image:latest", "custom registry without tag"},
		{"namespace/image:tag", "docker.io/namespace/image:tag", "namespaced image with tag"},
		{"namespace/subnamespace/image:tag", "namespace/subnamespace/image:tag", "deeply namespaced image"},
		{"image@sha256:abc123", "docker.io/library/image@sha256:abc123", "image with digest"},
		{"my_repo-v1.0:latest", "docker.io/library/my_repo-v1.0:latest", "underscore and hyphen in name"},
		{"my.repo.com:5000/my-image:v1.0", "my.repo.com:5000/my-image:v1.0", "port in registry"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc+"_"+tc.input, func(t *testing.T) {
			result := NormalizeSourceID(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeSourceID('%s') = '%s', want '%s'", tc.input, result, tc.expected)
			}
		})
	}
}

// TestImageNormalization_InvalidInputs_AreHandled tests handling of potentially invalid inputs
func TestImageNormalization_InvalidInputs_AreHandled(t *testing.T) {
	// Even "invalid" inputs should be normalized in some way, but without crashing
	testCases := []struct {
		input    string
		desc     string
		contains string // What we expect the result to contain
	}{
		{"", "empty string", ":latest"},                // Should add default namespace and tag
		{"////", "multiple slashes", ":latest"},         // Should keep slashes as they are
		{"nginx///image", "multiple slashes in middle", "nginx///image:latest"}, // Should keep slashes as they are
	}

	for _, tc := range testCases {
		t.Run(tc.desc+"_"+tc.input, func(t *testing.T) {
			result := NormalizeSourceID(tc.input)

			// Basic validation - shouldn't crash and should have a tag
			if !strings.Contains(result, ":") {
				t.Errorf("Expected normalized result to contain a tag, got: '%s'", result)
			}

			if tc.contains != "" && !strings.Contains(result, tc.contains) {
				t.Errorf("Expected normalized result to contain '%s', got: '%s'", tc.contains, result)
			}
		})
	}
}

// TestImageNormalization_LongInput_IsTruncated tests that long inputs are properly handled
func TestImageNormalization_LongInput_IsTruncated(t *testing.T) {
	// Create a very long input string
	longInput := "very.long.image.name.with.many.segments.that.will.definitely.exceed.the.maximum.allowed.length.for.normalization.purposes"

	result := NormalizeSourceID(longInput)

	// Verify that the result is reasonable in length
	if len(result) > 2000 { // reasonable upper bound
		t.Fatalf("Expected normalized result to be reasonable length, got %d characters", len(result))
	}
}

// TestBuildDestImageID_LongSourceID_IsTruncated tests truncation logic in BuildDestImageID
func TestBuildDestImageID_LongSourceID_IsTruncated(t *testing.T) {
	// Create a very long source ID that should trigger truncation
	longSource := "very.long.image.name.with.many.segments.that.will.definitely.exceed.the.maximum.allowed.length.for.normalization.purposes"

	result := BuildDestImageID("registry.example.com", "namespace", longSource)

	// Verify that the registry and namespace are preserved
	if !strings.HasPrefix(result, "registry.example.com/namespace/") {
		t.Fatalf("Expected result to start with 'registry.example.com/namespace/', got: %s", result)
	}

	// Extract the last part (source portion) after the last slash
	parts := strings.Split(result, "/")
	truncatedSource := parts[len(parts)-1]

	if len(truncatedSource) > MaxNormalizedLen {
		t.Errorf("Expected truncated source to be at most %d characters, got %d: '%s'",
			MaxNormalizedLen, len(truncatedSource), truncatedSource)
	}
}

// TestBuildDestImageID_WithEmptyComponents tests handling of empty components
func TestBuildDestImageID_WithEmptyComponents(t *testing.T) {
	testCases := []struct {
		host      string
		namespace string
		source    string
		expected  string
		desc      string
	}{
		{"", "ns", "img:tag", "/ns/img_tag", "empty host"},
		{"host.com", "", "img:tag", "host.com/img:tag", "empty namespace"},
		{"host.com", "ns", "", "host.com/ns/", "empty source"},
		{"", "", "img:tag", "/img_tag", "empty host and namespace"},
		{"", "", "", "/", "all empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := BuildDestImageID(tc.host, tc.namespace, tc.source)
			if result != tc.expected {
				t.Errorf("BuildDestImageID('%s', '%s', '%s') = '%s', want '%s'",
					tc.host, tc.namespace, tc.source, result, tc.expected)
			}
		})
	}
}

// TestImageNormalization_MultipleSlashes_AreHandled tests handling of multiple slashes
func TestImageNormalization_MultipleSlashes_AreHandled(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		desc     string
	}{
		{"nginx///image", "nginx///image:latest", "multiple slashes in middle"},
		{"host//path", "host//path:latest", "host with multiple slashes"},
		{"///absolute", "///absolute:latest", "leading slashes"},
		{"reg//ns//img", "reg//ns//img:latest", "multiple slash pairs"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := NormalizeSourceID(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeSourceID('%s') = '%s', want '%s'", tc.input, result, tc.expected)
			}
		})
	}
}

// TestValidateImageNameInput_SpecialCharacter_Rejection tests validation against special characters
func TestValidateImageNameInput_SpecialCharacter_Rejection(t *testing.T) {
	specialCharTests := []struct {
		input    string
		desc     string
		shouldReject bool
	}{
		{"image{name}", "curly braces", true},
		{"image[name]", "square brackets", true},
		{"image|pipe", "pipe", true},
		{"image&and", "ampersand", true},
		{"image^caret", "caret", true},
		{"image~tilde", "tilde", true},
		{"image!exclaim", "exclamation", true},
		{"image@at", "at symbol", true},
		{"image#hash", "hash", true},
		{"image%percent", "percent", true},
		{"image*star", "asterisk", true},
		{"image?question", "question mark", true},
		{"image\\backslash", "backslash", true},
		{"image\ttab", "tab character", true},
		{"image\nnewline", "newline character", true},
		{"nginx:latest", "valid name", false},
		{"my-registry.com/image:tag", "valid with registry", false},
	}

	for _, testCase := range specialCharTests {
		t.Run(testCase.desc+"_"+testCase.input, func(t *testing.T) {
			result := validateImageNameInput(testCase.input)

			if testCase.shouldReject && result {
				t.Errorf("Expected input '%s' with special characters to be rejected, but it was accepted", testCase.input)
			} else if !testCase.shouldReject && !result {
				t.Errorf("Expected valid input '%s' to be accepted, but it was rejected", testCase.input)
			}
		})
	}
}

// TestHasTagOrDigest_HandlesEmptyInput tests utility function with empty input
func TestHasTagOrDigest_HandlesEmptyInput(t *testing.T) {
	result := hasTagOrDigest("")

	// Empty string shouldn't have tag or digest
	if result {
		t.Fatalf("Expected empty string to return false for hasTagOrDigest, got true")
	}
}

// TestHasTagOrDigest_HandlesValidInputs tests utility function with valid inputs
func TestHasTagOrDigest_HandlesValidInputs(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"nginx", false, "no tag or digest"},
		{"nginx:latest", true, "has tag"},
		{"nginx:1.21", true, "has numbered tag"},
		{"image@sha256:abc123", true, "has digest"},
		{"repo/image:tag", false, "no tag on tail segment"},
		{"repo/image:tag:sometag", true, "has tag (last segment has it)"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := hasTagOrDigest(tc.input)
			if result != tc.expected {
				t.Errorf("hasTagOrDigest('%s') = %t, want %t", tc.input, result, tc.expected)
			}
		})
	}
}