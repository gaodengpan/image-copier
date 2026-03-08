package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/shared/sanitizer"
)

// SyncCLIPresenter implements SyncPresenter for CLI output
type SyncCLIPresenter struct{}

// NewSyncCLIPresenter creates a new CLI presenter
func NewSyncCLIPresenter() *SyncCLIPresenter {
	return &SyncCLIPresenter{}
}

// PresentSyncStart presents the start of sync operation
func (p *SyncCLIPresenter) PresentSyncStart(count int) {
	fmt.Printf("Starting sync for %d image(s)...\n\n", count)
}

// PresentSyncPhaseResult presents the sync phase result
func (p *SyncCLIPresenter) PresentSyncPhaseResult(result *input.SyncPhaseResult) {
	fmt.Println("=== Phase 1: Sync to Staging Registry ===")
	fmt.Printf("Already existed: %d\n", len(result.AlreadyExisted))
	fmt.Printf("Newly synced:    %d\n", len(result.NewlySynced))
	if len(result.Failed) > 0 {
		fmt.Printf("Failed:          %d\n", len(result.Failed))
		for _, task := range result.Failed {
			fmt.Printf("  - %s: FAILED\n", task.DisplayName())
		}
	}
	fmt.Println()
}

// PresentDistributePhaseResult presents the distribute phase result
func (p *SyncCLIPresenter) PresentDistributePhaseResult(result *input.DistributePhaseResult) {
	fmt.Println("=== Phase 2: Distribute to Targets ===")
	fmt.Printf("Success: %d\n", result.SuccessCount)
	fmt.Printf("Skipped: %d\n", result.SkippedCount)
	if result.FailedCount > 0 {
		fmt.Printf("Failed:  %d\n", result.FailedCount)
		for _, err := range result.Errors {
			fmt.Printf("  - %s -> %s: %v\n", err.ImageName, err.TargetName, err.Error)
		}
	}
	fmt.Println()
}

// PresentSummary presents the final summary
func (p *SyncCLIPresenter) PresentSummary(summary *SyncSummary) {
	fmt.Println("=== Summary ===")
	fmt.Printf("Total images: %d\n", summary.TotalImages)
	fmt.Printf("Sync - Success: %d, Failed: %d\n", summary.SyncSuccess, summary.SyncFailed)
	fmt.Printf("Dist - Success: %d, Skipped: %d, Failed: %d\n",
		summary.DistSuccess, summary.DistSkipped, summary.DistFailed)
	fmt.Printf("Duration: %s\n", summary.Duration.Round(time.Second))
}

// PresentError presents an error
func (p *SyncCLIPresenter) PresentError(err error) {
	fmt.Printf("Error: %v\n", err)
}

// SyncJSONPresenter implements SyncPresenter for JSON output
type SyncJSONPresenter struct {
	syncResult      *input.SyncPhaseResult
	distributeResult *input.DistributePhaseResult
}

// NewSyncJSONPresenter creates a new JSON presenter
func NewSyncJSONPresenter() *SyncJSONPresenter {
	return &SyncJSONPresenter{}
}

// jsonSyncResult represents the JSON output structure
type jsonSyncResult struct {
	Images   []jsonImageResult `json:"images"`
	Summary  jsonSummary       `json:"summary"`
	Success  bool              `json:"success"`
	Duration string            `json:"duration"`
}

type jsonImageResult struct {
	Source       string                  `json:"source"`
	StagingID    string                  `json:"staging_id,omitempty"`
	SyncStatus   string                  `json:"sync_status"`
	SyncError    *jsonError              `json:"sync_error,omitempty"`
	Distribution []jsonDistributionResult `json:"distribution,omitempty"`
}

type jsonDistributionResult struct {
	Target string    `json:"target"`
	Status string    `json:"status"`
	Error  *jsonError `json:"error,omitempty"`
}

type jsonError struct {
	Message string `json:"message"`
}

