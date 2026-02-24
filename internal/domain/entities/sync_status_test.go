package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncStatus(t *testing.T) {
	assert.Equal(t, SyncStatus("pending"), SyncStatusPending)
	assert.Equal(t, SyncStatus("syncing"), SyncStatusSyncing)
	assert.Equal(t, SyncStatus("completed"), SyncStatusCompleted)
	assert.Equal(t, SyncStatus("failed"), SyncStatusFailed)
}
