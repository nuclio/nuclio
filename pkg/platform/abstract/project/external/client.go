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

package external

import (
	"context"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mlrun"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	platformConfiguration *platformconfig.Config
	synchronizer          *iguazio.Synchronizer
	internalClient        project.Client
	leaderClient          leader.Client
}

func NewClient(parentLogger logger.Logger,
	internalClient project.Client,
	platformConfiguration *platformconfig.Config) (*Client, error) {
	var err error

	newClient := Client{}
	newClient.platformConfiguration = platformConfiguration

	// use the internal client (for now), so projects will be modified both on leader's side and internally by nuclio
	newClient.internalClient = internalClient

	newClient.leaderClient, err = newLeaderClient(parentLogger, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create leader client")
	}

	// get leader synchronization interval
	synchronizationIntervalStr := "0"
	if platformConfiguration.ProjectsLeader != nil {
		synchronizationIntervalStr = platformConfiguration.ProjectsLeader.SynchronizationInterval
	}

	namespaces := platformConfiguration.ManagedNamespaces
	if len(namespaces) == 0 {
		namespaces = append(namespaces, common.ResolveDefaultNamespace("@nuclio.selfNamespace"))
	}

	newClient.synchronizer, err = iguazio.NewSynchronizer(parentLogger,
		synchronizationIntervalStr,
		namespaces,
		newClient.leaderClient,
		internalClient)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create synchronizer")
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {
	if err := c.synchronizer.Start(); err != nil {
		return errors.Wrap(err, "Failed to start the projects synchronizer")
	}

	return c.internalClient.Initialize()
}

func (c *Client) Get(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return c.internalClient.Get(ctx, getProjectsOptions)
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	switch createProjectOptions.RequestOrigin {

	// if request came from leader, create it internally
	case c.platformConfiguration.ProjectsLeader.Kind:
		return c.internalClient.Create(ctx, createProjectOptions)

	// request came from user / non-leader client
	// ask leader to create
	default:
		if err := c.leaderClient.Create(ctx, createProjectOptions); err != nil {
			return nil, errors.Wrap(err, "Failed while requesting from the leader to create the project")
		}

		return nil, platform.ErrSuccessfulCreateProjectLeader
	}
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	switch updateProjectOptions.RequestOrigin {
	case c.platformConfiguration.ProjectsLeader.Kind:
		return c.internalClient.Update(ctx, updateProjectOptions)
	default:
		if err := c.leaderClient.Update(ctx, updateProjectOptions); err != nil {
			return nil, errors.Wrap(err, "Failed while requesting from the leader to update the project")
		}

		return nil, platform.ErrSuccessfulUpdateProjectLeader
	}
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	switch deleteProjectOptions.RequestOrigin {
	case c.platformConfiguration.ProjectsLeader.Kind:

		// request came from leader, delete it internally
		return c.internalClient.Delete(ctx, deleteProjectOptions)
	default:

		// request came from user / non-leader client. ask leader to delete
		if err := c.leaderClient.Delete(ctx, deleteProjectOptions); err != nil {
			return errors.Wrap(err, "Failed while requesting from the leader to delete the project")
		}

		return platform.ErrSuccessfulDeleteProjectLeader
	}
}

func newLeaderClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (leader.Client, error) {
	switch platformConfiguration.ProjectsLeader.Kind {

	// mlrun projects leader
	case platformconfig.ProjectsLeaderKindMlrun:
		return mlrun.NewClient(parentLogger, platformConfiguration)

	// iguazio projects leader
	case platformconfig.ProjectsLeaderKindIguazio:
		return iguazio.NewClient(parentLogger, platformConfiguration)

	case platformconfig.ProjectsLeaderKindMock:
		return mock.NewClient()
	}

	return nil, errors.Errorf("Unknown projects leader kind: %s", platformConfiguration.ProjectsLeader.Kind)
}
