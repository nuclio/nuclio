package leader

import (
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Get delegate the project get to leader
	Get(*platform.GetProjectsOptions) ([]platform.Project, error)

	// Create delegates project creation to leader
	Create(*platform.CreateProjectOptions) error

	// Update delegates project update to leader
	Update(*platform.UpdateProjectOptions) error

	// Delete delegates project deletion to leader
	Delete(*platform.DeleteProjectOptions) error

	// GetUpdatedAfter gets all projects from the leader that updated after the given time (to get all, pass nil time)
	GetUpdatedAfter(*time.Time) ([]platform.Project, error)
}
