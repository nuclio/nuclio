package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	Logger   logger.Logger
	platform platform.Platform
	consumer *client.Consumer
}

func NewClient(parentLogger logger.Logger,
	platformInstance platform.Platform,
	consumer *client.Consumer) (project.Client, error) {
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
		projectInstance, err := c.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			Get(ctx, getProjectsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformProjects, nil
			}

			return nil, errors.Wrap(err, "Failed to get a project")
		}

		projects = append(projects, *projectInstance)

	} else {

		projectInstanceList, err := c.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			List(ctx, metav1.ListOptions{LabelSelector: ""})
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

	nuclioProject, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(newProject.Namespace).
		Create(ctx, &newProject, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nuclio.WrapErrConflict(err)
		}
		return nil, errors.Wrap(err, "Failed to create nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	projectInstance, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(updateProjectOptions.ProjectConfig.Meta.Namespace).
		Get(ctx, updateProjectOptions.ProjectConfig.Meta.Name, metav1.GetOptions{})
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

	nuclioProject, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(projectInstance.Namespace).
		Update(ctx, projectInstance, metav1.UpdateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(deleteProjectOptions.Meta.Namespace).
		Delete(ctx, deleteProjectOptions.Meta.Name, metav1.DeleteOptions{}); err != nil {

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
