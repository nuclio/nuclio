package kube

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/project"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


type Client struct {
	project.Client
	consumer *kube.Consumer
}

func NewClient(parentLogger logger.Logger, consumer *kube.Consumer) (*Client, error) {
	client := Client{}

	abstractClient, err := project.NewAbstractClient(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract client")
	}

	client.Client = abstractClient
	client.consumer = consumer

	return &client, nil
}

func (c *Client) Initialize(p platform.Platform) error {
	return p.EnsureDefaultProjectExistence()
}

func (c *Client) Create(newProject *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(newProject.Namespace).
		Create(newProject)
}

func (c *Client) Update(project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(project.Namespace).
		Update(project)
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

	return nil
}

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]nuclioio.NuclioProject, error) {
	var projects []nuclioio.NuclioProject

	// if identifier specified, we need to get a single NuclioProject
	if getProjectsOptions.Meta.Name != "" {

		// get specific NuclioProject CR
		Project, err := c.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			Get(getProjectsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return projects, nil
			}

			return nil, errors.Wrap(err, "Failed to get a project")
		}

		projects = append(projects, *Project)

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
