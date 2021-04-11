package leader

import (
	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Sends to the leader a request to create a project
	Create(*platform.CreateProjectOptions) error

	// Sends to the leader a request to update a project
	Update(*platform.UpdateProjectOptions) error

	// Sends to the leader a request to delete a project
	Delete(*platform.DeleteProjectOptions) error
}
