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
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	// kafka clients
	broker   *sarama.Broker
	producer sarama.SyncProducer

	// messaging
	topic         string
	consumerGroup string
	initialOffset string
	NumPartitions int32

	// kafka cluster
	brokerPort             int
	brokerURL              string
	brokerContainerName    string
	zooKeeperContainerName string

	// for cleanup
	zooKeeperContainerID string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// messaging
	suite.topic = "myTopic"
	suite.consumerGroup = "myConsumerGroup"
	suite.initialOffset = "earliest"
	suite.NumPartitions = 4

	// kafka cluster
	suite.brokerPort = 9092
	suite.brokerContainerName = "nuclio-kafka-broker"
	suite.zooKeeperContainerName = "nuclio-kafka-zookeeper"
	suite.brokerURL = fmt.Sprintf("%s:%d", suite.BrokerHost, suite.brokerPort)

	// start broker and zookeeper containers explicitly
	suite.AbstractBrokerSuite.SkipStartBrokerContainer = true
	suite.AbstractBrokerSuite.BrokerContainerNetworkName = "nuclio-kafka-test"
	suite.AbstractBrokerSuite.SetupSuite()

	// start zoo keeper container
	suite.zooKeeperContainerID = suite.RunContainer(suite.getKafkaZooKeeperContainerRunInfo())

	// start broker container
	suite.StartBrokerContainer(suite.GetContainerRunInfo())

	suite.Logger.InfoWith("Creating broker resources",
		"brokerHost", suite.BrokerHost)

	// create broker
	suite.broker = sarama.NewBroker(suite.brokerURL)

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_11_0_2

	// connect to the broker
	err = suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// create topic
	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			suite.topic: {
				NumPartitions:     suite.NumPartitions,
				ReplicationFactor: 1,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", suite.topic,
		"createTopicResponse", createTopicsResponse)

	// create a sync producer
	suite.producer, err = sarama.NewSyncProducer([]string{suite.brokerURL}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testSuite) TearDownSuite() {
	if suite.zooKeeperContainerID != "" {
		err := suite.DockerClient.RemoveContainer(suite.zooKeeperContainerID)
		suite.NoError(err)
	}

	suite.AbstractBrokerSuite.TearDownSuite()
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}

	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"my-kafka": {
			Kind: "kafka-cluster",
			URL:  fmt.Sprintf("%s:9090", suite.brokerContainerName),
			Attributes: map[string]interface{}{
				"topics":        []string{suite.topic},
				"consumerGroup": suite.consumerGroup,
				"initialOffset": suite.initialOffset,
			},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.topic: {
				NumMessages: int(suite.NumPartitions),
			},
		},
		nil,
		suite.publishMessageToTopic)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "wurstmeister/kafka", &dockerclient.RunOptions{
		ContainerName: suite.brokerContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        true,
		Ports: map[int]int{

			// broker
			suite.brokerPort: suite.brokerPort,
		},
		Env: map[string]string{
			"KAFKA_ZOOKEEPER_CONNECT":              fmt.Sprintf("%s:2181", suite.zooKeeperContainerName),
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":     "INTERNAL",
			"KAFKA_LISTENERS": fmt.Sprintf("INTERNAL://:9090,EXTERNAL://:%d",
				suite.brokerPort),
			"KAFKA_ADVERTISED_LISTENERS": fmt.Sprintf(
				"INTERNAL://%s:9090,EXTERNAL://%s:%d",
				suite.brokerContainerName,
				suite.BrokerHost,
				suite.brokerPort,
			),
		},
	}
}

func (suite *testSuite) getKafkaZooKeeperContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "wurstmeister/zookeeper", &dockerclient.RunOptions{
		ContainerName: suite.zooKeeperContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        true,
		Ports: map[int]int{
			dockerclient.RunOptionsNoPort: 2181,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	producerMessage := sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder("key"),
		Value: sarama.StringEncoder(body),
	}

	suite.Logger.InfoWith("Producing", "topic", topic, "body", body)

	partition, offset, err := suite.producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

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
