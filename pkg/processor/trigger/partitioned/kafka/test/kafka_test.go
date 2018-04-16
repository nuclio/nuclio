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
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/suite"
)

const (
	brokerPort  = 9092
	triggerName = "my-kafka"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	broker    *sarama.Broker
	brokerURL string
	producer  sarama.SyncProducer
	topic     string
}

type TriggerInfo struct {
	Tasks []struct {
		ID string `json:"id"`
	}
}

var dealerRequestData = `
{
  "name": "fn1-0003",
  "namespace": "default",
  "function": "fn1",
  "version": "latest",
  "alias": "latest",
  "ip": "",
  "port": 0,
  "state": 1,
  "lastEvent": "0001-01-01T00:00:00Z",
  "jobs": {
    "franz": {
      "totalTasks": 5,
      "tasks": [
        {
          "id": 4,
          "state": 4
        }
      ]
    }
  }
}
`

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
	createFunctionOptions := suite.functionOptions(suite.topic, []int{0})

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
	require := suite.Require()
	dealerTopic := "dealer-topic"

	// We can create a topic with partitions using sarama
	// See https://github.com/Shopify/sarama/issues/1048
	command := strings.Join([]string{
		"/opt/kafka_2.11-0.10.1.0/bin/kafka-topics.sh",
		"--create", "--topic", dealerTopic,
		"--zookeeper", "localhost:2181",
		"--partitions", "7",
		"--replication-factor", "1",
	}, " ")
	_, err := suite.DockerClient.ExecuteInContainer(suite.ContainerID, command)
	require.NoError(err, "Can't create dealer topic")

	createFunctionOptions := suite.functionOptions(dealerTopic, []int{0, 1})
	onAfterContainerRun := func(deployResult *platform.CreateFunctionResult) bool {
		dealerURL := "http://localhost:8081/dealer"
		containerID := deployResult.ContainerID

		callCommand := fmt.Sprintf("curl %s", dealerURL)
		out, err := suite.DockerClient.ExecuteInContainer(containerID, callCommand)
		require.NoError(err, "Can't call dealer API")

		dealerReply := make(map[string]TriggerInfo)
		err = json.Unmarshal([]byte(out), &dealerReply)
		require.NoError(err, "Bad response from dealer")

		trigger, ok := dealerReply[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)
		require.Equal(2, len(trigger.Tasks), "Wrong number of tasks/partitions")

		requestFilePath, err := suite.createTempFile(dealerRequestData)
		require.NoError(err, "Can't creat temporary file")

		err = suite.DockerClient.CopyToContainer(containerID, requestFilePath, "/")
		require.NoError(err, "Can't copy to container")

		cmd := fmt.Sprintf("curl -d@/%s %s", path.Base(requestFilePath), dealerURL)
		_, err = suite.DockerClient.ExecuteInContainer(containerID, cmd)
		require.NoError(err, "Can't call dealer API")

		out, err = suite.DockerClient.ExecuteInContainer(containerID, callCommand)
		require.NoError(err, "Can't call dealer API")
		err = json.Unmarshal([]byte(out), &dealerReply)
		require.NoError(err, "Bad response from dealer")

		trigger, ok = dealerReply[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)
		require.Equal(1, len(trigger.Tasks), "Wrong number of tasks/partitions")

		return true
	}

	suite.DeployFunction(createFunctionOptions, onAfterContainerRun)
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

func (suite *testSuite) createTempFile(content string) (string, error) {
	file, err := ioutil.TempFile("", "kafka-test")
	if err != nil {
		return "", err
	}

	_, err = file.Write([]byte(content))
	if err != nil {
		return "", err
	}

	return file.Name(), file.Close()

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

func (suite *testSuite) functionOptions(topic string, partitions []int) *platform.CreateFunctionOptions {

	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		triggerName: functionconfig.Trigger{
			Kind:       "kafka",
			URL:        suite.brokerURL,
			Partitions: suite.createConfigPartitions(partitions),
			Attributes: map[string]interface{}{
				"topic": topic,
			},
		},
	}

	return createFunctionOptions
}

func (suite *testSuite) createConfigPartitions(partitions []int) []functionconfig.Partition {
	configPartitions := make([]functionconfig.Partition, len(partitions))
	for i, partitionID := range partitions {
		configPartitions[i] = functionconfig.Partition{
			ID: fmt.Sprintf("%d", partitionID),
		}
	}

	return configPartitions
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
