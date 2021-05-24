package leader

import (
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Delegates project creation to leader
	Create(*platform.CreateProjectOptions) error

	// Delegates project update to leader
	Update(*platform.UpdateProjectOptions) error

	// Delegates project deletion to leader
	Delete(*platform.DeleteProjectOptions) error

	// Gets all projects from the leader that updated after the given time (to get all, pass nil time)
	GetUpdatedAfter(*time.Time) ([]platform.Project, error)
}
