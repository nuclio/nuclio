package project

import (
	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Initialize client
	Initialize() error

	// Create a new project
	Create(*platform.CreateProjectOptions) (platform.Project, error)

	// Update a project
	Update(*platform.UpdateProjectOptions) (platform.Project, error)

	// Delete a project
	Delete(*platform.DeleteProjectOptions) error

	// Get projects
	Get(*platform.GetProjectsOptions) ([]platform.Project, error)
}
