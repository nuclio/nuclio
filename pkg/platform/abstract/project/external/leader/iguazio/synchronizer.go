package iguazio

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Synchronizer struct {
	logger                      logger.Logger
	platformConfiguration       *platformconfig.Config
	leaderClient                leader.Client
	internalProjectsClient      project.Client
	lastSuccessfulSyncTimestamp string
}

func NewSynchronizer(parentLogger logger.Logger,
	platformConfiguration *platformconfig.Config,
	leaderClient leader.Client,
	internalProjectsClient project.Client) (*Synchronizer, error) {

	parentLogger.DebugWith("Creating project synchronizer")
	newSynchronizer := Synchronizer{
		logger:                 parentLogger.GetChild("leader-synchronizer-iguazio"),
		platformConfiguration:  platformConfiguration,
		leaderClient:           leaderClient,
		internalProjectsClient: internalProjectsClient,
	}

	return &newSynchronizer, nil
}

func (c *Synchronizer) Start() error {
	synchronizationInterval, err := time.ParseDuration(c.platformConfiguration.ProjectsLeader.SynchronizationInterval)
	if err != nil {
		return errors.Wrap(err, "Failed to parse synchronization interval")
	}

	// don't synchronize when set to 0
	if synchronizationInterval == 0 {
		c.logger.InfoWith("Synchronization interval set to 0. Projects will not synchronize with leader")
		return nil
	}

	// start synchronization loop in the background
	go c.synchronizationLoop(synchronizationInterval)

	return nil
}

func (c *Synchronizer) synchronizationLoop(interval time.Duration) {
	c.logger.InfoWith("Starting synchronization loop", "interval", interval)

	ticker := time.NewTicker(interval)
	for range ticker.C {
		if err := c.synchronizeProjectsAccordingToLeader(); err != nil {
			c.logger.WarnWith("Failed to synchronize projects according to leader", "err", err)
			continue
		}

		// update last successful sync timestamp
		c.updateLastSuccessfulSyncTimestamp()
	}
}

func (c *Synchronizer) updateLastSuccessfulSyncTimestamp() {
	loc, err := time.LoadLocation("GMT")
	if err != nil {
		c.logger.WarnWith("Failed to load GMT location (Should not happen on unix based systems). "+
			"Skipping last successful sync timestamp update",
			"err", err)
		return
	}

	t := time.Now().In(loc)
	c.lastSuccessfulSyncTimestamp = t.Format(time.RFC3339)
}

func (c *Synchronizer) getModifiedProjects(leaderProjects []platform.Project, internalProjects []platform.Project) (
	projectsToCreate []*platform.ProjectConfig,
	projectsToUpdate []*platform.ProjectConfig) {

	// a helper function - generates unique key to be used by the projects map later
	generateUniqueProjectKey := func(configInstance *platform.ProjectConfig) string {
		return fmt.Sprintf("%s:%s", configInstance.Meta.Namespace, configInstance.Meta.Name)
	}

	// create a mapping of all internal projects
	internalProjectsMap := map[string]*platform.ProjectConfig{}
	for _, internalProject := range internalProjects {
		internalProjectConfig := internalProject.GetConfig()
		if internalProjectConfig == nil {
			continue
		}

		namespaceAndNameKey := generateUniqueProjectKey(internalProjectConfig)
		internalProjectsMap[namespaceAndNameKey] = internalProjectConfig
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

		// check if the project exists internally
		namespaceAndNameKey := generateUniqueProjectKey(leaderProjectConfig)
		matchingInternalProjectConfig, found := internalProjectsMap[namespaceAndNameKey]
		if !found {
			projectsToCreate = append(projectsToCreate, leaderProjectConfig)
		} else if !matchingInternalProjectConfig.IsEqual(leaderProjectConfig, true) {

			// if the project exists both internally and on the leader - update it
			projectsToUpdate = append(projectsToUpdate, leaderProjectConfig)
		}
	}

	return
}

func (c *Synchronizer) synchronizeProjectsAccordingToLeader() error {

	// fetch projects from leader (created/updated since last sync)
	leaderProjects, err := c.leaderClient.GetAll(c.lastSuccessfulSyncTimestamp)
	if err != nil {
		return errors.Wrap(err, "Failed to get leader projects")
	}

	// fetch all internal projects
	internalProjects, err := c.internalProjectsClient.Get(&platform.GetProjectsOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to get all internal projects")
	}

	// filter modified projects
	projectsToCreate, projectsToUpdate := c.getModifiedProjects(leaderProjects, internalProjects)
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
			c.logger.DebugWith("Creating project (during sync)", "projectInstance", *projectInstance)
			createProjectConfig := &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: projectInstance.Meta,
					Spec: projectInstance.Spec,
				},
			}
			if _, err := c.internalProjectsClient.Create(createProjectConfig); err != nil {
				c.logger.WarnWith("Failed to create project (during sync)",
					"name", createProjectConfig.ProjectConfig.Meta.Name,
					"namespace", createProjectConfig.ProjectConfig.Meta.Namespace,
					"err", err)
				return
			}
			c.logger.DebugWith("Successfully created project (during sync)",
				"name", createProjectConfig.ProjectConfig.Meta.Name,
				"namespace", createProjectConfig.ProjectConfig.Meta.Namespace)
		}()
	}

	// update projects that exist both internally and on the leader
	for _, projectInstance := range projectsToUpdate {
		projectInstance := projectInstance
		go func() {
			c.logger.DebugWith("Updating project (during sync)", "projectInstance", *projectInstance)
			updateProjectOptions := &platform.UpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: projectInstance.Meta,
					Spec: projectInstance.Spec,
				},
			}
			if _, err := c.internalProjectsClient.Update(updateProjectOptions); err != nil {
				c.logger.WarnWith("Failed to update project (during sync)",
					"name", updateProjectOptions.ProjectConfig.Meta.Name,
					"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace,
					"err", err)
				return
			}
			c.logger.DebugWith("Successfully updated project (during sync)",
				"name", updateProjectOptions.ProjectConfig.Meta.Name,
				"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)
		}()
	}

	return nil
}
