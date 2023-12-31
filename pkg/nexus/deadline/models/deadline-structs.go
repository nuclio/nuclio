package models

import (
	"time"
)

type DeadlineSchedulerConfig struct {
	// The amount of time to wait before removing a deadline from the queue.
	DeadlineRemovalThreshold time.Duration
}

func NewDeadlineSchedulerConfig(deadlineRemovalThreshold time.Duration) *DeadlineSchedulerConfig {
	return &DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}
}

func NewDefaultDeadlineSchedulerConfig() *DeadlineSchedulerConfig {
	return NewDeadlineSchedulerConfig(10 * time.Second)
}
