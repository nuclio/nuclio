package models

import "time"

// DeadlineSchedulerConfig defines the configuration for the deadline scheduler. This allows to fine tune the scheduler.
type DeadlineSchedulerConfig struct {
	// The amount of time to wait before removing a deadline from the queue.
	DeadlineRemovalThreshold time.Duration
}

// NewDeadlineSchedulerConfig allows to create a deadline config.
func NewDeadlineSchedulerConfig(deadlineRemovalThreshold time.Duration) *DeadlineSchedulerConfig {
	return &DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}
}

// NewDefaultDeadlineSchedulerConfig allows to create a deadline config with default values.
// DeadlineRemovalThreshold is set to 10 seconds.
func NewDefaultDeadlineSchedulerConfig() *DeadlineSchedulerConfig {
	return NewDeadlineSchedulerConfig(10 * time.Second)
}
