package config

import "time"

// BaseNexusSchedulerConfig defines the configuration for the base nexus scheduler.
// This allows to fine tune all scheduler, which are composed of the base scheduler
type BaseNexusSchedulerConfig struct {
	RunFlag       bool
	SleepDuration time.Duration
}

// NewBaseNexusSchedulerConfig allows to create a base scheduler config.
func NewBaseNexusSchedulerConfig(runFlag bool, sleepDuration time.Duration) BaseNexusSchedulerConfig {
	return BaseNexusSchedulerConfig{
		RunFlag:       runFlag,
		SleepDuration: sleepDuration,
	}
}

// NewDefaultBaseNexusSchedulerConfig allows to create a base scheduler config with default values.
// RunFlag is set to false.
// SleepDuration is set to 1 second.
func NewDefaultBaseNexusSchedulerConfig() BaseNexusSchedulerConfig {
	return NewBaseNexusSchedulerConfig(false, 1*time.Second)
}
