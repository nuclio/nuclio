package external

import (
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/platform/kube/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platform/kube/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platform/kube/project/external/leader/mlrun"
	"github.com/nuclio/nuclio/pkg/platform/kube/project/kube"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	*kube.Client
	leaderClient leader.Client
}

func NewClient(parentLogger logger.Logger,
	platform platform.Platform,
	consumer *client.Consumer,
	platformConfiguration *platformconfig.Config) (*Client, error) {

	newClient := Client{}

	// inherits from kube client - because for now we will want to create the project both on the external
	// project manager, and on the k8s platform (mainly for the use of nuctl)
	kubeClient, err := kube.NewClient(parentLogger, platform, consumer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create projects kube client")
	}

	newClient.Client = kubeClient
	newClient.leaderClient, err = newLeaderClient(parentLogger, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create leader client")
	}

	return &newClient, nil
}

func (c *Client) Initialize() error {

	// do nothing
	return nil
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (*nuclioio.NuclioProject, error) {
	switch createProjectOptions.RequestOrigin {
	case platform.RequestOriginLeader:
		return c.Client.Create(createProjectOptions)
	default:
		return nil, c.leaderClient.Create(createProjectOptions)
	}
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (*nuclioio.NuclioProject, error) {
	switch updateProjectOptions.RequestOrigin {
	case platform.RequestOriginLeader:
		return c.Client.Update(updateProjectOptions)
	default:
		return nil, c.leaderClient.Update(updateProjectOptions)
	}
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	switch deleteProjectOptions.RequestOrigin {
	case platform.RequestOriginLeader:
		return c.Client.Delete(deleteProjectOptions)
	default:
		return c.leaderClient.Delete(deleteProjectOptions)
	}
}

func newLeaderClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (leader.Client, error) {
	switch platformConfiguration.Kube.ProjectsLeader.Kind {

	// mlrun projects leader
	case platformconfig.ProjectsLeaderKindMlrun:
		return mlrun.NewClient(parentLogger, platformConfiguration)

	// iguazio projects leader
	case platformconfig.ProjectsLeaderKindIguazio:
		return iguazio.NewClient(parentLogger, platformConfiguration)
	}

	return nil, errors.New("Unknown projects leader type")
}
