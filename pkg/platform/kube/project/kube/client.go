package kube

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/platform/kube/project"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	project.Client

	Logger   logger.Logger
	platform platform.Platform
	consumer *client.Consumer
}

func NewClient(parentLogger logger.Logger, platform platform.Platform, consumer *client.Consumer) (*Client, error) {
	newClient := Client{
		Logger:   parentLogger.GetChild("projects-client"),
		consumer: consumer,
		platform: platform,
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {
	return c.platform.EnsureDefaultProjectExistence()
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (*nuclioio.NuclioProject, error) {
	newProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(createProjectOptions.ProjectConfig, &newProject)

	return c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(newProject.Namespace).
		Create(&newProject)
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (*nuclioio.NuclioProject, error) {
	projectInstance, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(updateProjectOptions.ProjectConfig.Meta.Namespace).
		Get(updateProjectOptions.ProjectConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get a project")
	}

	updatedProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(&updateProjectOptions.ProjectConfig, &updatedProject)
	projectInstance.Spec = updatedProject.Spec
	projectInstance.Annotations = updatedProject.Annotations
	projectInstance.Labels = updatedProject.Labels

	return c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(projectInstance.Namespace).
		Update(projectInstance)
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(deleteProjectOptions.Meta.Namespace).
		Delete(deleteProjectOptions.Meta.Name, &metav1.DeleteOptions{}); err != nil {

		if apierrors.IsNotFound(err) {
			return nuclio.NewErrNotFound(fmt.Sprintf("Project %s not found", deleteProjectOptions.Meta.Name))
		}
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

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]nuclioio.NuclioProject, error) {
	var projects []nuclioio.NuclioProject

	// if identifier specified, we need to get a single NuclioProject
	if getProjectsOptions.Meta.Name != "" {

		// get specific NuclioProject CR
		projectInstance, err := c.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			Get(getProjectsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return projects, nil
			}

			return nil, errors.Wrap(err, "Failed to get a project")
		}

		projects = append(projects, *projectInstance)

	} else {

		projectInstanceList, err := c.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			List(metav1.ListOptions{LabelSelector: ""})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list projects")
		}

		// convert []NuclioProject to []*NuclioProject
		projects = projectInstanceList.Items
	}

	return projects, nil
}

func (c *Client) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.NuclioProject) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
}
