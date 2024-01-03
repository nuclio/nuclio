package models

type BulkSchedulerConfig struct {
	MinAmountOfBulkItems                         int     // The minimum amount of items that must be in the bulk queue before the bulk scheduler will run.
	MaxPercentageUsageCPU, MaxPercentageUsageRAM float64 // The maximum percentage of CPU and RAM that can be used before the bulk scheduler will run.
}

func NewBulkSchedulerConfig(minAmountOfBulkItems int, maxPercentageUsageCPU, maxPercentageUsageRAM float64) *BulkSchedulerConfig {
	return &BulkSchedulerConfig{
		MinAmountOfBulkItems:  minAmountOfBulkItems,
		MaxPercentageUsageCPU: maxPercentageUsageCPU,
		MaxPercentageUsageRAM: maxPercentageUsageRAM,
	}
}

func NewDefaultBulkSchedulerConfig() *BulkSchedulerConfig {
	return NewBulkSchedulerConfig(10, 80, 80)
}
