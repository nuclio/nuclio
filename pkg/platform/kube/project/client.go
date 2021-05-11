package project

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform"
	abstractproject "github.com/nuclio/nuclio/pkg/platform/abstract/project"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	Logger        logger.Logger
	platform      platform.Platform
	consumer      *client.Consumer
	projectsCache []platform.Project
}

func NewClient(parentLogger logger.Logger, platformInstance platform.Platform, consumer *client.Consumer) (abstractproject.Client, error) {
	newClient := Client{
		Logger:   parentLogger.GetChild("projects-client-kube"),
		consumer: consumer,
		platform: platformInstance,
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {
	c.Logger.DebugWith("Initializing projects client")
	if err := c.syncProjectsCache(); err != nil {
		return errors.Wrap(err, "Failed to sync projects cache")
	}
	return nil
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	newProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(createProjectOptions.ProjectConfig, &newProject)

	nuclioProject, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(newProject.Namespace).
		Create(&newProject)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio project")
	}

	platformProject, err := c.nuclioProjectToPlatformProject(nuclioProject)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert nuclio project to platform project")
	}

	// add project to cache
	c.addProjectToCache(platformProject)

	return platformProject, nil
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
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

	nuclioProject, err := c.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioProjects(projectInstance.Namespace).
		Update(projectInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update nuclio project")
	}

	platformProject, err := c.nuclioProjectToPlatformProject(nuclioProject)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert nuclio project to platform project")
	}

	// update project in cache
	c.updateProjectInCache(platformProject)

	return platformProject, nil
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

	// delete project from cache
	c.deleteProjectFromCache(deleteProjectOptions.Meta.Namespace, deleteProjectOptions.Meta.Name)

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return c.platform.WaitForProjectResourcesDeletion(&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {

	// first, try getting projects from cache
	if c.projectsCache != nil {
		return c.getProjectsFromCache(getProjectsOptions), nil
	}

	return c.getProjectsFromKube(getProjectsOptions)
}

func (c *Client) getProjectsFromKube(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	var platformProjects []platform.Project
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
				return platformProjects, nil
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

func (c *Client) syncProjectsCache() error {
	c.projectsCache = []platform.Project{}

	// get all managed namespaces
	namespaces, err := c.platform.GetNamespaces()
	if err != nil {
		namespaces = []string{c.platform.ResolveDefaultNamespace("")}
	}

	for _, namespace := range namespaces {
		projectsInNamespace, err := c.getProjectsFromKube(&platform.GetProjectsOptions{Meta: platform.ProjectMeta{Namespace: namespace}})
		if err != nil {
			return errors.Wrapf(err, "Failed to get projects in namespace: %s", namespace)
		}

		c.projectsCache = append(c.projectsCache, projectsInNamespace...)
	}

	return nil
}

func (c *Client) addProjectToCache(projectInstance platform.Project) {
	c.projectsCache = append(c.projectsCache, projectInstance)
}

func (c *Client) updateProjectInCache(projectInstance platform.Project) {

	// delete project and re-add it to update the cache
	c.deleteProjectFromCache(projectInstance.GetConfig().Meta.Namespace, projectInstance.GetConfig().Meta.Name)
	c.addProjectToCache(projectInstance)
}

func (c *Client) deleteProjectFromCache(namespace, name string) {
	newProjectsCache := []platform.Project{}

	for _, projectInstance := range c.projectsCache {
		projectConfig := projectInstance.GetConfig()
		if projectConfig.Meta.Namespace == namespace && projectConfig.Meta.Name == name {
			continue
		}
		newProjectsCache = append(newProjectsCache, projectInstance)
	}

	c.projectsCache = newProjectsCache
}

func (c *Client) getProjectsFromCache(getProjectOptions *platform.GetProjectsOptions) []platform.Project {
	matchingProjects := []platform.Project{}

	for _, projectInstance := range c.projectsCache {
		projectConfig := projectInstance.GetConfig()

		// if a specific namespace was requested and this project is not in it - skip it
		if projectConfig.Meta.Namespace != getProjectOptions.Meta.Namespace {
			continue
		}

		// if a specific namespace and name were requested - return this project (can't be more than one)
		if getProjectOptions.Meta.Name != "" {
			if projectConfig.Meta.Name == getProjectOptions.Meta.Name {
				return []platform.Project{projectInstance}
			}
		}

		matchingProjects = append(matchingProjects, projectInstance)
	}

	return matchingProjects
}

func (c *Client) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.NuclioProject) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
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
			Spec: nuclioProject.Spec,
		})
}
