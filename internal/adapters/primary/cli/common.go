package cli

import (
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/gaodengpan/image-copier/pkg/logformat"
	"github.com/sirupsen/logrus"
)

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
