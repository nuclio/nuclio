//go:build test_integration && test_local && test_broken

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
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/suite"
)

type testConfluentSuite struct {
	*triggertest.AbstractBrokerSuite

	// kafka clients
	broker   *sarama.Broker
	producer sarama.SyncProducer

	// messaging
	topic         string
	consumerGroup string
	numPartitions int32
	user          string
	password      string
	brokerURL     string
}

func (suite *testConfluentSuite) SetupSuite() {
	var err error

	// messaging
	suite.topic = "nuclio-test"
	suite.consumerGroup = "myConsumerGroup"
	suite.numPartitions = 8

	suite.brokerURL = "<REPLACE-ME>"
	suite.user = "<REPLACE-ME>"
	suite.password = "<REPLACE=ME>"

	// start broker and zookeeper containers explicitly
	suite.AbstractBrokerSuite.SkipStartBrokerContainer = true
	suite.AbstractBrokerSuite.SetupSuite()

	// create broker
	suite.broker = sarama.NewBroker(suite.brokerURL)
	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V2_4_0_0

	// sasl
	brokerConfig.Net.SASL.Enable = true
	brokerConfig.Net.SASL.User = suite.user
	brokerConfig.Net.SASL.Password = suite.password
	brokerConfig.Net.SASL.Handshake = true
	brokerConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext

	// tls
	brokerConfig.Net.TLS.Enable = true

	// connect to the broker
	err = suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// TODO: this does not seems to work, find a way to create a topic programmatically
	// alternatively, create a topic here: https://confluent.cloud/environments/<env-id>/clusters/<cluster-id>/topics

	//createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
	//	Version: 2,
	//	TopicDetails: map[string]*sarama.TopicDetail{
	//		suite.topic: {
	//			NumPartitions:     suite.NumPartitions,
	//			ReplicationFactor: 1,
	//		},
	//	},
	//})
	//suite.Require().NoError(err, "Failed to create topics broker")
	//suite.Logger.InfoWith("Created topic",
	//	"topic", suite.topic,
	//	"createTopicResponse", createTopicsResponse)

	// create a sync producer
	brokerConfig.Producer.Return.Successes = true
	suite.producer, err = sarama.NewSyncProducer([]string{suite.brokerURL}, brokerConfig)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testConfluentSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"confluent-kafka": {
			Kind:       "kafka-cluster",
			URL:        suite.brokerURL,
			MaxWorkers: 4,
			Attributes: map[string]interface{}{
				"topics":        []string{suite.topic},
				"consumerGroup": suite.consumerGroup,
				"sasl": map[string]interface{}{
					"enable":    true,
					"handshake": true,
					"user":      suite.user,
					"password":  suite.password,
					"mechanism": sarama.SASLTypePlaintext,
				},
				"tls": map[string]interface{}{
					"enable": true,
				},
			},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.topic: {
				NumMessages: int(suite.numPartitions),
			},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testConfluentSuite) publishMessageToTopic(topic string, body string) error {
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

func TestIntegrationConfluentSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	t.Skip("This test requires confluent kafka broker set up")

	testSuiteInstance := &testConfluentSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
