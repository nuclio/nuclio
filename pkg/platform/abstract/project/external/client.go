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
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/client"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mlrun"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	platformConfiguration *platformconfig.Config
	synchronizer          *client.Synchronizer
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

	namespaces := platformConfiguration.ManagedNamespaces
	if len(namespaces) == 0 {
		namespaces = append(namespaces, common.ResolveDefaultNamespace(common.NuclioSelfNamespace))
	}

	newClient.leaderClient, err = newLeaderClient(parentLogger, platformConfiguration, namespaces[0])
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create leader client")
	}

	// get leader synchronization interval
	synchronizationIntervalStr := "0"
	if platformConfiguration.ProjectsLeader != nil {
		synchronizationIntervalStr = platformConfiguration.ProjectsLeader.SynchronizationInterval
	}

	newClient.synchronizer, err = client.NewSynchronizer(parentLogger,
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

// Create routes the request through 2PC evaluation when the request originates
// from the leader, or forwards it to the external leader HTTP client when it
// comes from a user. The X-Projects-Role header is retained for IG3 compatibility.
func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	if createProjectOptions.RequestOrigin == c.platformConfiguration.ProjectsLeader.Kind {
		shouldApply, existingProject, err := c.evaluateLeaderRequestWithCRD(ctx,
			createProjectOptions.ProjectConfig.Meta.Name,
			createProjectOptions.ProjectConfig.Meta.Namespace,
			createProjectOptions.ProjectConfig.Meta.Labels)
		if err != nil {
			return nil, err
		}
		if !shouldApply {
			return existingProject, nil // idempotent: same op_id already stored
		}
		return c.internalClient.Create(ctx, createProjectOptions)
	}

	if err := c.leaderClient.Create(ctx, createProjectOptions); err != nil {
		return nil, errors.Wrap(err, "Failed while requesting from the leader to create the project")
	}

	return nil, platform.ErrSuccessfulCreateProjectLeader
}

// Update routes the request through 2PC evaluation when the request originates
// from the leader, or forwards it to the external leader HTTP client.
func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	if updateProjectOptions.RequestOrigin == c.platformConfiguration.ProjectsLeader.Kind {
		shouldApply, existingProject, err := c.evaluateLeaderRequestWithCRD(ctx,
			updateProjectOptions.ProjectConfig.Meta.Name,
			updateProjectOptions.ProjectConfig.Meta.Namespace,
			updateProjectOptions.ProjectConfig.Meta.Labels)
		if err != nil {
			return nil, err
		}
		if !shouldApply {
			return existingProject, nil // idempotent
		}
		return c.internalClient.Update(ctx, updateProjectOptions)
	}

	if err := c.leaderClient.Update(ctx, updateProjectOptions); err != nil {
		return nil, errors.Wrap(err, "Failed while requesting from the leader to update the project")
	}

	return nil, platform.ErrSuccessfulUpdateProjectLeader
}

// Delete routes the request through 2PC evaluation when the request originates
// from the leader, or forwards it to the external leader HTTP client.
func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	if deleteProjectOptions.RequestOrigin == c.platformConfiguration.ProjectsLeader.Kind {
		shouldApply, _, err := c.evaluateLeaderRequestWithCRD(ctx,
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace,
			deleteProjectOptions.Meta.Labels)
		if err != nil {
			return err
		}
		if !shouldApply {
			return nil // idempotent: CRD already gone
		}
		return c.internalClient.Delete(ctx, deleteProjectOptions)
	}

	if err := c.leaderClient.Delete(ctx, deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed while requesting from the leader to delete the project")
	}

	return platform.ErrSuccessfulDeleteProjectLeader
}

// evaluateLeaderRequestWithCRD runs the full 2PC evaluation pipeline for a leader-origin
// write: fetch the existing CRD, then ask the leader whether the write should be applied,
// is idempotent (skip), or invalid (error).
//
// When 2PC is disabled on the configured leader (Iguazio pass-through, or MLRun with the
// feature flag off), the entire pipeline is short-circuited: EvaluateLeaderRequest would
// be an unconditional (true, nil) pass-through and the Get would be wasted, so we return
// (true, nil, nil) directly and let the caller proceed to the internal write.
func (c *Client) evaluateLeaderRequestWithCRD(ctx context.Context,
	name, namespace string,
	labels map[string]string) (bool, platform.Project, error) {
	if !c.leaderClient.ProjectSync2PCEnabled() {
		return true, nil, nil
	}

	existingProject, err := c.getExistingProject(ctx, name, namespace)
	if err != nil {
		return false, nil, errors.Wrap(err, "Failed to fetch existing project for leader evaluation")
	}
	shouldApply, err := c.leaderClient.EvaluateLeaderRequest(ctx, labels, existingProject)
	if err != nil {
		return false, existingProject, errors.Wrap(err, "Failed to evaluate leader request")
	}
	return shouldApply, existingProject, nil
}

func (c *Client) getExistingProject(ctx context.Context, name, namespace string) (platform.Project, error) {
	existingProjects, err := c.internalClient.Get(ctx, &platform.GetProjectsOptions{Meta: platform.ProjectMeta{Name: name, Namespace: namespace}})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch existing project")
	}
	if len(existingProjects) == 0 {
		return nil, nil
	}
	return existingProjects[0], nil
}

// newLeaderClient constructs the concrete leader HTTP client for the configured leader kind.
func newLeaderClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config, namespace string) (leader.Client, error) {
	var skipTLSVerification bool
	var leaderOps leader.LeaderOps
	switch platformConfiguration.ProjectsLeader.Kind {

	// mlrun projects leader
	case platformconfig.ProjectsLeaderKindMlrun:
		skipTLSVerification = true
		leaderOps = mlrun.NewLeaderOps(parentLogger, namespace, platformConfiguration.ProjectsLeader.ProjectSync2PCEnabled)

	// iguazio projects leader
	case platformconfig.ProjectsLeaderKindIguazio:
		skipTLSVerification = true
		leaderOps = iguazio.NewLeaderOps(parentLogger)

	case platformconfig.ProjectsLeaderKindMock:
		leaderOps = mock.NewLeaderOps()
	default:
		return nil, errors.Errorf("Unknown projects leader kind: %s", platformConfiguration.ProjectsLeader.Kind)
	}

	return client.NewClient(parentLogger, skipTLSVerification, platformConfiguration, leaderOps)
}
