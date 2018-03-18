package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const triggerName string = "test_cron"

type event struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type TestSuite struct {
	processorsuite.TestSuite
	containerID  string
	eventCounter chan int
	event        event
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.event.Body = "hello world"
	suite.event.Headers = map[string]string{
		"h1": "v1",
		"h2": "v2",
	}
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.containerID != "" {
		suite.DockerClient.RemoveContainer(suite.containerID)
	}
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.FunctionDir = suite.getFunctionsPath()
	suite.eventCounter = make(chan int)
}

func (suite *TestSuite) TestPostEventPythonInterval() {
	createFunctionOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["interval"] = "3s"

	suite.invokeEventRecorder(createFunctionOptions, "python")
}

func (suite *TestSuite) TestPostEventPythonSchedule() {
	createFunctionOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["schedule"] = "*/3 * * * *"

	suite.invokeEventRecorder(createFunctionOptions, "python")
}

func (suite *TestSuite) getCronDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	if createFunctionOptions.FunctionConfig.Spec.Triggers == nil {
		createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
			triggerName: suite.getCronTriggerConfig(),
		}
	} else {
		createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName] = suite.getCronTriggerConfig()
	}

	return createFunctionOptions
}

func (suite *TestSuite) getCronTriggerConfig() functionconfig.Trigger {
	return functionconfig.Trigger{
		Kind: "cron",
		Attributes: map[string]interface{}{
			"event": map[string]interface{}{
				"body":    suite.event.Body,
				"headers": suite.event.Headers,
			},
		},
	}
}

func (suite *TestSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions, runtimeType string) {
	suite.Runtime = runtimeType
	createFunctionOptions.FunctionConfig.Spec.Runtime = runtimeType
	createFunctionOptions.FunctionConfig.Meta.Name = "cron-trigger-test"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// Wait 10 seconds to give time for the container to trigger 3-4 events
		time.Sleep(10 * time.Second)

		baseURL := "localhost"

		// Check if situation is dockerized, if so set url to given NUCLIO_TEST_HOST
		if os.Getenv("NUCLIO_TEST_HOST") != "" {
			baseURL = os.Getenv("NUCLIO_TEST_HOST")
		}

		// Set http request url
		url := fmt.Sprintf("http://%s:%d", baseURL, deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s; err: %v", url, err)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshal the body into a list
		var receivedEvents []event

		err = json.Unmarshal(marshalledResponseBody, &receivedEvents)
		suite.Require().NoError(err, "Failed to unmarshal response. Response: %s", marshalledResponseBody)

		recievedEventsAmount := len(receivedEvents)

		// Testing between 2 and 7 events because of potential lags between container startup and request handling
		suite.Require().Condition(
			assert.Comparison(func() bool { return recievedEventsAmount > 1 && recievedEventsAmount < 7 }),
			"Expected between 2 and 7 events (Optimal is exactly 4). Received %d", recievedEventsAmount)

		suite.Logger.DebugWith("Received events from container",
			"expected", 4,
			"actual", recievedEventsAmount)

		// compare bodies / headers
		for _, receivedEvent := range receivedEvents {
			suite.Require().Equal(suite.event.Body, receivedEvent.Body)
			suite.Require().Equal(suite.event.Headers, receivedEvent.Headers)
		}

		return true
	})
}

func (suite *TestSuite) getFunctionsPath() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "cron", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
