package configs

import "time"

type BaseNexusSchedulerConfig struct {
	RunFlag       bool
	SleepDuration time.Duration
}

func CreateBaseNexusSchedulerConfig(runFlag bool, sleepDuration time.Duration) BaseNexusSchedulerConfig {
	return BaseNexusSchedulerConfig{
		RunFlag:       runFlag,
		SleepDuration: sleepDuration,
	}
}

func CreateDefaultBaseNexusSchedulerConfig() BaseNexusSchedulerConfig {
	return CreateBaseNexusSchedulerConfig(true, 1*time.Second)
}
