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
	broker        *sarama.Broker
	producer      sarama.SyncProducer
	topic         string
	consumerGroup string
	initialOffset string
	NumPartitions int32
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		topic:         "myTopic",
		consumerGroup: "myConsumerGroup",
		initialOffset: "earliest",
		NumPartitions: 4,
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

func (suite *testSuite) SetupSuite() {
	suite.AbstractBrokerSuite.SetupSuite()

	suite.Logger.Info("Creating broker resources")

	// create broker
	suite.broker = sarama.NewBroker(fmt.Sprintf("%s:9092", suite.BrokerHost))

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_10_1_0

	// connect to the broker
	err := suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// init a create topic request
	createTopicsRequest := sarama.CreateTopicsRequest{}
	createTopicsRequest.TopicDetails = map[string]*sarama.TopicDetail{
		suite.topic: {
			NumPartitions:     suite.NumPartitions,
			ReplicationFactor: 1,
		},
	}

	// create topic
	resp, err := suite.broker.CreateTopics(&createTopicsRequest)
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic", "topic", suite.topic, "response", resp)

	// create a sync producer
	suite.producer, err = sarama.NewSyncProducer([]string{fmt.Sprintf("%s:9092", suite.BrokerHost)}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["http"] = functionconfig.Trigger{
		Kind:       "http",
		MaxWorkers: 1,
		URL:        ":8080",
		Attributes: map[string]interface{}{
			"port": 8080,
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = functionconfig.Trigger{
		Kind: "kafka-cluster",
		URL:  fmt.Sprintf("%s:9092", suite.BrokerHost),
		Attributes: map[string]interface{}{
			"topics":        []string{suite.topic},
			"consumerGroup": suite.consumerGroup,
			"initialOffset": suite.initialOffset,
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{suite.topic: {int(suite.NumPartitions)}},
		nil,
		suite.publishMessageToTopic)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "spotify/kafka", &dockerclient.RunOptions{
		Ports: map[int]int{2181: 2181, 9092: 9092},
		Env:   map[string]string{"ADVERTISED_HOST": suite.BrokerHost, "ADVERTISED_PORT": "9092"},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	producerMessage := sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder("key"),
		Value: sarama.StringEncoder(body),
	}

	suite.Logger.InfoWith("Producing")

	partition, offset, err := suite.producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

	return nil
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
