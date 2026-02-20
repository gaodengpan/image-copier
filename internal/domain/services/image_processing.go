package services

type ImageProcessingStrategy int

const (
	StrategyLocalOnly ImageProcessingStrategy = iota
	StrategyRegistryOnly
	StrategyFullSync
)

type ImageProcessingResult struct {
	LocalExists    bool
	RegistryExists bool
	ShouldSync     bool
	ShouldTrigger  bool
	Strategy       ImageProcessingStrategy
	SkipReason     string
}

func DetermineProcessingResult(localExists, registryExists, force, dryRun bool) ImageProcessingResult {
	result := ImageProcessingResult{
		LocalExists:    localExists,
		RegistryExists: registryExists,
	}

	if localExists && !force {
		result.ShouldSync = false
		result.SkipReason = "image already exists locally (use --force to override)"
		result.Strategy = StrategyLocalOnly
		return result
	}

	if dryRun {
		result.ShouldSync = false
		result.Strategy = StrategyLocalOnly
		return result
	}

	if !registryExists {
		result.ShouldSync = true
		result.ShouldTrigger = true
		result.Strategy = StrategyFullSync
		return result
	}

	result.ShouldSync = true
	result.ShouldTrigger = false
	result.Strategy = StrategyRegistryOnly

	return result
}

func ShouldTriggerWorkflow(localExists, registryExists, force bool) bool {
	if force {
		return true
	}
	if localExists {
		return false
	}
	if !registryExists {
		return true
	}
	return false
}
