package configs

import "time"

type BaseNexusSchedulerConfig struct {
	RunFlag       bool
	SleepDuration time.Duration
}

func NewBaseNexusSchedulerConfig(runFlag bool, sleepDuration time.Duration) BaseNexusSchedulerConfig {
	return BaseNexusSchedulerConfig{
		RunFlag:       runFlag,
		SleepDuration: sleepDuration,
	}
}

func NewDefaultBaseNexusSchedulerConfig() BaseNexusSchedulerConfig {
	return NewBaseNexusSchedulerConfig(false, 1*time.Second)
}
