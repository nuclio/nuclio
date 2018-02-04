/*
Copyright 2017 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

const (
	brokerProject = "nuclio.pubsub_trigger_test"
	brokerTopic   = "test_topic"
)

type TestSuite struct {
	processorsuite.TestSuite
	brokerClient       *pubsub.Client
	brokerSubscription *pubsub.Subscription
}

func (suite *TestSuite) SetupSuite() {
	var err error
	suite.TestSuite.SetupSuite()
	// TODO: check if Pub/Sub is enabled in gcloud
	suite.Require().NoError(err, "Failed to check Pub/Sub")
	suite.waitBrokerReady()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.FunctionDir = suite.getFunctionsPath()

	// create broker stuff
	suite.createBrokerResources()
}

func (suite *TestSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// delete broker stuff
	suite.deleteBrokerResources()
}

func (suite *TestSuite) TestPostEventGolang() {
	suite.invokeEventRecorder(path.Join("event_recorder", "event_recorder.go"), "golang")
}

func (suite *TestSuite) invokeEventRecorder(functionPath string, runtimeType string) {
	suite.Runtime = runtimeType

	deployOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(functionPath))

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		// var results []*pubsub.PublishResult
		// send 3 messages
		for requestIdx := 0; requestIdx < 3; requestIdx++ {

			message.ContentType = "application/json"
			message.Body = []byte(fmt.Sprintf(`{"request": "%d"}`, requestIdx))

			// publish the message
			result := suite.brokerClient.Topic(suite.brokerTopic).Publish(ctx, &pubsub.Message{
				Data: []byte(message),
			})
			_, err := result.Get(ctx)
			suite.Require().NoError(err, "Failed to publish to topic")
		}

		// TODO: retry until successful
		time.Sleep(2 * time.Second)

		// read the events from the function
		url := fmt.Sprintf("http://localhost:%d", deployResult.Port)
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s", url)
		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshall the body into a list
		var receivedEvents []map[string]string
		err = json.Unmarshal(marshalledResponseBody, &receivedEvents)
		suite.Require().NoError(err, "Failed to unmarshal response")

		// must have received 3 events
		suite.Require().Equal([]map[string]string{
			{"request": "0"},
			{"request": "1"},
			{"request": "2"},
		}, receivedEvents)

		return true
	})
}

func (suite *TestSuite) createBrokerResources() {
	var err error

	ctx := context.Background()
	suite.brokerClient, err = pubsub.NewClient(ctx)
	suite.Require().NoError(err, "Failed to create a connection to broker")

	suite.brokerSubscription, err = suite.brokerClient.Subscription(suite.brokerTopic)
	suite.Require().NoError(err, "Failed to create a subscription to topic")

	cctx, cancel := context.WithCancel(ctx)
	err := sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		received++
		if received >= 3 {
			cancel()
			msg.Nack()
			return
		}
		fmt.Printf("Got message: %q\n", string(msg.Data))
		msg.Ack()
	})
	suite.Require().NoError(err, "Failed to recieve messages")
}

func (suite *TestSuite) deleteBrokerResources() {

	// delete the subscription
	suite.brokerSubscription.Delete(ctx)

	// delete the topic
	suite.brokerClient.Topic(suite.brokerTopic)
}

func (suite *TestSuite) waitBrokerReady() {
	time.Sleep(5 * time.Second)
}

func (suite *TestSuite) getFunctionsPath() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "pubsub", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
