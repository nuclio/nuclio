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

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/Shopify/sarama"
)

// KafkaTestSuite is a test suite using Kafka
type KafkaTestSuite struct {
	*triggertest.AbstractBrokerSuite
	Broker        *sarama.Broker
	Producer      sarama.SyncProducer
	Topic         string
	NumPartitions int32
}

// NewKafkaTestSuite returns a new KafkaTestSuite
func NewKafkaTestSuite(topic string, numPartitions int32, brokerSuite triggertest.BrokerSuite) *KafkaTestSuite {
	newTestSuite := &KafkaTestSuite{
		Topic:         topic,
		NumPartitions: numPartitions,
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(brokerSuite)

	return newTestSuite
}

// SetupSuite is called by the test suite
func (suite *KafkaTestSuite) SetupSuite() {
	suite.AbstractBrokerSuite.SetupSuite()

	suite.Logger.Info("Creating broker resources")

	// create broker
	suite.Broker = sarama.NewBroker(fmt.Sprintf("%s:9092", suite.BrokerHost))

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_10_1_0

	// connect to the broker
	err := suite.Broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// init a create topic request
	createTopicsRequest := sarama.CreateTopicsRequest{}
	createTopicsRequest.TopicDetails = map[string]*sarama.TopicDetail{
		suite.Topic: {
			NumPartitions: suite.NumPartitions,
		},
	}

	// create topic
	_, err = suite.Broker.CreateTopics(&createTopicsRequest)
	suite.Require().NoError(err, "Failed to create topic")

	// create a sync producer
	suite.Producer, err = sarama.NewSyncProducer([]string{fmt.Sprintf("%s:9092", suite.BrokerHost)}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "spotify/kafka", &dockerclient.RunOptions{
		Ports: map[int]int{2181: 2181, 9092: 9092},
		Env:   map[string]string{"ADVERTISED_HOST": suite.BrokerHost, "ADVERTISED_PORT": "9092"},
	}
}

// PublishMessageToTopic publishes a message to a topic
func (suite *testSuite) PublishMessageToTopic(topic string, body string) error {
	producerMessage := sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder("key"),
		Value: sarama.StringEncoder(body),
	}

	suite.Logger.InfoWith("Producing")

	partition, offset, err := suite.Producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

	return nil
}
