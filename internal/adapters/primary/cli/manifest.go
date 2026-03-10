package cli

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

type SyncManifest struct {
	Images []SyncImage `yaml:"images"`
}

type SyncImage struct {
	Source    string   `yaml:"source"`
	Platforms []string `yaml:"platforms"`
}

type syncTask struct {
	Source string
	Arch   string
	Os     string
}

func readSyncManifest(path, defaultArch, defaultOs string) ([]syncTask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var manifest SyncManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var tasks []syncTask
	for _, img := range manifest.Images {
		if img.Source == "" {
			continue
		}
		platforms := img.Platforms
		if len(platforms) == 0 {
			platforms = []string{defaultOs + "/" + defaultArch}
		}
		for _, plat := range platforms {
			parts := strings.SplitN(plat, "/", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid platform format %q (expected os/arch)", plat)
			}
			tasks = append(tasks, syncTask{
				Source: img.Source,
				Arch:   parts[1],
				Os:     parts[0],
			})
		}
	}
	return tasks, nil
}

func calculateAdaptiveWorkerCount(userSpecified bool, userValue, taskCount, cpuCount int) int {
	if userSpecified {
		return userValue
	}

	maxWorkers := cpuCount * 4
	if taskCount < maxWorkers {
		maxWorkers = taskCount
	}
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	return maxWorkers
}
