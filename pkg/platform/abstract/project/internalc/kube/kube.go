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

package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	Logger   logger.Logger
	platform platform.Platform
	consumer *nuclioclient.Consumer
}

func NewClient(parentLogger logger.Logger,
	platformInstance platform.Platform,
	consumer *nuclioclient.Consumer) (project.Client, error) {
	newClient := &Client{
		Logger:   parentLogger.GetChild("projects-kube"),
		consumer: consumer,
		platform: platformInstance,
	}

	return newClient, nil
}

func (c *Client) Initialize() error {
	return nil
}

func (c *Client) Get(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	var platformProjects []platform.Project
	var projects []nuclioio.NuclioProject

	// if identifier specified, we need to get a single NuclioProject
	if getProjectsOptions.Meta.Name != "" {

		// get specific NuclioProject CR
		projectInstance, err := c.consumer.NuclioClientSet.GetNuclioProject(ctx,
			getProjectsOptions.Meta.Namespace,
			getProjectsOptions.Meta.Name)

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformProjects, nil
			}

			return nil, errors.Wrap(err, "Failed to get a project")
		}

		projects = append(projects, *projectInstance)

	} else {

		projectInstanceList, err := c.consumer.NuclioClientSet.ListNuclioProjects(ctx,
			getProjectsOptions.Meta.Namespace,
			metav1.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list projects")
		}

		// convert []NuclioProject to []*NuclioProject
		projects = projectInstanceList.Items
	}

	// convert each nuclioio.NuclioProject -> platform.Project
	for projectInstanceIndex := 0; projectInstanceIndex < len(projects); projectInstanceIndex++ {
		projectInstance := projects[projectInstanceIndex]

		newProject, err := c.nuclioProjectToPlatformProject(&projectInstance)
		if err != nil {
			return nil, err
		}

		platformProjects = append(platformProjects, newProject)
	}

	return platformProjects, nil
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	newProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(createProjectOptions.ProjectConfig, &newProject)

	nuclioProject, err := c.consumer.NuclioClientSet.CreateNuclioProject(ctx, newProject.Namespace, &newProject)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nuclio.WrapErrConflict(err)
		}
		return nil, errors.Wrap(err, "Failed to create nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	projectInstance, err := c.consumer.NuclioClientSet.GetNuclioProject(ctx,
		updateProjectOptions.ProjectConfig.Meta.Namespace,
		updateProjectOptions.ProjectConfig.Meta.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nuclio.WrapErrNotFound(err)
		}
		return nil, errors.Wrap(err, "Failed to get a project")
	}

	updatedProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(&updateProjectOptions.ProjectConfig, &updatedProject)
	projectInstance.Spec = updatedProject.Spec
	projectInstance.Annotations = updatedProject.Annotations
	projectInstance.Labels = updatedProject.Labels
	projectInstance.Status = updatedProject.Status
	now := time.Now()
	projectInstance.Status.UpdatedAt = &now

	nuclioProject, err := c.consumer.NuclioClientSet.UpdateNuclioProject(ctx, projectInstance.Namespace, projectInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := c.consumer.NuclioClientSet.DeleteNuclioProject(ctx,
		deleteProjectOptions.Meta.Namespace,
		deleteProjectOptions.Meta.Name,
		metav1.DeleteOptions{}); err != nil {

		if apierrors.IsNotFound(err) {
			return nuclio.NewErrNotFound(fmt.Sprintf("Project %s not found", deleteProjectOptions.Meta.Name))
		}
		return errors.Wrapf(err,
			"Failed to delete project %s from namespace %s",
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace)
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return c.platform.WaitForProjectResourcesDeletion(ctx,
			&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}

// Follower operations are unsupported on this plain CRD client: the dedicated
// /api/v1/follower/projects/* surface is implemented by external.Client (when Oris is the
// configured leader), which wraps this client as its internalClient rather than being it.

// PrepareCreate is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) PrepareCreate(context.Context,
	*platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error) {
	return nil, platform.ErrUnsupportedMethod
}

// CommitCreate is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) CommitCreate(context.Context,
	*platform.CommitCreateProjectOptions) (*platform.Project2PCState, error) {
	return nil, platform.ErrUnsupportedMethod
}

// CommitUpdate is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) CommitUpdate(context.Context,
	*platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error) {
	return nil, platform.ErrUnsupportedMethod
}

// PrepareDelete is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) PrepareDelete(context.Context,
	*platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error) {
	return nil, platform.ErrUnsupportedMethod
}

// CommitDelete is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) CommitDelete(context.Context,
	*platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error) {
	return nil, platform.ErrUnsupportedMethod
}

// List is unsupported on the plain CRD client: implemented by external.Client instead.
func (c *Client) List(context.Context,
	*platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error) {
	return nil, platform.ErrUnsupportedMethod
}

func (c *Client) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.NuclioProject) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
	project.Status = platformProject.Status
}

func (c *Client) nuclioProjectToPlatformProject(nuclioProject *nuclioio.NuclioProject) (platform.Project, error) {
	return platform.NewAbstractProject(c.Logger,
		c.platform,
		platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:        nuclioProject.Name,
				Namespace:   nuclioProject.Namespace,
				Labels:      nuclioProject.Labels,
				Annotations: nuclioProject.Annotations,
			},
			Spec:   nuclioProject.Spec,
			Status: nuclioProject.Status,
		})
}
