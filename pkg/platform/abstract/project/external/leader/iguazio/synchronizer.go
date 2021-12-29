package iguazio

import (
	"context"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Synchronizer struct {
	logger                     logger.Logger
	synchronizationIntervalStr string
	managedNamespaces          []string
	leaderClient               leader.Client
	internalProjectsClient     project.Client
}

func NewSynchronizer(parentLogger logger.Logger,
	synchronizationIntervalStr string,
	managedNamespaces []string,
	leaderClient leader.Client,
	internalProjectsClient project.Client) (*Synchronizer, error) {

	parentLogger.DebugWith("Creating project synchronizer")
	newSynchronizer := Synchronizer{
		logger:                     parentLogger.GetChild("leader-synchronizer-iguazio"),
		synchronizationIntervalStr: synchronizationIntervalStr,
		leaderClient:               leaderClient,
		internalProjectsClient:     internalProjectsClient,
		managedNamespaces:          managedNamespaces,
	}

	return &newSynchronizer, nil
}

func (c *Synchronizer) Start() error {
	synchronizationInterval, err := time.ParseDuration(c.synchronizationIntervalStr)
	if err != nil {
		return errors.Wrap(err, "Failed to parse synchronization interval")
	}

	// don't synchronize when set to 0
	if synchronizationInterval == 0 {
		c.logger.InfoWith("Synchronization interval set to 0. Projects will not synchronize with leader")
		return nil
	}

	// start synchronization loop in the background
	go c.startSynchronizationLoop(synchronizationInterval, c.managedNamespaces)

	return nil
}

func (c *Synchronizer) startSynchronizationLoop(interval time.Duration, namespaces []string) {
	namespaceToMostRecentUpdatedProjectTimeMap := map[string]*time.Time{}

	// fil it up with default
	for _, namespace := range namespaces {
		namespaceToMostRecentUpdatedProjectTimeMap[namespace] = nil
	}

	c.logger.InfoWith("Starting synchronization loop",
		"namespaces", namespaces,
		"interval", interval)

	ticker := time.NewTicker(interval)
	for range ticker.C {
		for _, namespace := range namespaces {
			newMostRecentUpdatedProjectTime, err := c.synchronizeProjectsFromLeader(namespace,
				namespaceToMostRecentUpdatedProjectTimeMap[namespace])
			if err != nil {
				c.logger.WarnWith("Failed to synchronize projects according to leader",
					"err", errors.GetErrorStackString(err, 10))
			}

			// update most recent updated project time
			if newMostRecentUpdatedProjectTime != nil {
				namespaceToMostRecentUpdatedProjectTimeMap[namespace] = newMostRecentUpdatedProjectTime
			}
		}
	}
}

func (c *Synchronizer) getModifiedProjects(leaderProjects []platform.Project, internalProjects []platform.Project) (
	projectsToCreate []*platform.ProjectConfig,
	projectsToUpdate []*platform.ProjectConfig,
	mostRecentUpdatedProjectTime *time.Time) {

	// create a mapping of all internal projects
	internalProjectsMap := map[string]*platform.ProjectConfig{}
	for _, internalProject := range internalProjects {
		internalProjectConfig := internalProject.GetConfig()
		if internalProjectConfig == nil {
			continue
		}

		projectKey := c.generateUniqueProjectKey(internalProjectConfig)
		internalProjectsMap[projectKey] = internalProjectConfig
	}

	// iterate over leader projects and figure which we should create/update
	for _, leaderProject := range leaderProjects {
		leaderProjectConfig := leaderProject.GetConfig()

		// skip projects that their status is not online
		if leaderProjectConfig == nil ||
			leaderProjectConfig.Status.OperationalStatus != "online" ||
			leaderProjectConfig.Status.AdminStatus != "online" {
			continue
		}

		// no time was given, set it to empty
		if leaderProjectConfig.Status.UpdatedAt == nil {
			leaderProjectConfig.Status.UpdatedAt = &time.Time{}
		}

		// check if it's the most recent updated project
		if mostRecentUpdatedProjectTime == nil || mostRecentUpdatedProjectTime.Before(*leaderProjectConfig.Status.UpdatedAt) {
			mostRecentUpdatedProjectTime = leaderProjectConfig.Status.UpdatedAt
		}

		// check if the project exists internally
		projectKey := c.generateUniqueProjectKey(leaderProjectConfig)
		matchingInternalProjectConfig, found := internalProjectsMap[projectKey]
		if !found {
			projectsToCreate = append(projectsToCreate, leaderProjectConfig)
		} else if !matchingInternalProjectConfig.IsEqual(leaderProjectConfig, true) {

			// if the project exists both internally and on the leader - update it
			projectsToUpdate = append(projectsToUpdate, leaderProjectConfig)
		}
	}

	return
}

func (c *Synchronizer) synchronizeProjectsFromLeader(namespace string,
	mostRecentUpdatedProjectTime *time.Time) (*time.Time, error) {

	// fetch updated projects from leader
	leaderProjects, err := c.leaderClient.GetUpdatedAfter(mostRecentUpdatedProjectTime)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects from leader")
	}

	// fetch all internal projects
	internalProjects, err := c.internalProjectsClient.Get(context.Background(), &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Namespace: namespace,
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get projects from internal client")
	}

	// filter modified projects
	projectsToCreate, projectsToUpdate, newMostRecentUpdatedProjectTime := c.getModifiedProjects(leaderProjects, internalProjects)

	if len(projectsToCreate) == 0 && len(projectsToUpdate) == 0 {

		// nothing to create/update - return
		return newMostRecentUpdatedProjectTime, nil
	}

	c.logger.DebugWith("Synchronization loop modified projects",
		"projectsToCreateNum", len(projectsToCreate),
		"projectsToUpdateNum", len(projectsToUpdate))

	// create projects that exist on the leader but weren't created internally
	createProjectErrGroup, _ := errgroup.WithContextSemaphore(context.Background(), c.logger, errgroup.DefaultErrgroupConcurrency)
	for _, projectInstance := range projectsToCreate {
		projectInstance := projectInstance
		createProjectErrGroup.Go("create projects", func() error {
			c.logger.DebugWith("Creating project from leader sync", "projectInstance", *projectInstance)
			createProjectConfig := &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta:   projectInstance.Meta,
					Spec:   projectInstance.Spec,
					Status: projectInstance.Status,
				},
			}
			if _, err := c.internalProjectsClient.Create(context.Background(), createProjectConfig); err != nil {
				c.logger.WarnWith("Failed to create project from leader sync",
					"name", createProjectConfig.ProjectConfig.Meta.Name,
					"namespace", createProjectConfig.ProjectConfig.Meta.Namespace,
					"err", err)
				return err
			}
			c.logger.DebugWith("Successfully created project from leader sync",
				"name", createProjectConfig.ProjectConfig.Meta.Name,
				"namespace", createProjectConfig.ProjectConfig.Meta.Namespace)
			return nil
		})
	}

	if err := createProjectErrGroup.Wait(); err != nil {
		return nil, errors.Wrap(err, "Failed to create projects")
	}

	// update projects that exist both internally and on the leader
	updateProjectErrGroup, _ := errgroup.WithContextSemaphore(context.Background(), c.logger, errgroup.DefaultErrgroupConcurrency)
	for _, projectInstance := range projectsToUpdate {
		projectInstance := projectInstance
		updateProjectErrGroup.Go("update projects", func() error {
			c.logger.DebugWith("Updating project from leader sync", "projectInstance", *projectInstance)
			updateProjectOptions := &platform.UpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta:   projectInstance.Meta,
					Spec:   projectInstance.Spec,
					Status: projectInstance.Status,
				},
			}
			if _, err := c.internalProjectsClient.Update(context.Background(), updateProjectOptions); err != nil {
				c.logger.WarnWith("Failed to update project from leader sync",
					"name", updateProjectOptions.ProjectConfig.Meta.Name,
					"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace,
					"err", err)
				return err
			}
			c.logger.DebugWith("Successfully updated project from leader sync",
				"name", updateProjectOptions.ProjectConfig.Meta.Name,
				"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)
			return nil
		})
	}

	if err := updateProjectErrGroup.Wait(); err != nil {
		return nil, errors.Wrap(err, "Failed to update projects")
	}

	return newMostRecentUpdatedProjectTime, nil
}

// a helper function - generates unique key to be used by projects maps
func (c *Synchronizer) generateUniqueProjectKey(configInstance *platform.ProjectConfig) string {
	return fmt.Sprintf("%s:%s", configInstance.Meta.Namespace, configInstance.Meta.Name)
}