type jsonSummary struct {
	TotalImages      int    `json:"total_images"`
	SyncSuccess      int    `json:"sync_success"`
	SyncFailed       int    `json:"sync_failed"`
	DistributeSuccess int   `json:"distribute_success"`
	DistributeSkipped int   `json:"distribute_skipped"`
	DistributeFailed  int   `json:"distribute_failed"`
}

// PresentSyncStart presents the start of sync operation (no-op for JSON)
func (p *SyncJSONPresenter) PresentSyncStart(count int) {
	// No output during start for JSON mode
}

// PresentSyncPhaseResult stores the sync phase result for later output
func (p *SyncJSONPresenter) PresentSyncPhaseResult(result *input.SyncPhaseResult) {
	p.syncResult = result
}

// PresentDistributePhaseResult stores the distribute phase result for later output
func (p *SyncJSONPresenter) PresentDistributePhaseResult(result *input.DistributePhaseResult) {
	p.distributeResult = result
}

// PresentSummary presents the final JSON output
func (p *SyncJSONPresenter) PresentSummary(summary *SyncSummary) {
	images := p.buildImageResults()

	result := jsonSyncResult{
		Images: images,
		Summary: jsonSummary{
			TotalImages:       summary.TotalImages,
			SyncSuccess:       summary.SyncSuccess,
			SyncFailed:        summary.SyncFailed,
			DistributeSuccess: summary.DistSuccess,
			DistributeSkipped: summary.DistSkipped,
			DistributeFailed:  summary.DistFailed,
		},
		Success:  summary.SyncFailed == 0 && summary.DistFailed == 0,
		Duration: summary.Duration.String(),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("{\"error\": \"failed to marshal JSON: %v\"}\n", err)
		return
	}
	fmt.Println(string(data))
}

// buildImageResults builds the image results from stored phase results
func (p *SyncJSONPresenter) buildImageResults() []jsonImageResult {
	images := make([]jsonImageResult, 0)

	// Build map of distribute tasks by original source
	distributeMap := make(map[string][]jsonDistributionResult)
	if p.distributeResult != nil {
		for _, task := range p.distributeResult.Tasks {
			dists := make([]jsonDistributionResult, 0, len(task.Results))
			for _, r := range task.Results {
				distResult := jsonDistributionResult{
					Target: r.TargetName,
					Status: "success",
				}
				if r.Skipped {
					distResult.Status = "skipped"
				}
				if r.Error != nil {
					distResult.Status = "failed"
					distResult.Error = &jsonError{Message: r.Error.Error()}
				}
				dists = append(dists, distResult)
			}
			distributeMap[task.OriginalSource] = dists
		}
	}

	// Process synced images (already existed)
	if p.syncResult != nil {
		for _, task := range p.syncResult.AlreadyExisted {
			images = append(images, jsonImageResult{
				Source:       task.Source,
				SyncStatus:   "synced",
				Distribution: distributeMap[task.Source],
			})
		}

		// Process newly synced images
		for _, task := range p.syncResult.NewlySynced {
			images = append(images, jsonImageResult{
				Source:       task.Source,
				SyncStatus:   "synced",
				Distribution: distributeMap[task.Source],
			})
		}

		// Process failed images
		for _, task := range p.syncResult.Failed {
			imgResult := jsonImageResult{
				Source:     task.Source,
				SyncStatus: "failed",
			}
			if task.Error != nil {
				imgResult.SyncError = &jsonError{Message: task.Error.Error()}
			}
			images = append(images, imgResult)
		}
	}

	return images
}

// PresentError presents an error as JSON
func (p *SyncJSONPresenter) PresentError(err error) {
	errResult := struct {
		Error string `json:"error"`
	}{
		Error: sanitizer.SanitizeError(err.Error(), 500),
	}
	data, marshalErr := json.MarshalIndent(errResult, "", "  ")
	if marshalErr != nil {
		fmt.Printf("{\"error\": \"failed to marshal error JSON: %v\"}\n", marshalErr)
		return
	}
	fmt.Println(string(data))
}
