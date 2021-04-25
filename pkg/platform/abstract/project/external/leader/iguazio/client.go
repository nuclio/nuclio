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

const (
	ProjectsRoleHeaderKey         = "x-projects-role"
	ProjectsRoleHeaderValueNuclio = "nuclio"
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
	headers := c.generateCommonRequestHeaders()
	responseBody, _, err := common.SendHTTPRequest(http.MethodPost,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		[]*http.Cookie{createProjectOptions.SessionCookie},
		http.StatusCreated,
		true)
	if err != nil {
		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace,
		"responseBody", string(responseBody))

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
	headers := c.generateCommonRequestHeaders()
	responseBody, _, err := common.SendHTTPRequest(http.MethodPut,
		fmt.Sprintf("%s/%s/%s",
			c.platformConfiguration.ProjectsLeader.APIAddress,
			"projects/__name__",
			updateProjectOptions.ProjectConfig.Meta.Name),
		body,
		headers,
		[]*http.Cookie{updateProjectOptions.SessionCookie},
		http.StatusOK,
		true)
	if err != nil {
		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace,
		"responseBody", string(responseBody))

	return nil
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	c.logger.DebugWith("Sending delete project request to leader",
		"name", deleteProjectOptions.Meta.Name)

	// generate request body
	body, err := c.generateProjectDeletionRequestBody(deleteProjectOptions.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// send the request
	headers := c.generateCommonRequestHeaders()
	headers["igz-project-deletion-strategy"] = string(deleteProjectOptions.Strategy)
	if _, _, err := common.SendHTTPRequest(http.MethodDelete,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		[]*http.Cookie{deleteProjectOptions.SessionCookie},
		http.StatusAccepted,
		true); err != nil {

		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent delete project request to leader",
		"name", deleteProjectOptions.Meta.Name)

	return nil
}

func (c *Client) generateCommonRequestHeaders() map[string]string {
	return map[string]string{
		ProjectsRoleHeaderKey: ProjectsRoleHeaderValueNuclio,
		"Content-Type": "application/json",
	}
}

func (c *Client) generateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project := Project{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name: projectConfig.Meta.Name,
				Labels: projectConfig.Meta.Labels,
				Annotations: projectConfig.Meta.Annotations,
				Description: projectConfig.Spec.Description,
			},
		},
	}

	c.enrichProjectWithNuclioFields(&project)

	return json.Marshal(project)
}

func (c *Client) enrichProjectWithNuclioFields(project *Project) {

	// TODO: update this function when nuclio fields are added
	//project.Data.Attributes.NuclioProject = NuclioProject{}
}

func (c *Client) generateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(Project{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name: projectName,
			},
		},
	})
}
