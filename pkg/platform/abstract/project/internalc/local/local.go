/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"

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

func (c *Client) Get(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return c.localStore.GetProjects(&getProjectsOptions.Meta)
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	c.Logger.DebugWithCtx(ctx,
		"Creating a project",
		"projectName", createProjectOptions.ProjectConfig.Meta.Name)
	return nil, c.localStore.CreateOrUpdateProject(createProjectOptions.ProjectConfig)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	c.Logger.DebugWithCtx(ctx,
		"Updating a project",
		"projectName", updateProjectOptions.ProjectConfig.Meta.Name)
	return nil, c.localStore.CreateOrUpdateProject(&updateProjectOptions.ProjectConfig)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	c.Logger.DebugWithCtx(ctx,
		"Deleting a project",
		"projectMeta", deleteProjectOptions.Meta)
	if err := c.localStore.DeleteProject(ctx, &deleteProjectOptions.Meta); err != nil {
		return errors.Wrapf(err,
			"Failed to delete project %s from namespace %s",
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace)
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return c.platform.WaitForProjectResourcesDeletion(ctx, &deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}
