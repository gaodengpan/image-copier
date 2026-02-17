package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatImageResults_Cancelled(t *testing.T) {
	results := []ImageResult{
		{
			Image:     "test:latest",
			Cancelled: true,
			Error:     "timeout exceeded",
		},
	}

	output := FormatImageResults(results)

	assert.True(t, strings.Contains(output, "⊘"), "Cancelled should show ⊘ symbol")
	assert.True(t, strings.Contains(output, "test:latest"), "Should contain image name")
	assert.True(t, strings.Contains(output, "timeout exceeded"), "Should contain error message")
	assert.False(t, strings.Contains(output, "Error:"), "Should not contain Error: prefix")
}
