package project

import (
	"context"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Initialize client
	Initialize() error

	// Create a new project
	Create(context.Context, *platform.CreateProjectOptions) (platform.Project, error)

	// Update a project
	Update(context.Context, *platform.UpdateProjectOptions) (platform.Project, error)

	// Delete a project
	Delete(context.Context, *platform.DeleteProjectOptions) error

	// Get projects
	Get(context.Context, *platform.GetProjectsOptions) ([]platform.Project, error)
}
