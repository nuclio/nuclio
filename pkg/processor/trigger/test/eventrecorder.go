/*
Copyright 2023 The Nuclio Authors.

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

package triggertest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Event struct {
	Body      string            `json:"body"`
	Headers   map[string]string `json:"headers"`
	Timestamp string            `json:"timestamp"`
}

type MessagePublisher func(string, string) error

type TopicMessages struct {
	NumMessages int
}

func InvokeEventRecorder(suite *processorsuite.TestSuite,
	host string,
	createFunctionOptions *platform.CreateFunctionOptions,
	numExpectedMessagesPerTopic map[string]TopicMessages,
	numNonExpectedMessagesPerTopic map[string]TopicMessages,
	messagePublisher MessagePublisher) {

	// deploy functions
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult, "Unexpected empty deploy results")
		var sentBodies []string

		suite.Logger.DebugWith("Producing",
			"numExpectedMessagesPerTopic", numExpectedMessagesPerTopic,
			"numNonExpectedMessagesPerTopic", numNonExpectedMessagesPerTopic)

		// send messages we expect to see arrive @ the function, each to their own topic
		for topic, topicMessages := range numExpectedMessagesPerTopic {
			for messageIdx := 0; messageIdx < topicMessages.NumMessages; messageIdx++ {
				messageBody := fmt.Sprintf("%s-%d", topic, messageIdx)

				// send the message
				err := messagePublisher(topic, messageBody)
				suite.Require().NoError(err, "Failed to publish message")

				// add body to bodies we expect to see in response
				sentBodies = append(sentBodies, messageBody)
			}
		}

		// send messages we *don't* expect to see arrive @ the function
		for topic, topicMessages := range numNonExpectedMessagesPerTopic {
			for messageIdx := 0; messageIdx < topicMessages.NumMessages; messageIdx++ {
				messageBody := fmt.Sprintf("%s-%d", topic, messageIdx)

				// send the message
				err := messagePublisher(topic, messageBody)
				suite.Require().NoError(err, "Failed to publish message")
			}
		}

		var receivedEvents []Event
		var getEventErr error
		err := common.RetryUntilSuccessful(10*time.Second,
			2*time.Second,
			func() bool {
				receivedEvents, getEventErr = GetEventRecorderReceivedEvents(suite.Logger, host, deployResult.Port)
				suite.Require().NoError(getEventErr)
				return len(receivedEvents) >= len(sentBodies)
			})
		suite.Require().NoError(err, "Failed to get events")
		suite.Logger.DebugWith("Done producing")

		var receivedBodies []string

		// compare only bodies due to a deficiency in CompareNoOrder
		for _, receivedEvent := range receivedEvents {

			// some brokers need data to be able to read the stream. these write "ignore", so we ignore that
			if receivedEvent.Body != "ignore" {
				receivedBodies = append(receivedBodies, receivedEvent.Body)
			}
		}

		sort.Strings(sentBodies)
		sort.Strings(receivedBodies)

		// compare bodies
		suite.Require().Equal(sentBodies, receivedBodies)

		return true
	})
}

func GetEventRecorderReceivedEvents(logger logger.Logger,
	functionHost string,
	functionPort int) ([]Event, error) {

	// Set the url for the http request
	url := fmt.Sprintf("http://%s:%d", functionHost, functionPort)

	// read the events from the function
	httpResponse, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get events")
	}

	marshalledResponseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response body")
	}
	logger.DebugWith("Got messages", "marshalledResponseBody", string(marshalledResponseBody))

	// unmarshall the body into a list
	// TODO: accept various of events
	var receivedEvents []Event

	if err := json.Unmarshal(marshalledResponseBody, &receivedEvents); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return receivedEvents, nil

}
