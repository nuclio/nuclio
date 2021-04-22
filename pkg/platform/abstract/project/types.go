package project

import (
	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Initializes the projects client
	Initialize() error

	// Creates a new project
	Create(*platform.CreateProjectOptions) (platform.Project, error)

	// Updates a project
	Update(*platform.UpdateProjectOptions) (platform.Project, error)

	// Deletes a project
	Delete(*platform.DeleteProjectOptions) error

	// Gets projects
	Get(*platform.GetProjectsOptions) ([]platform.Project, error)
}
