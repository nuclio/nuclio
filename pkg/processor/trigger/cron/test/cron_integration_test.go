package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

type TestSuite struct {
	processorsuite.TestSuite
	containerID  string
	eventCounter chan int
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
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
	deployOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	deployOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["interval"] = "3s"

	suite.invokeEventRecorder(deployOptions, "python")
}

func (suite *TestSuite) TestPostEventPythonSchedule() {
	deployOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	deployOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["schedule"] = "*/3 * * * *"

	suite.invokeEventRecorder(deployOptions, "python")
}

func (suite *TestSuite) getCronDeployOptions() *platform.DeployOptions {
	deployOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	if deployOptions.FunctionConfig.Spec.Triggers == nil {
		deployOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
			triggerName: suite.getCronTriggerConfig(),
		}
	} else {
		deployOptions.FunctionConfig.Spec.Triggers[triggerName] = suite.getCronTriggerConfig()
	}

	return deployOptions
}

func (suite *TestSuite) getCronTriggerConfig() functionconfig.Trigger {
	return functionconfig.Trigger{
		Kind: "cron",
		Attributes: map[string]interface{}{
			"body": "hello world",
			"headers": map[string]interface{}{
				"foo": "bar",
			},
		},
	}
}

func (suite *TestSuite) invokeEventRecorder(deployOptions *platform.DeployOptions, runtimeType string) {
	suite.Runtime = runtimeType
	deployOptions.FunctionConfig.Spec.Runtime = runtimeType
	deployOptions.FunctionConfig.Meta.Name = "nuclio/cron-trigger-test"

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {

		// Wait 10 seconds to give time for the container to trigger 3-4 events
		time.Sleep(10 * time.Second)

		url := fmt.Sprintf("http://localhost:%d", deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s; err: %v", url, err)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshal the body into a list
		var receivedEvents []string

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
