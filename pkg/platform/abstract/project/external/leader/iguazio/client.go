package iguazio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	ProjectsRoleHeaderValueNuclio = "nuclio"
	DefaultRequestTimeout         = 10 * time.Second

	// didn't use "x-nuclio.." prefix, because this header is used across iguazio, mlrun and nuclio (not nuclio specific)
	ProjectsRoleHeaderKey = "x-projects-role"
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
	var cookies []*http.Cookie

	c.logger.DebugWith("Sending create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace)

	// generate request body
	body, err := c.generateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// attach session cookie
	if createProjectOptions.SessionCookie != nil {
		cookies = append(cookies, createProjectOptions.SessionCookie)
	}

	// send the request
	headers := c.generateCommonRequestHeaders()
	responseBody, _, err := common.SendHTTPRequest(http.MethodPost,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		cookies,
		http.StatusCreated,
		true,
		DefaultRequestTimeout)
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
	var cookies []*http.Cookie

	c.logger.DebugWith("Sending update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)

	// generate request body
	body, err := c.generateProjectRequestBody(&updateProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// attach session cookie
	if updateProjectOptions.SessionCookie != nil {
		cookies = append(cookies, updateProjectOptions.SessionCookie)
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
		cookies,
		http.StatusOK,
		true,
		DefaultRequestTimeout)
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
	var cookies []*http.Cookie

	c.logger.DebugWith("Sending delete project request to leader",
		"name", deleteProjectOptions.Meta.Name)

	// generate request body
	body, err := c.generateProjectDeletionRequestBody(deleteProjectOptions.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project deletion request body")
	}

	// attach session cookie
	if deleteProjectOptions.SessionCookie != nil {
		cookies = append(cookies, deleteProjectOptions.SessionCookie)
	}

	// send the request
	headers := c.generateCommonRequestHeaders()
	headers["igz-project-deletion-strategy"] = string(deleteProjectOptions.Strategy)
	if _, _, err := common.SendHTTPRequest(http.MethodDelete,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		cookies,
		http.StatusAccepted,
		true,
		DefaultRequestTimeout); err != nil {

		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWith("Successfully sent delete project request to leader",
		"name", deleteProjectOptions.Meta.Name,
		"namespace", deleteProjectOptions.Meta.Namespace)

	return nil
}

func (c *Client) GetUpdatedAfter(updatedAfterTime *time.Time) ([]platform.Project, error) {
	c.logger.DebugWith("Fetching all projects from leader", "updatedAfterTime", updatedAfterTime)

	// if updatedAfterTime arg was specified, filter by it
	updatedAfterTimestampQuery := ""
	if updatedAfterTime != nil {
		updatedAfterTimestamp := updatedAfterTime.Format(time.RFC3339Nano)
		updatedAfterTimestampQuery = fmt.Sprintf("?filter[updated_at]=[$gt]%s", updatedAfterTimestamp)
	}

	// send the request
	headers := c.generateCommonRequestHeaders()
	responseBody, _, err := common.SendHTTPRequest(http.MethodGet,
		fmt.Sprintf("%s/%s%s",
			c.platformConfiguration.ProjectsLeader.APIAddress,
			"projects",
			updatedAfterTimestampQuery),
		nil,
		headers,
		[]*http.Cookie{{Name: "session", Value: c.platformConfiguration.IguazioSessionCookie}},
		http.StatusOK,
		true,
		DefaultRequestTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request to leader")
	}

	projectsList := ProjectList{}
	if err := json.Unmarshal(responseBody, &projectsList); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return projectsList.ToSingleProjectList(), nil
}

func (c *Client) generateCommonRequestHeaders() map[string]string {
	return map[string]string{
		ProjectsRoleHeaderKey: ProjectsRoleHeaderValueNuclio,
		"Content-Type":        "application/json",
	}
}

func (c *Client) generateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project := NewProjectFromProjectConfig(projectConfig)
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
