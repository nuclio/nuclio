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
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	kinesisclient "github.com/sendgridlabs/go-kinesis"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	kinesisClient   kinesisclient.KinesisClient
	streamName      string
	shardCount      int
	shards          []string
	useDummyKinesis bool

	brokerContainerPort   int
	brokerContainerName   string
	brokerSecretAccessKey string
	brokerAccessKeyID     string
	brokerRegionName      string
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{}
	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)
	return newTestSuite
}

func (suite *testSuite) SetupSuite() {

	// NOTE: use  dummy for testing purposes, change if you want to test against a production-like stream
	suite.useDummyKinesis = true
	suite.brokerSecretAccessKey = "a"
	suite.brokerAccessKeyID = "b"
	suite.brokerRegionName = "c"
	suite.streamName = "nuclio-test"
	suite.shards = []string{"shard-0", "shard-1", "shard-2"}
	suite.shardCount = len(suite.shards)

	// kineses test & function clients configuration
	suite.brokerContainerPort = 4567
	suite.brokerContainerName = "nuclio-kinesis"
	suite.BrokerContainerNetworkName = "nuclio-kinesis-network"

	// setup parent
	suite.AbstractBrokerSuite.SetupSuite()

	kinesisAuth := kinesisclient.NewAuth(suite.brokerAccessKeyID, suite.brokerSecretAccessKey, "")
	if suite.useDummyKinesis {
		suite.kinesisClient = kinesisclient.NewWithEndpoint(kinesisAuth,
			suite.brokerRegionName,
			fmt.Sprintf("http://%s:%d/", suite.BrokerHost, suite.brokerContainerPort))
	} else {
		suite.kinesisClient = kinesisclient.New(kinesisAuth, suite.brokerRegionName)
	}

	// create the stream
	err := suite.kinesisClient.CreateStream(suite.streamName, suite.shardCount)
	suite.Require().NoError(err)
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.getDeployOptions("kinesis-event-recorder")
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{suite.streamName: {NumMessages: suite.shardCount}},
		nil,
		suite.publishMessageToTopic)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	if !suite.useDummyKinesis {
		return "", nil
	}
	return "instructure/kinesalite", &dockerclient.RunOptions{
		ContainerName: suite.brokerContainerName,
		Ports:         map[int]int{suite.brokerContainerPort: suite.brokerContainerPort},
		Network:       suite.BrokerContainerNetworkName,
	}
}

func (suite *testSuite) getDeployOptions(functionName string) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions(functionName, suite.FunctionPaths["python"])

	if suite.useDummyKinesis {
		// function must be within the same network of broker to allow communication
		createFunctionOptions.FunctionConfig.Spec.Platform.Attributes = map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		}
	}

	// create function kinesis trigger
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"kinesisTrigger": suite.resolveKinesisFunctionTrigger(),
	}

	return createFunctionOptions
}

func (suite *testSuite) resolveKinesisFunctionTrigger() functionconfig.Trigger {
	kinesisEndpointURL := ""
	if suite.useDummyKinesis {
		kinesisEndpointURL = fmt.Sprintf("http://%s:%d", suite.brokerContainerName, suite.brokerContainerPort)
	}
	return functionconfig.Trigger{
		Kind: "kinesis",
		URL:  kinesisEndpointURL,
		Attributes: map[string]interface{}{
			"accessKeyID":     suite.brokerAccessKeyID,
			"secretAccessKey": suite.brokerSecretAccessKey,
			"regionName":      suite.brokerRegionName,
			"streamName":      suite.streamName,
			"shards":          suite.shards,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, message string) error {
	suite.Logger.InfoWith("Publishing message to topic", "topic", topic, "message", message)
	args := kinesisclient.NewArgs()
	args.Add("StreamName", topic)
	args.AddRecord([]byte(message), "partitionKey-"+message)
	_, err := suite.kinesisClient.PutRecord(args)
	return err
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
