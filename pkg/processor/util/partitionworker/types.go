package partitionworker

type AllocationMode string

const (
	AllocationModePool   AllocationMode = "pool"
	AllocationModeStatic AllocationMode = "static"
)
