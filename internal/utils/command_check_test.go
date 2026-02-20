package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCommandExists(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "bash exists",
			command: "bash",
			want:    true,
		},
		{
			name:    "ls exists",
			command: "ls",
			want:    true,
		},
		{
			name:    "nonexistent command",
			command: "thiscommanddoesnotexist12345",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckCommandExists(tt.command)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckCommandsExist(t *testing.T) {
	commands := []string{"bash", "ls", "nonexistent12345"}
	results := CheckCommandsExist(commands)

	assert.Len(t, results, 3)
	assert.True(t, results["bash"])
	assert.True(t, results["ls"])
	assert.False(t, results["nonexistent12345"])
}

func TestGetMissingCommands(t *testing.T) {
	tests := []struct {
		name      string
		commands  []string
		wantLen   int
		wantEmpty bool
	}{
		{
			name:      "all exist",
			commands:  []string{"bash", "ls"},
			wantLen:   0,
			wantEmpty: true,
		},
		{
			name:      "none exist",
			commands:  []string{"nonexist1", "nonexist2"},
			wantLen:   2,
			wantEmpty: false,
		},
		{
			name:      "mixed",
			commands:  []string{"bash", "nonexist"},
			wantLen:   1,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := GetMissingCommands(tt.commands)
			assert.NoError(t, err)
			assert.Len(t, missing, tt.wantLen)
		})
	}
}
