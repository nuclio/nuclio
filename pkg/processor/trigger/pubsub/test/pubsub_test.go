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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	pubsubtrigger "github.com/nuclio/nuclio/pkg/processor/trigger/pubsub"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/suite"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	// messaging
	client      *pubsub.Client
	topic       *pubsub.Topic
	projectID   string
	numMessages int

	// pubsub broker
	brokerPort          int
	brokerURL           string
	brokerContainerName string
}

func (suite *testSuite) SetupSuite() {
	var err error

	suite.projectID = "nuclio-test"

	// pubsub cluster
	suite.brokerPort = 9200
	suite.brokerContainerName = "nuclio-pubsub-server"
	suite.brokerURL = fmt.Sprintf("%s:%d", suite.BrokerHost, suite.brokerPort)
	suite.BrokerContainerNetworkName = "nuclio-pubsub-test"
	suite.LoggerName = "pubsub-test"

	suite.AbstractBrokerSuite.SetupSuite()
	suite.Logger.InfoWith("Creating pubsub broker resources",
		"brokerURL", suite.brokerURL)

	time.Sleep(10 * time.Second)

	conn, err := grpc.Dial(suite.brokerURL, grpc.WithInsecure()) // local address, insecure
	suite.Require().NoError(err)

	// pubsub client
	ctx := context.TODO()
	suite.client, err = pubsub.NewClient(ctx,
		suite.projectID,
		option.WithGRPCConn(conn),
		option.WithTelemetryDisabled(),
		option.WithoutAuthentication(),
	)
	suite.Require().NoError(err)

	// create topic
	suite.topic, err = suite.client.CreateTopic(ctx, "nuclio-test-topic")
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", suite.topic)

	suite.numMessages = 10

}

func (suite *testSuite) TestReceiveRecords() {
	pubsubContainerBrokerURL := fmt.Sprintf("%s:%d", suite.brokerContainerName, suite.brokerPort)
	createFunctionOptions := suite.GetDeployOptions("pubsub-event-recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Env = []v1.EnvVar{
		{
			Name:  "PUBSUB_EMULATOR_HOST",
			Value: pubsubContainerBrokerURL,
		},
	}

	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"test-pubsub": {
			Kind: "pubsub",
			URL:  pubsubContainerBrokerURL,
			Attributes: map[string]interface{}{
				"subscriptions": []pubsubtrigger.Subscription{
					{
						Topic: suite.topic.ID(),
					},
				},
				"projectID":     suite.projectID,
				"NoCredentials": true,
			},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.topic.ID(): {
				NumMessages: suite.numMessages,
			},
		},
		nil,
		suite.publishMessageToTopic)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "gcr.io/iguazio/nuclio/simple-pubsub-emulator:v1", &dockerclient.RunOptions{
		Network: suite.BrokerContainerNetworkName,
		Ports: map[int]int{
			suite.brokerPort: suite.brokerPort,
		},
		ContainerName: suite.brokerContainerName,
		Env: map[string]string{
			"PROJECT": suite.projectID,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	suite.Logger.DebugWith("Publishing message to topic", "topic", topic, "body", body)
	suite.topic.Publish(context.TODO(), &pubsub.Message{
		Data: []byte(body),
	})
	suite.Logger.Debug("Successfully published a message")
	return nil
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	testSuiteInstance := &testSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
