package iguazio

import (
	"fmt"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
)

type Synchronizer struct {
	logger                       logger.Logger
	synchronizationIntervalStr      string
	leaderClient                 leader.Client
	internalProjectsClient       project.Client
	mostRecentUpdatedProjectTime *time.Time
}

func NewSynchronizer(parentLogger logger.Logger,
	synchronizationIntervalStr string,
	leaderClient leader.Client,
	internalProjectsClient project.Client) (*Synchronizer, error) {

	parentLogger.DebugWith("Creating project synchronizer")
	newSynchronizer := Synchronizer{
		logger:                  parentLogger.GetChild("leader-synchronizer-iguazio"),
		synchronizationIntervalStr: synchronizationIntervalStr,
		leaderClient:            leaderClient,
		internalProjectsClient:  internalProjectsClient,
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
	go c.startSynchronizationLoop(synchronizationInterval)

	return nil
}

func (c *Synchronizer) startSynchronizationLoop(interval time.Duration) {
	c.logger.InfoWith("Starting synchronization loop", "interval", interval)

	ticker := time.NewTicker(interval)
	for range ticker.C {
		if err := c.synchronizeProjectsFromLeader(); err != nil {
			c.logger.WarnWith("Failed to synchronize projects according to leader", "err", err)
		}
	}
}

// a helper function - generates unique key to be used by projects maps
func (c *Synchronizer) generateUniqueProjectKey(configInstance *platform.ProjectConfig) string {
	return fmt.Sprintf("%s:%s", configInstance.Meta.Namespace, configInstance.Meta.Name)
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

		// check if it's the most recent updated project
		if mostRecentUpdatedProjectTime == nil || mostRecentUpdatedProjectTime.Before(leaderProjectConfig.Status.UpdatedAt) {
			mostRecentUpdatedProjectTime = &leaderProjectConfig.Status.UpdatedAt
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

func (c *Synchronizer) synchronizeProjectsFromLeader() error {

	// fetch updated projects from leader
	leaderProjects, err := c.leaderClient.GetUpdatedAfter(c.mostRecentUpdatedProjectTime)
	if err != nil {
		return errors.Wrap(err, "Failed to get projects from leader")
	}

	// fetch all internal projects
	internalProjects, err := c.internalProjectsClient.Get(&platform.GetProjectsOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to get projects from internal client")
	}

	// filter modified projects
	projectsToCreate, projectsToUpdate, mostRecentUpdatedProjectTime := c.getModifiedProjects(leaderProjects, internalProjects)
	if len(projectsToCreate) == 0 && len(projectsToUpdate) == 0 {

		// nothing to create/update - return
		return nil
	}

	c.logger.DebugWith("Synchronization loop modified projects",
		"projectsToCreateNum", len(projectsToCreate),
		"projectsToUpdateNum", len(projectsToUpdate))

	// create projects that exist on the leader but weren't created internally
	for _, projectInstance := range projectsToCreate {
		projectInstance := projectInstance
		go func() {
			c.logger.DebugWith("Creating project from leader sync", "projectInstance", *projectInstance)
			createProjectConfig := &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: projectInstance.Meta,
					Spec: projectInstance.Spec,
				},
			}
			if _, err := c.internalProjectsClient.Create(createProjectConfig); err != nil {
				c.logger.WarnWith("Failed to create project from leader sync",
					"name", createProjectConfig.ProjectConfig.Meta.Name,
					"namespace", createProjectConfig.ProjectConfig.Meta.Namespace,
					"err", err)
				return
			}
			c.logger.DebugWith("Successfully created project from leader sync",
				"name", createProjectConfig.ProjectConfig.Meta.Name,
				"namespace", createProjectConfig.ProjectConfig.Meta.Namespace)
		}()
	}

	// update projects that exist both internally and on the leader
	for _, projectInstance := range projectsToUpdate {
		projectInstance := projectInstance
		go func() {
			c.logger.DebugWith("Updating project from leader sync", "projectInstance", *projectInstance)
			updateProjectOptions := &platform.UpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: projectInstance.Meta,
					Spec: projectInstance.Spec,
				},
			}
			if _, err := c.internalProjectsClient.Update(updateProjectOptions); err != nil {
				c.logger.WarnWith("Failed to update project from leader sync",
					"name", updateProjectOptions.ProjectConfig.Meta.Name,
					"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace,
					"err", err)
				return
			}
			c.logger.DebugWith("Successfully updated project from leader sync",
				"name", updateProjectOptions.ProjectConfig.Meta.Name,
				"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)
		}()
	}

	// update most recent updated project time
	c.mostRecentUpdatedProjectTime = mostRecentUpdatedProjectTime

	return nil
}
