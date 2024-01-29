package interfaces

type SchedulerStatus string

// The status of a scheduler.
const (
	Running SchedulerStatus = "Running"
	Stopped SchedulerStatus = "Stopped"
)

// INexusScheduler is an interface for a nexus scheduler.
type INexusScheduler interface {
	// Start starts the scheduler.
	Start()
	// Stop stops the scheduler.
	Stop()
	// GetStatus returns a status update of the scheduler.
	GetStatus() SchedulerStatus
}
