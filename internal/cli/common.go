package cli

import (
	"time"

	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/use_cases"
	"github.com/gaodengpan/image-copier/pkg/logformat"
	"github.com/gaodengpan/image-copier/pkg/progress"
	"github.com/sirupsen/logrus"
)

// stageWeights defines the cumulative percentage weight for each pull stage.
var stageWeights = [6]float64{5, 15, 20, 80, 95, 100}

var stageNames = [6]string{
	"checking local",
	"checking registry",
	"triggering workflow",
	"workflow running",
	"downloading",
	"loading",
}

// asymptotic computes progress that slows as it approaches the ceiling.
// Formula: base + range * (1 - 1/(1 + k*polls))
func asymptotic(base, rangeSize float64, polls int) float64 {
	const k = 0.05
	return base + rangeSize*(1-1/(1+k*float64(polls)))
}

// CreateStageCallback creates a shared stage callback function for updating progress
func CreateStageCallback(progressMgr *progress.Progress, workerIdx int, label string, startTime time.Time) func(use_cases.PullStage, int) {
	return func(stage use_cases.PullStage, polls int) {
		var pct float64
		stageIdx := int(stage)

		if stage == use_cases.StageWaitWorkflow && polls > 0 {
			// For workflow waiting, compute asymptotic progress
			base := stageWeights[2]
			ceiling := stageWeights[3]
			pct = asymptotic(base, ceiling-base, polls)
		} else if stageIdx > 0 {
			// For other stages, use pre-defined weights
			pct = stageWeights[stageIdx-1]
		}

		progressMgr.UpdateStage(workerIdx, progress.StageInfo{
			Label:     label,
			StageName: stageNames[stageIdx],
			Percent:   pct,
			StartAt:   startTime,
		})
	}
}

// CreateCoreConfigFromConfig creates runtime config from loaded config
func CreateCoreConfigFromConfig(cfg *config.Config, force bool, dryRun bool) *config.Config {
	cfg.Force = force
	cfg.DryRun = dryRun
	return cfg
}

// SetupLogger sets up the logger based on config
func SetupLogger(cfg *config.Config, verbose bool) *logrus.Logger {
	// Convert log level string to logrus.Level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	if verbose {
		level = logrus.DebugLevel
	}

	logger := logrus.New()
	logger.SetLevel(level)

	if verbose {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		})
	} else {
		logger.SetFormatter(&logformat.CLIFormatter{})
	}

	return logger
}
