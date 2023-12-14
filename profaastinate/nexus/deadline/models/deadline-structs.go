package models

import (
	"time"
)

type DeadlineSchedulerConfig struct {
	// The amount of time to wait before removing a deadline from the queue.
	DeadlineRemovalThreshold time.Duration
}
