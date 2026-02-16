package cli

import "time"

type PullPresenter interface {
	PresentCheckingImageCount(count int)
	PresentDiffSummary(syncedCount, toSyncCount int)
	PresentDryRunResults(synced, toSync []syncTask)
	PresentProgress(current, total int)
	PresentSummary(s *PullSummary, results []ImageResult)
	PresentError(err error)
}

type PullResult struct {
	Succeeded []syncTask
	Skipped   []syncTask
	Failed    []syncTask
	DryRun    []syncTask
	Duration  time.Duration
}
