package cli

import (
	"fmt"
)

type CLIPresenter struct{}

func NewCLIPresenter() *CLIPresenter {
	return &CLIPresenter{}
}

func (p *CLIPresenter) PresentCheckingImageCount(count int) {
	fmt.Printf("Checking %d image(s) against destination registry...\n\n", count)
}

func (p *CLIPresenter) PresentDiffSummary(syncedCount, toSyncCount int) {
	fmt.Printf("  ✓ %d already synced\n  → %d to sync\n\n", syncedCount, toSyncCount)
}

func (p *CLIPresenter) PresentDryRunResults(synced, toSync []syncTask) {
	for _, t := range synced {
		fmt.Printf("  ✓ %s (synced)\n", t.displayName())
	}
	for _, t := range toSync {
		fmt.Printf("  → %s (will sync)\n", t.displayName())
	}
}

func (p *CLIPresenter) PresentProgress(current, total int) {
	fmt.Printf("[%d/%d]\n", current, total)
}

func (p *CLIPresenter) PresentSummary(s *PullSummary, results []ImageResult) {
	fmt.Println()
	fmt.Println(FormatPullSummary(s))
	fmt.Print(FormatImageResults(results))
}

func (p *CLIPresenter) PresentError(err error) {
	fmt.Printf("Error: %v\n", err)
}
