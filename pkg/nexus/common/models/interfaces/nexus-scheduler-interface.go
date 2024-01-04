package interfaces

type SchedulerStatus string

const (
	Running SchedulerStatus = "Running"
	Stopped SchedulerStatus = "Stopped"
)

// *nexus.Nexus
type INexusScheduler interface {
	Start()
	Stop()
	GetStatus() SchedulerStatus
}
