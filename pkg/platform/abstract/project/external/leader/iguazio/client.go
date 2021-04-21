package iguazio

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config
}

func NewClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (*Client, error) {
	newClient := Client{
		logger:                parentLogger.GetChild("leader-client-iguazio"),
		platformConfiguration: platformConfiguration,
	}

	return &newClient, nil
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) error {
	c.logger.DebugWith("Sending create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace)

	// generate request body
	body, err := c.generateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// send the request
	if err := common.SendHTTPRequest(http.MethodPost,
		fmt.Sprintf("%s%s", c.platformConfiguration.ProjectsLeader.Address, "/projects"),
		body,
		[]*http.Cookie{createProjectOptions.SessionCookie},
		http.StatusAccepted); err != nil {

		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace)

	return nil
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) error {
	c.logger.DebugWith("Sending update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)

	// generate request body
	body, err := c.generateProjectRequestBody(&updateProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// send the request
	if err := common.SendHTTPRequest(http.MethodPut,
		fmt.Sprintf("%s%s", c.platformConfiguration.ProjectsLeader.Address, "/projects"),
		body,
		[]*http.Cookie{updateProjectOptions.SessionCookie},
		http.StatusAccepted); err != nil {

		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)

	return nil
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	c.logger.DebugWith("Sending delete project request to leader",
		"name", deleteProjectOptions.Meta.Name,
		"namespace", deleteProjectOptions.Meta.Namespace)

	// send the request
	if err := common.SendHTTPRequest(http.MethodDelete,
		fmt.Sprintf("%s%s", c.platformConfiguration.ProjectsLeader.Address, "/projects"),
		nil,
		[]*http.Cookie{deleteProjectOptions.SessionCookie},
		http.StatusAccepted); err != nil {

		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent delete project request to leader",
		"name", deleteProjectOptions.Meta.Name,
		"namespace", deleteProjectOptions.Meta.Namespace)

	return nil
}

func (c *Client) generateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"attributes": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": projectConfig.Meta.Name,
				"namespace": projectConfig.Meta.Namespace,
				"labels": projectConfig.Meta.Labels,
				"annotations": projectConfig.Meta.Annotations,
			},
			"spec": map[string]interface{}{
				"description": projectConfig.Spec.Description,
			},
		},
	})
}
