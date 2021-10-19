package local

import (
	"github.com/nuclio/nuclio/pkg/platform"
	abstractproject "github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/local/client"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	Logger     logger.Logger
	platform   platform.Platform
	localStore *client.Store
}

func NewClient(parentLogger logger.Logger, platform platform.Platform, localStore *client.Store) (abstractproject.Client, error) {
	newClient := Client{
		Logger:     parentLogger.GetChild("projects-local"),
		localStore: localStore,
		platform:   platform,
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {
	return nil
}

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return c.localStore.GetProjects(&getProjectsOptions.Meta)
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	c.Logger.DebugWith("Creating a project", "projectName", createProjectOptions.ProjectConfig.Meta.Name)
	return nil, c.localStore.CreateOrUpdateProject(createProjectOptions.ProjectConfig)
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	c.Logger.DebugWith("Updating a project", "projectName", updateProjectOptions.ProjectConfig.Meta.Name)
	return nil, c.localStore.CreateOrUpdateProject(&updateProjectOptions.ProjectConfig)
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	c.Logger.DebugWith("Deleting a project", "projectMeta", deleteProjectOptions.Meta)
	if err := c.localStore.DeleteProject(&deleteProjectOptions.Meta); err != nil {
		return errors.Wrapf(err,
			"Failed to delete project %s from namespace %s",
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace)
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return c.platform.WaitForProjectResourcesDeletion(&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}
