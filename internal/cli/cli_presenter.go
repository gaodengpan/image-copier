package cli

import (
	"encoding/json"
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

type JSONPresenter struct{}

func NewJSONPresenter() *JSONPresenter {
	return &JSONPresenter{}
}

func (p *JSONPresenter) PresentCheckingImageCount(count int) {
}

func (p *JSONPresenter) PresentDiffSummary(syncedCount, toSyncCount int) {
}

func (p *JSONPresenter) PresentDryRunResults(synced, toSync []syncTask) {
}

func (p *JSONPresenter) PresentProgress(current, total int) {
}

func (p *JSONPresenter) PresentSummary(s *PullSummary, results []ImageResult) {
	type imageResultJSON struct {
		Image     string `json:"image"`
		Arch      string `json:"arch"`
		Os        string `json:"os"`
		Success   bool   `json:"success,omitempty"`
		Skipped   bool   `json:"skipped,omitempty"`
		DryRun    bool   `json:"dry_run,omitempty"`
		Failed    bool   `json:"failed,omitempty"`
		Cancelled bool   `json:"cancelled,omitempty"`
		Error     string `json:"error,omitempty"`
	}

	imgResults := make([]imageResultJSON, len(results))
	for i, r := range results {
		imgResults[i] = imageResultJSON{
			Image:     r.Image,
			Arch:      r.Arch,
			Os:        r.Os,
			Success:   r.Success,
			Skipped:   r.Skipped,
			DryRun:    r.DryRun,
			Failed:    r.Failed,
			Cancelled: r.Cancelled,
			Error:     r.Error,
		}
	}

	output := map[string]interface{}{
		"summary": s,
		"images":  imgResults,
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}

func (p *JSONPresenter) PresentError(err error) {
	errJSON := map[string]string{"error": err.Error()}
	jsonBytes, _ := json.MarshalIndent(errJSON, "", "  ")
	fmt.Println(string(jsonBytes))
}
