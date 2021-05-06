package external

import (
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mlrun"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Client struct {
	platformConfiguration *platformconfig.Config
	internalClient        project.Client
	synchronizer          *iguazio.Synchronizer
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

	newClient.synchronizer, err = iguazio.NewSynchronizer(parentLogger, platformConfiguration, newClient.leaderClient, internalClient)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create synchronizer")
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {
	c.synchronizer.Start()

	return c.internalClient.Initialize()
}

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return c.internalClient.Get(getProjectsOptions)
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	switch createProjectOptions.RequestOrigin {
	case c.platformConfiguration.ProjectsLeader.Kind:
		return c.internalClient.Create(createProjectOptions)
	default:
		if err := c.leaderClient.Create(createProjectOptions); err != nil {
			return nil, errors.Wrap(err, "Failed while requesting from the leader to create the project")
		}

		return nil, nuclio.NewErrAccepted("Successfully requested from the leader to create the project")
	}
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	switch updateProjectOptions.RequestOrigin {
	case c.platformConfiguration.ProjectsLeader.Kind:
		return c.internalClient.Update(updateProjectOptions)
	default:
		if err := c.leaderClient.Update(updateProjectOptions); err != nil {
			return nil, errors.Wrap(err, "Failed while requesting from the leader to update the project")
		}

		return nil, nuclio.NewErrAccepted("Successfully requested from the leader to update the project")
	}
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	switch deleteProjectOptions.RequestOrigin {
	case c.platformConfiguration.ProjectsLeader.Kind:
		return c.internalClient.Delete(deleteProjectOptions)
	default:
		if err := c.leaderClient.Delete(deleteProjectOptions); err != nil {
			return errors.Wrap(err, "Failed while requesting from the leader to delete the project")
		}

		return nuclio.NewErrAccepted("Successfully requested from the leader to delete the project")
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
	}

	return nil, errors.Errorf("Unknown projects leader kind: %s", platformConfiguration.ProjectsLeader.Kind)
}
