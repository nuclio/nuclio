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
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"
	"github.com/nuclio/nuclio/pkg/processor/webadmin/dealer"

	"github.com/Shopify/sarama"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

const (
	brokerPort    = 9092
	triggerName   = "my-kafka"
	stoppedTaskID = 1
	newTaskID     = 3
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
	dealerTopic := xid.New().String()

	// We can't create a topic with partitions using sarama
	// See https://github.com/Shopify/sarama/issues/1048
	execOptions := &dockerclient.ExecuteOptions{
		Command: strings.Join([]string{
			"/opt/kafka_2.11-0.10.1.0/bin/kafka-topics.sh",
			"--create", "--topic", dealerTopic,
			"--zookeeper", "localhost:2181",
			"--partitions", "7",
			"--replication-factor", "1",
		}, " "),
	}

	err := suite.DockerClient.ExecuteInContainer(suite.ContainerID, execOptions)
	require.NoError(err, "Can't create dealer topic")

	createFunctionOptions := suite.functionOptions(dealerTopic, []int{0, 1})
	onAfterContainerRun := func(deployResult *platform.CreateFunctionResult) bool {
		go suite.spammer(dealerTopic)

		dealerURL := "http://localhost:8081/triggers"
		containerID := deployResult.ContainerID
		dealerReply := suite.callDealer(containerID, dealerURL, "")

		trigger, ok := dealerReply.Triggers[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)
		require.Equal(2, len(trigger.Tasks), "Wrong number of tasks/partitions")

		requestFilePath, err := suite.createDealerRequest()
		require.NoError(err, "Can't creat dealer request file")

		copyOptions := &dockerclient.CopyOptions{
			SourcePath:      requestFilePath,
			DestinationPath: "/",
		}

		err = suite.DockerClient.CopyToContainer(containerID, copyOptions)
		require.NoError(err, "Can't copy to container")

		dealerReply = suite.callDealer(containerID, dealerURL, path.Base(requestFilePath))
		trigger, ok = dealerReply.Triggers[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)

		// Make sure we got also deleted tasks in the reply
		require.Equal(3, len(trigger.Tasks), "Bad number of tasks")
		trigger, ok = dealerReply.Triggers[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)
		stoppedTask := suite.findTask(stoppedTaskID, trigger.Tasks)
		require.NotNilf(stoppedTask, "Can't find task %v in %v", stoppedTaskID, trigger.Tasks)
		require.Equalf(dealer.TaskStateDeleted, stoppedTask.State, "Bad task state: %s", stoppedTask.State)

		newTask := suite.findTask(newTaskID, trigger.Tasks)
		require.NotNilf(newTask, "Can't find task %v in %v", newTaskID, trigger.Tasks)
		oldCheckpoint := newTask.Checkpoint

		// Give processor some time to ingest messages
		timeout := 3 * time.Second
		time.Sleep(timeout)

		dealerReply = suite.callDealer(containerID, dealerURL, "")
		trigger, ok = dealerReply.Triggers[triggerName]
		require.Truef(ok, "Can't find trigger %s in %+v", triggerName, dealerReply)
		newTask = suite.findTask(newTaskID, trigger.Tasks)
		require.NotNilf(newTask, "Can't find task %v in %v", newTaskID, trigger.Tasks)
		require.NotEqualf(oldCheckpoint, newTask.Checkpoint, "No new messages after %s", timeout)

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

func (suite *testSuite) spammer(topic string) {
	suite.Logger.InfoWith("Starting spamming kafka", "topic", topic)
	for messageID := 0; ; messageID++ {
		value := fmt.Sprintf("Spam message #%d", messageID)
		producerMessage := sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(value),
		}

		_, _, err := suite.producer.SendMessage(&producerMessage)
		if err != nil {
			suite.Logger.ErrorWith("Can't send message to kafka", "topic", topic, "error", err)
		}

		if (messageID+1)%50 == 0 {
			suite.Logger.InfoWith("Spam kafka checkpoint", "topic", topic, "count", messageID+1)
		}

		time.Sleep(10 * time.Millisecond)
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
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{
		"apk --update --no-cache add curl",
	}

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

func (suite *testSuite) createDealerRequest() (string, error) {
	requestData := dealer.Message{
		Name:        "archer",
		Namespace:   "space",
		Function:    "kafka test handler",
		Version:     "0",
		IP:          "172.31.25.104",
		Port:        8081,
		State:       dealer.ProcessStateReady,
		TotalEvents: 0,
		Timestamp:   time.Now(),
		DealerURL:   "172.31.25.104",

		Triggers: map[string]*dealer.Trigger{
			triggerName: &dealer.Trigger{
				TotalTasks: 2,
				Disable:    false,
				Tasks: []dealer.Task{
					dealer.Task{
						ID:    0,
						State: dealer.TaskStateRunning,
					},
					dealer.Task{
						ID:    stoppedTaskID,
						State: dealer.TaskStateStopping,
					},
					dealer.Task{
						ID:    newTaskID,
						State: dealer.TaskStateRunning,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", err
	}

	return suite.createTempFile(string(jsonData))
}

func (suite *testSuite) findTask(taskID int, tasks []dealer.Task) *dealer.Task {
	for _, task := range tasks {
		if task.ID == taskID {
			return &task
		}
	}

	return nil
}

func (suite *testSuite) callDealer(containerID string, dealerURL string, requestFilePath string) *dealer.Message {

	fileSwitch := ""
	if requestFilePath != "" {
		fileSwitch = fmt.Sprintf("-d@%s", requestFilePath)
	}

	var stdOut string
	execOptions := &dockerclient.ExecuteOptions{
		Command: fmt.Sprintf("curl -sf %s %s", fileSwitch, dealerURL),
		Stdout:  &stdOut,
	}

	err := suite.DockerClient.ExecuteInContainer(containerID, execOptions)
	suite.Require().NoError(err, "Can't call dealer API")

	dealerReply := &dealer.Message{}
	err = json.Unmarshal([]byte(stdOut), dealerReply)
	suite.Require().NoErrorf(err, "Can't decode dealer reply:\n%q", stdOut)

	return dealerReply
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
