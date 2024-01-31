package models

// BulkSchedulerConfig defines the configuration for the bulk scheduler. This allows to fine tune the scheduler.
type BulkSchedulerConfig struct {
	// The minimum amount of items that must be in the bulk queue before the bulk scheduler will run.
	MinAmountOfBulkItems int
}

// NewBulkSchedulerConfig allows to create a bulk config.
func NewBulkSchedulerConfig(minAmountOfBulkItems int) *BulkSchedulerConfig {
	return &BulkSchedulerConfig{
		MinAmountOfBulkItems: minAmountOfBulkItems,
	}
}

// NewDefaultBulkSchedulerConfig allows to create a bulk config with default values.
// MinAmountOfBulkItems is set to 10.
func NewDefaultBulkSchedulerConfig() *BulkSchedulerConfig {
	return NewBulkSchedulerConfig(10)
}
