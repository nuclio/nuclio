/*
Copyright 2017 The Nuclio Authors.

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

package platform

import (
	"context"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const (
	ProjectGetUponCreationTimeout       = 30 * time.Second
	ProjectGetUponCreationRetryInterval = 1 * time.Second
)

type Project interface {

	// GetConfig returns the project config
	GetConfig() *ProjectConfig
}

type AbstractProject struct {
	Logger        logger.Logger
	Platform      Platform
	ProjectConfig ProjectConfig
}

func NewAbstractProject(parentLogger logger.Logger,
	parentPlatform Platform,
	projectConfig ProjectConfig) (*AbstractProject, error) {

	return &AbstractProject{
		Logger:        parentLogger.GetChild("project"),
		Platform:      parentPlatform,
		ProjectConfig: projectConfig,
	}, nil
}

// GetConfig returns the project config
func (ap *AbstractProject) GetConfig() *ProjectConfig {
	return &ap.ProjectConfig
}

func (ap *AbstractProject) CreateAndWait(ctx context.Context, createProjectOptions *CreateProjectOptions) error {
	if err := ap.Platform.CreateProject(ctx, createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	if err := common.RetryUntilSuccessful(ProjectGetUponCreationTimeout, ProjectGetUponCreationRetryInterval, func() bool {
		ap.Logger.DebugWith("Trying to get created project",
			"projectMeta", ap.GetConfig().Meta,
			"timeout", ProjectGetUponCreationTimeout,
			"retryInterval", ProjectGetUponCreationRetryInterval)
		projects, err := ap.Platform.GetProjects(ctx, &GetProjectsOptions{
			Meta: ap.GetConfig().Meta,
		})
		return err == nil && len(projects) > 0
	}); err != nil {
		return nuclio.WrapErrInternalServerError(errors.Wrapf(err,
			"Failed to wait for a created project %s",
			ap.GetConfig().Meta.Name))
	}

	return nil
}
