package cli

import (
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/internal/adapters"
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSetupLogLevel(t *testing.T) {
	logger := logrus.New()

	// Test verbose mode
	setupLogLevel(logger, true)
	assert.Equal(t, logrus.DebugLevel, logger.GetLevel())

	// Test non-verbose mode
	setupLogLevel(logger, false)
	assert.Equal(t, logrus.WarnLevel, logger.GetLevel())
}

func TestCreatePresenter(t *testing.T) {
	t.Run("JSONPresenter", func(t *testing.T) {
		presenter := createPresenter("json")
		assert.NotNil(t, presenter)
		_, ok := presenter.(*SyncJSONPresenter)
		assert.True(t, ok)
	})

	t.Run("CLIPresenter", func(t *testing.T) {
		presenter := createPresenter("text")
		assert.NotNil(t, presenter)
		_, ok := presenter.(*SyncCLIPresenter)
		assert.True(t, ok)
	})

	t.Run("DefaultPresenter", func(t *testing.T) {
		presenter := createPresenter("unknown")
		assert.NotNil(t, presenter)
		_, ok := presenter.(*SyncCLIPresenter)
		assert.True(t, ok)
	})
}

func TestCreateProgress(t *testing.T) {
	presenter := NewSyncCLIPresenter()
	prog := createProgress(presenter, 10, 3)

	assert.NotNil(t, prog)
}

func TestCreateProgressCallbacks(t *testing.T) {
	presenter := NewSyncCLIPresenter()
	prog := presenter.PresentProgress(3, 3)
	prog.AddImage(0, "nginx")
	prog.AddImage(1, "redis")
	prog.AddImage(2, "mysql")

	images := []string{"nginx", "redis", "mysql"}
	callbacks := createProgressCallbacks(prog, images, 3)

	assert.NotNil(t, callbacks.ProgressCallback)
	assert.NotNil(t, callbacks.TaskComplete)

	// Test ProgressCallback
	callbacks.ProgressCallback("nginx", value_objects.SyncStageSyncing, "", 50.0)

	// Test TaskComplete
	callbacks.TaskComplete("nginx", false, nil)
}

func TestBuildSyncUseCase(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	cfg.Registry.Host = "staging.example.com"
	cfg.Registry.Namespace = "ns"
	cfg.Registry.Username = "user"
	cfg.Registry.Password = "pass"
	cfg.Github.Owner = "owner"
	cfg.Github.Repo = "repo"
	cfg.Github.Token = "token"
	cfg.Github.WorkflowID = "workflow.yml"
	factory := adapters.NewAdapterFactory(logger)

	useCase := buildSyncUseCase(logger, cfg, factory)

	assert.NotNil(t, useCase)
}

func TestBuildSyncSummary_NilPhases(t *testing.T) {
	result := &input.SyncCommandResult{
		Duration: 1 * time.Second,
	}
	images := []string{"img1"}

	summary := buildSyncSummary(result, images)

	assert.Equal(t, 1, summary.TotalImages)
	assert.Equal(t, 0, summary.SyncSuccess)
	assert.Equal(t, 0, summary.DistSuccess)
}
