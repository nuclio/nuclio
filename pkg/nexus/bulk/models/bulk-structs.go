package models

// BulkSchedulerConfig is defined the configuration for the bulk scheduler.
type BulkSchedulerConfig struct {
	MinAmountOfBulkItems                         int     // The minimum amount of items that must be in the bulk queue before the bulk scheduler will run.
	MaxPercentageUsageCPU, MaxPercentageUsageRAM float64 // The maximum percentage of CPU and RAM that can be used before the bulk scheduler will run.
}

// NewBulkSchedulerConfig allows to create a bulk config.
func NewBulkSchedulerConfig(minAmountOfBulkItems int, maxPercentageUsageCPU, maxPercentageUsageRAM float64) *BulkSchedulerConfig {
	return &BulkSchedulerConfig{
		MinAmountOfBulkItems:  minAmountOfBulkItems,
		MaxPercentageUsageCPU: maxPercentageUsageCPU,
		MaxPercentageUsageRAM: maxPercentageUsageRAM,
	}
}

// NewDefaultBulkSchedulerConfig allows to create a bulk config with default values.
func NewDefaultBulkSchedulerConfig() *BulkSchedulerConfig {
	return NewBulkSchedulerConfig(10, 80, 80)
}
