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

	// Gets all projects from the leader (gets projects that have been updated since the given timestamp)
	GetAll(*time.Time) ([]platform.Project, error)
}
