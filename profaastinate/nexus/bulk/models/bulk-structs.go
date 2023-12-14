package models

type BulkSchedulerConfig struct {
	MinAmountOfBulkItems int // The minimum amount of items that must be in the bulk queue before the bulk scheduler will run.
}
