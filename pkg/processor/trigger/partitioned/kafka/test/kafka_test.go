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
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/suite"
)

const (
	brokerPort = 9092
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	broker    *sarama.Broker
	brokerURL string
	producer  sarama.SyncProducer
	topic     string
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		topic: "test-topic",
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

func (suite *testSuite) SetupSuite() {
	suite.AbstractBrokerSuite.SetupSuite()

	suite.Logger.Info("Creating broker resources")

	// create broker
	suite.brokerURL = fmt.Sprintf("%s:%d", suite.BrokerHost, brokerPort)
	suite.broker = sarama.NewBroker(suite.brokerURL)

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_10_1_0

	// connect to the broker
	err := suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	err = suite.createTopic(suite.topic, 1)
	suite.Require().NoError(err, "Failed to create topics")

	// create a sync producer
	suite.producer, err = sarama.NewSyncProducer([]string{suite.brokerURL}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = functionconfig.Trigger{
		Kind: "kafka",
		URL:  suite.brokerURL,
		Attributes: map[string]interface{}{
			"topic":      suite.topic,
			"partitions": []int{0},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.topic: {NumMessages: 3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) TestDealer() {
	/*
		dealerTopic := "dealer-topic"

		// init a create topic request
		createTopicsRequest := sarama.CreateTopicsRequest{
			TopicDetails: map[string]*sarama.TopicDetail{
				dealerTopic: {
					NumPartitions: 10,
				},
			},
		}

		// create topic
		_, err := suite.broker.CreateTopics(&createTopicsRequest)
		suite.Require().NoError(err, "Failed to create topics")

		createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
		createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
		createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = functionconfig.Trigger{
			Kind: "kafka",
			URL:  suite.brokerURL,
			Attributes: map[string]interface{}{
				"topic":      dealerTopic,
				"partitions": []int{0, 1},
			},
		}

	*/
	onAfterContainerRun := func(deployResult *platform.CreateFunctionResult) bool {
		return true
	}
	fmt.Println("%v\n", onAfterContainerRun)

	//	suite.DeployFunction(createFunctionOptions, onAfterContainerRun)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "spotify/kafka", &dockerclient.RunOptions{
		Ports: map[int]int{
			2181:       2181,
			brokerPort: brokerPort,
		},
		Env: map[string]string{
			"ADVERTISED_HOST": suite.BrokerHost,
			"ADVERTISED_PORT": fmt.Sprintf("%d", brokerPort),
		},
	}
}

// WaitForBroker waits until the broker is ready
func (suite *testSuite) WaitForBroker() error {
	var err error
	timeout := 10 * time.Second

	for start := time.Now(); time.Since(start) <= timeout; time.Sleep(100 * time.Millisecond) {
		_, err = sarama.NewConsumer([]string{suite.brokerURL}, nil)
		if err == nil {
			return nil
		}
	}

	return err
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

func (suite *testSuite) createTopic(topic string, numPartitions int32) error {
	createTopicsRequest := &sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			topic: {
				NumPartitions: numPartitions,
			},
		},
	}

	_, err := suite.broker.CreateTopics(createTopicsRequest)
	return err
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
