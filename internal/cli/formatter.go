package cli

import (
	"fmt"
	"time"
)

type PullSummary struct {
	Succeeded int
	Skipped   int
	DryRun    int
	Failed    int
	Duration  time.Duration
}

func FormatPullSummary(s *PullSummary) string {
	if s.Duration > 0 {
		if s.DryRun > 0 {
			return fmt.Sprintf("Summary: %d succeeded, %d skipped, %d dry-run, %d failed | Total: %s",
				s.Succeeded, s.Skipped, s.DryRun, s.Failed, formatDuration(s.Duration))
		}
		return fmt.Sprintf("Summary: %d succeeded, %d skipped, %d failed | Total: %s",
			s.Succeeded, s.Skipped, s.Failed, formatDuration(s.Duration))
	}

	if s.DryRun > 0 {
		return fmt.Sprintf("Summary: %d succeeded, %d skipped, %d dry-run, %d failed",
			s.Succeeded, s.Skipped, s.DryRun, s.Failed)
	}
	return fmt.Sprintf("Summary: %d succeeded, %d skipped, %d failed",
		s.Succeeded, s.Skipped, s.Failed)
}

type ImageResult struct {
	Image   string
	Arch    string
	Os      string
	Success bool
	Skipped bool
	DryRun  bool
	Failed  bool
	Error   string
}

func FormatImageResults(results []ImageResult) string {
	var output string
	for _, r := range results {
		if r.Skipped {
			output += fmt.Sprintf("  ◦ %s\n", r.Image)
		} else if r.DryRun {
			output += fmt.Sprintf("  ~ %s\n", r.Image)
		} else if r.Failed {
			output += fmt.Sprintf("  ✗ %s: %s\n", r.Image, r.Error)
		} else if r.Success {
			output += fmt.Sprintf("  ✓ %s\n", r.Image)
		}
	}
	return output
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}
