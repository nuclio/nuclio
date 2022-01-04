//go:build test_integration && test_local

/*
Copyright 2018 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	"github.com/stretchr/testify/suite"
)

const triggerName = "test_cron"

type TestSuite struct {
	processorsuite.TestSuite

	// TODO: use cron.Event instead
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

	tests := []struct {
		duration time.Duration
		interval time.Duration
	}{
		{10 * time.Second, 3 * time.Second},
		{2 * time.Second, 250 * time.Millisecond},
	}

	for _, test := range tests {
		createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["interval"] = test.interval.String()
		expectedOccurredEvents := int(test.duration / test.interval)
		suite.Logger.DebugWith("Invoking event recorder",
			"test", test,
			"expectedOccurredEvents", expectedOccurredEvents,
			"interval", test.interval.String())
		suite.invokeEventRecorder(createFunctionOptions,
			test.duration,
			expectedOccurredEvents-1,
			expectedOccurredEvents+1)
	}
}

func (suite *TestSuite) TestPostEventPythonSchedule() {
	createFunctionOptions := suite.getCronDeployOptions()

	// Once every 3 seconds
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["schedule"] = "*/3 * * * * *"
	testDurationLength := 10 * time.Second
	minimumOccurredEvents := 3
	maximumOccurredEvents := 4
	suite.invokeEventRecorder(createFunctionOptions, testDurationLength, minimumOccurredEvents, maximumOccurredEvents)
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

func (suite *TestSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions,
	testDurationLength time.Duration,
	minimumOccurredEvents int,
	maximumOccurredEvents int) {
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// give time for the container to trigger its events
		start := time.Now()
		time.Sleep(testDurationLength)
		end := time.Now()

		// Set http request url
		url := fmt.Sprintf("http://%s:%d", suite.GetTestHost(), deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s; err: %v", url, err)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshal the body into a list
		var allEvents []triggertest.Event
		var relevantEvents []triggertest.Event

		err = json.Unmarshal(marshalledResponseBody, &allEvents)
		suite.Require().NoError(err, "Failed to unmarshal response. Response: %s", marshalledResponseBody)

		// we want only events that occurred in our test time span
		for _, event := range allEvents {
			eventDate, err := time.Parse("2006-01-02T15:04:05.000000", event.Timestamp)
			suite.Assert().NoError(err)

			// event happened within the required time frame
			if eventDate.After(start) && eventDate.Before(end) {
				relevantEvents = append(relevantEvents, event)
			}
		}

		receivedEventsAmount := len(relevantEvents)
		suite.Require().Condition(
			func() bool {
				return receivedEventsAmount >= minimumOccurredEvents && receivedEventsAmount <= maximumOccurredEvents
			},
			"Expected between %d and %d events. Received %d",
			minimumOccurredEvents,
			maximumOccurredEvents,
			receivedEventsAmount)

		suite.Logger.DebugWith("Received events from container",
			"minimumOccurredEvents", minimumOccurredEvents,
			"maximumOccurredEvents", maximumOccurredEvents,
			"actual", receivedEventsAmount)

		// compare bodies / headers
		for _, receivedEvent := range relevantEvents {
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
