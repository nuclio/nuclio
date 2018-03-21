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
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const triggerName = "test_cron"

type TestSuite struct {
	processorsuite.TestSuite
	event        triggertest.Event
	functionPath string
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	// use the python event recorder
	suite.functionPath = path.Join(suite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"python",
		"event_recorder.py")

	suite.event.Body = "hello world"
	suite.event.Headers = map[string]string{
		"h1": "v1",
		"h2": "v2",
	}
}

func (suite *TestSuite) TestPostEventPythonInterval() {
	createFunctionOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["interval"] = "3s"

	suite.invokeEventRecorder(createFunctionOptions)
}

func (suite *TestSuite) TestPostEventPythonSchedule() {
	createFunctionOptions := suite.getCronDeployOptions()

	// Once every 3 seconds. Should occur 3-4 times during a 10-second test
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["schedule"] = "*/3 * * * *"

	suite.invokeEventRecorder(createFunctionOptions)
}

func (suite *TestSuite) getCronDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "cron-trigger-test"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = suite.functionPath
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName] = functionconfig.Trigger{
		Kind: "cron",
		Attributes: map[string]interface{}{
			"event": map[string]interface{}{
				"body":    suite.event.Body,
				"headers": suite.event.Headers,
			},
		},
	}

	return createFunctionOptions
}

func (suite *TestSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions) {
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// Wait 10 seconds to give time for the container to trigger 3-4 events
		time.Sleep(10 * time.Second)

		// Set http request url
		url := fmt.Sprintf("http://%s:%d", suite.GetTestHost(), deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s; err: %v", url, err)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshal the body into a list
		var receivedEvents []triggertest.Event

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

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
