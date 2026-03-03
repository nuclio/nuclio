//go:build test_integration && test_local

/*
Copyright 2023 The Nuclio Authors.

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
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/IBM/sarama"
	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	httpClient *http.Client

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
	zooKeeperConnect       string // "ip:2181" so Kafka does not rely on Docker DNS (flaky on GH runners)
	brokerTriggerURL       string // Kafka broker URL for trigger (ip:9090), avoids Docker DNS for processor containers (flaky on GH runners)

	// for cleanup
	zooKeeperContainerID string
}

type kafkaDeployOptionsConfig struct {
	topic         string
	consumerGroup string

	functionName string
	functionPath string
	runtime      string

	explicitAck          functionconfig.ExplicitAckMode
	workerAllocationMode partitionworker.AllocationMode

	numWorkers               int
	workerTerminationTimeout string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// create http client
	suite.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

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
	suite.SkipStartBrokerContainer = true
	suite.BrokerContainerNetworkName = "nuclio-kafka-test"
	suite.AbstractBrokerSuite.SetupSuite()

	// start zoo keeper container
	suite.zooKeeperContainerID = suite.RunContainer(suite.getKafkaZooKeeperContainerRunInfo())

	// start kafka broker container
	suite.startContainers()

	// create broker
	suite.broker = sarama.NewBroker(suite.brokerURL)

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_11_0_2

	// change partitioner , so we can specify which partition to send on
	brokerConfig.Producer.Partitioner = sarama.NewManualPartitioner

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

func (suite *testSuite) WaitForBroker() error {
	expectedLogSubstring := "started (kafka.server.KafkaServer)"
	var containerLogs string
	var containerLogsErr error
	var brokerExitedErr error

	err := common.RetryUntilSuccessful(120*time.Second, 3*time.Second, func() bool {
		// fetch Kafka container logs
		containerLogs, containerLogsErr = suite.DockerClient.GetContainerLogs(suite.brokerContainerName)
		if containerLogsErr != nil {
			suite.Logger.WarnWith("Failed to get Kafka container logs", "err", containerLogsErr)
			return false
		}

		// check if container exited (broker crashed); capture error so caller can retry (e.g. on flaky GH runner)
		containers, containersErr := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
			Name:    suite.brokerContainerName,
			Stopped: true,
		})
		if containersErr == nil && len(containers) == 1 && containers[0].State != nil && containers[0].State.Status == "exited" {
			state := containers[0].State
			exitCode := 0
			if state.ExitCode != 0 {
				exitCode = state.ExitCode
			}
			zookeeperContainerLogs := suite.getZookeeperContainerLogs()
			suite.Logger.WarnWith("Kafka broker container exited (will retry if attempt 1)",
				"exitCode", exitCode,
				"oomKilled", state.OOMKilled,
				"error", state.Error,
				"containerLogs", containerLogs,
				"zookeeperLogs", zookeeperContainerLogs)
			brokerExitedErr = errors.Errorf("Kafka broker container exited unexpectedly (exit code %d). Kafka logs:\n%s\nZookeeper logs:\n%s",
				exitCode, containerLogs, zookeeperContainerLogs)
			return false
		}

		// check for broker startup confirmation line
		if strings.Contains(containerLogs, expectedLogSubstring) {
			suite.Logger.DebugWith("Kafka broker started successfully", "logsSnippet", expectedLogSubstring)
			return true
		}

		return false
	})

	if brokerExitedErr != nil {
		return brokerExitedErr
	}
	if err != nil {
		zookeeperContainerLogs := suite.getZookeeperContainerLogs()
		suite.Require().NoError(err, fmt.Sprintf("Kafka broker did not start within the given timeframe. Zookeeper logs:\n%s", zookeeperContainerLogs))
	}
	return nil
}

func (suite *testSuite) TestReceiveRecords() {
	// we don't reset it every test, because all tests use the same topic to write to
	expectedNumberOfCommittedMessages := 0
	for _, testCase := range []struct {
		name         string
		functionPath string
		runtime      string
		dependencies []string
		handler      string
	}{
		{
			name:         "python-runtime",
			functionPath: suite.FunctionPaths["python"],
			runtime:      "python",
		},
		{
			name:         "golang-runtime",
			functionPath: suite.FunctionPaths["golang"],
			runtime:      "golang",
		},
		{
			name:         "java-runtime",
			functionPath: suite.FunctionPaths["java"],
			runtime:      "java",
			dependencies: []string{"group: org.json, name: json, version: 20210307"},
			handler:      "Handler",
		},
	} {
		suite.Run(testCase.name, func() {
			functionName := "event_recorder"
			createFunctionOptions := suite.GetDeployOptions(functionName, testCase.functionPath)
			createFunctionOptions.FunctionConfig.Spec.Runtime = testCase.runtime
			if testCase.handler != "" {
				createFunctionOptions.FunctionConfig.Spec.Handler = testCase.handler
			}
			createFunctionOptions.FunctionConfig.Spec.Build.Dependencies = testCase.dependencies
			createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
				Attributes: map[string]interface{}{
					"network": suite.BrokerContainerNetworkName,
				},
			}

			createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
				"my-kafka": {
					Kind: "kafka-cluster",
					URL:  suite.brokerTriggerURL,
					Attributes: map[string]interface{}{
						"topics":        []string{suite.topic},
						"consumerGroup": functionName,
						"initialOffset": suite.initialOffset,
					},
					WorkerTerminationTimeout: "5s",
				},
			}

			expectedNumberOfCommittedMessages += int(suite.NumPartitions)

			triggertest.InvokeEventRecorder(&suite.TestSuite,
				suite.BrokerHost,
				createFunctionOptions,
				map[string]triggertest.TopicMessages{
					suite.topic: {
						NumMessages:         int(suite.NumPartitions),
						CustomMessagePrefix: testCase.runtime,
					},
				},
				nil,
				suite.publishMessageToTopic,
				&triggertest.PostPublishChecks{
					ValidateAckFunction:              suite.validateNumberOfCommittedOffsets,
					ExpectedNumberOfCommittedOffsets: expectedNumberOfCommittedMessages,
					ConsumerGroup:                    functionName,
				})
		})
	}
}

func (suite *testSuite) TestExplicitAck() {
	topic := "myNewTopic"
	shardID := int32(1)
	functionName := "explicitacker"
	functionPath := path.Join(suite.GetTestFunctionsDir(),
		"python",
		"kafka-explicit-ack",
		"explicitacker.py")

	// create new topic
	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			topic: {
				NumPartitions:     suite.NumPartitions,
				ReplicationFactor: 1,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", topic,
		"createTopicResponse", createTopicsResponse)

	for _, testCase := range []struct {
		name            string
		explicitAckMode functionconfig.ExplicitAckMode
	}{
		{
			name:            "EnableMode",
			explicitAckMode: functionconfig.ExplicitAckModeEnable,
		},
		{
			name:            "ExplicitOnly",
			explicitAckMode: functionconfig.ExplicitAckModeExplicitOnly,
		},
	} {
		suite.Run(testCase.name, func() {

			// create explicit ack function
			createFunctionOptions := suite.GetDeployOptions(functionName, functionPath)
			createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install nuclio-sdk"}
			createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
				Attributes: map[string]interface{}{
					"network": suite.BrokerContainerNetworkName,
				},
			}

			// configure kafka trigger with explicit ack enabled
			createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
				"my-kafka": {
					Kind: "kafka-cluster",
					URL:  suite.brokerTriggerURL,
					Attributes: map[string]interface{}{
						"topics":               []string{topic},
						"consumerGroup":        functionName,
						"initialOffset":        suite.initialOffset,
						"workerAllocationMode": string(partitionworker.AllocationModeStatic),
					},
					WorkerTerminationTimeout: "5s",
					ExplicitAckMode:          testCase.explicitAckMode,
				},
				"my-http": {
					Kind:       "http",
					Attributes: map[string]interface{}{},
				},
			}

			// deploy function
			suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
				suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

				var err error

				// publish 10 messages to the topic
				for i := 0; i < 10; i++ {
					err = suite.publishMessageToTopicOnSpecificShard(topic, fmt.Sprintf("message-%d", i), shardID)
					suite.Require().NoError(err, "Failed to publish message")
				}

				// ensure queue size is 10
				suite.Logger.Debug("Getting current queue size")
				queueSize := suite.waitForFunctionQueueSize(deployResult.Port, 10, 5*time.Second)
				suite.Require().Equal(queueSize, 10, "Queue size is not 10")

				// ensure commit offset is 0
				suite.Logger.Debug("Getting commit offset before processing")
				commitOffset := suite.getLastCommitOffsetFromFunction(deployResult.Port)
				suite.Require().Equal(commitOffset, 0, "Commit offset is not 0")

				// send http request "start processing"
				suite.Logger.Debug("Sending start processing request")
				body := map[string]string{
					"resource": "start_processing",
				}

				marshalledBody, err := json.Marshal(body)
				suite.Require().NoError(err, "Failed to marshal body")
				response, err := suite.SendHTTPRequest(&triggertest.Request{
					Method: http.MethodPost,
					Port:   deployResult.Port,
					Body:   string(marshalledBody),
				})
				suite.Require().NoError(err, "Failed to send request")
				suite.Require().Equal(http.StatusOK, response.StatusCode)

				time.Sleep(2 * time.Second)

				// ensure queue size is 0 (or < 10)
				suite.Logger.Debug("Getting queue size after processing")
				queueSize = suite.waitForFunctionQueueSize(deployResult.Port, 0, 5*time.Second)
				suite.Require().Equal(queueSize, 0, "Queue size is not 0")

				// ensure commit offset is 9 (10 in zero-indexed offsets)
				suite.Logger.Debug("Getting commit offset after processing")
				commitOffset = suite.getLastCommitOffsetFromFunction(deployResult.Port)
				suite.Require().Equal(commitOffset, 9, "Commit offset is not 10")

				return true
			})
		})
	}
}

func (suite *testSuite) TestDrainHook() {
	topic := "myTopicDraining"
	defer func() {
		// delete topic
		_, err := suite.broker.DeleteTopics(&sarama.DeleteTopicsRequest{
			Topics: []string{topic},
		})
		suite.Require().NoError(err, "Failed to delete topic")
	}()
	functionName := "drain-hook"
	functionPath := path.Join(suite.GetTestFunctionsDir(),
		"python",
		"drain-hook",
		"drain-hook.py")

	// create new topic
	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			topic: {
				NumPartitions:     suite.NumPartitions,
				ReplicationFactor: 1,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", topic,
		"createTopicResponse", createTopicsResponse)

	createFunctionWithKafkaConfig := &kafkaDeployOptionsConfig{
		functionName:             functionName,
		functionPath:             functionPath,
		runtime:                  "python",
		topic:                    topic,
		consumerGroup:            suite.consumerGroup,
		workerAllocationMode:     partitionworker.AllocationModeStatic,
		numWorkers:               4,
		workerTerminationTimeout: "40s",
	}
	createFunctionOptions := suite.getDeployOptionsWithKafka(createFunctionWithKafkaConfig)

	// create a temp dir, delete it after the test
	tempDir, err := os.MkdirTemp("", "drain-hook")
	suite.Require().NoError(err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir) // nolint: errcheck

	mountPath := "/tmp/nuclio"
	directoryType := v1.HostPathDirectory

	// mount it to the function
	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{
		{
			Volume: v1.Volume{
				Name: "drain-hook",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: tempDir,
						Type: &directoryType,
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "drain-hook",
				ReadOnly:  false,
				MountPath: mountPath,
			},
		},
	}

	var rebalanceStartedTime time.Time

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

		// write messages on 4 shards
		for partitionIdx := int32(0); partitionIdx < suite.NumPartitions; partitionIdx++ {
			messageBody := fmt.Sprintf("%s-%d", "messagingCycleA", partitionIdx)

			// send the message
			err := suite.publishMessageToTopicOnSpecificShard(topic, messageBody, partitionIdx)
			suite.Require().NoError(err, "Failed to publish message")
		}

		// create another function that consumes from the same topic and consumer group, to trigger rebalance
		createFunctionWithKafkaConfig.functionName = "drain-hook-new"
		newCreateFunctionOptions := suite.getDeployOptionsWithKafka(createFunctionWithKafkaConfig)

		suite.Logger.Debug("Creating second function, to trigger rebalance")

		suite.DeployFunction(newCreateFunctionOptions, func(newDeployResult *platform.CreateFunctionResult) bool {
			suite.Require().NotNil(newDeployResult, "Unexpected empty second deploy results")
			rebalanceStartedTime = time.Now()

			suite.Logger.DebugWith("Created second function, producing messages to topic",
				"topic", topic)

			// write messages to all 4 shards
			for partitionIdx := int32(0); partitionIdx < suite.NumPartitions; partitionIdx++ {
				messageBody := fmt.Sprintf("%s-%d", "messagingCycleB", partitionIdx)

				// send the message
				err := suite.publishMessageToTopicOnSpecificShard(topic, messageBody, partitionIdx)
				suite.Require().NoError(err, "Failed to publish message")
			}

			// wait for at least the kafka trigger's WorkerTerminationTimeout to pass from first invocation,
			// to allow the function to run its termination hook before we delete the function
			<-time.After(time.Until(rebalanceStartedTime.Add(40 * time.Second)))

			return true
		})

		return true
	})

	// check that the function's drain hook was called by reading the file it should have written to
	// 1 file per worker -> 4 files
	for workerID := 0; workerID < 4; workerID++ {
		filePath := path.Join(tempDir, fmt.Sprintf("drain-hook-%d.txt", workerID))
		suite.Logger.DebugWith("Reading drain hook file", "filePath", filePath)
		fileBytes, err := os.ReadFile(filePath)
		suite.Require().NoError(err, "Failed to read drain hook file")

		// check that the file is not empty
		suite.Logger.DebugWith("Checking drain hook file is not empty", "fileContent", string(fileBytes))
		suite.Require().NotEmpty(fileBytes, "Drain hook file is empty")
	}
}

// TestFeatureCombinations tests different combinations of the following features:
// * ExplicitAck
// * WorkerAllocationMode
// * Draining callback
// * Termination callback
func (suite *testSuite) TestFeatureCombinations() {
	for _, testCase := range []struct {
		name                 string
		explicitAckMode      functionconfig.ExplicitAckMode
		workerAllocationMode partitionworker.AllocationMode
		partitionNum         int
		cycles               int
		messagesPerCycle     int
	}{
		{
			name:                 "noExplicitAck-16-pool",
			explicitAckMode:      functionconfig.ExplicitAckModeDisable,
			workerAllocationMode: partitionworker.AllocationModePool,
			partitionNum:         16,
			cycles:               5,
			messagesPerCycle:     5,
		},
		{
			name:                 "noExplicitAck-16-static",
			explicitAckMode:      functionconfig.ExplicitAckModeDisable,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			partitionNum:         16,
			cycles:               5,
			messagesPerCycle:     5,
		},
		{
			name:                 "ExplicitOnly-16",
			explicitAckMode:      functionconfig.ExplicitAckModeExplicitOnly,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			partitionNum:         16,
			cycles:               5,
			messagesPerCycle:     5,
		},
		{
			name:                 "ExplicitOnly-64",
			explicitAckMode:      functionconfig.ExplicitAckModeExplicitOnly,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			partitionNum:         64,
			cycles:               2,
			messagesPerCycle:     1,
		},
		{
			name:                 "ExplicitAckEnable-16",
			explicitAckMode:      functionconfig.ExplicitAckModeEnable,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			partitionNum:         16,
			cycles:               5,
			messagesPerCycle:     5,
		},
	} {
		suite.Run(testCase.name, func() {
			// create shared temp file for all functions
			tmpDir := suite.T().TempDir()
			sharedFilePath := filepath.Join(tmpDir, "shared.log")
			f, err := os.Create(sharedFilePath)
			suite.Require().NoError(err, "Failed to create shared log file")
			err = f.Close()
			suite.Require().NoError(err, "Failed to close shared file")

			// remove the file at the end of the test
			// since the file can be quite big, do not rely on auto-deletion of temp dir
			defer os.Remove(sharedFilePath)

			// create topic with many partitions to amplify rebalancing behaviour
			topic := "rebalance-many" + testCase.name
			consumerGroup := "rebalance-many-group"

			// create topic
			createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
				TopicDetails: map[string]*sarama.TopicDetail{
					topic: {
						NumPartitions:     int32(testCase.partitionNum),
						ReplicationFactor: 1,
					},
				},
			})
			suite.Require().NoError(err, "Failed to create topic")
			suite.Logger.InfoWith("Created topic",
				"topic", topic,
				"createTopicResponse", createTopicsResponse)

			// shared hostPath volume spec
			sharedVolume := functionconfig.Volume{
				Volume: v1.Volume{
					Name: "shared-log",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: sharedFilePath,
							Type: common.Pointer(v1.HostPathFileOrCreate),
						},
					},
				},
				VolumeMount: v1.VolumeMount{
					Name:      "shared-log",
					MountPath: "/tmp/events.json",
				},
			}
			deployFunctionWithKafkaConfig := &kafkaDeployOptionsConfig{
				functionName:         "rebalance-many-1",
				functionPath:         suite.FunctionPaths["python-streaming-features"],
				runtime:              "python",
				topic:                topic,
				consumerGroup:        consumerGroup,
				explicitAck:          testCase.explicitAckMode,
				workerAllocationMode: testCase.workerAllocationMode,
			}

			createFunctionOptions := suite.getDeployOptionsWithKafka(deployFunctionWithKafkaConfig)
			createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{sharedVolume}

			messageHistory := make([]string, 0)
			// parameters to exercise many rebalances
			cycles := testCase.cycles
			messagesPerPartitionPerCycle := testCase.messagesPerCycle

			produceMessages := func(cycle, round int) {
				for partitionIdx := 0; partitionIdx < testCase.partitionNum; partitionIdx++ {
					for i := 0; i < messagesPerPartitionPerCycle; i++ {
						body := fmt.Sprintf("cycle-%d-msg-%d-part-%d-round-%d", cycle, i, partitionIdx, round)
						err := suite.publishMessageToTopicOnSpecificShard(topic, body, int32(partitionIdx))
						messageHistory = append(messageHistory, body)
						suite.Require().NoError(err, "Failed to publish message")
					}
				}
			}

			suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
				suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

				// wait a bit for the function to start
				// sometimes it takes time for kafka to sync a new consumer group/topic
				// especially with many partitions and when running locally with arm64 simulating amd64
				time.Sleep(5 * time.Second)

				produceMessages(0, 0)
				// ensure they are all read
				err := common.RetryUntilSuccessful(60*time.Second,
					2*time.Second,
					func() bool {
						receivedBodies := suite.resolveReceivedEventBodies(deployResult)
						return len(receivedBodies) == messagesPerPartitionPerCycle*testCase.partitionNum
					})
				suite.Require().NoError(err, "Failed to get initial events")

				// run cycles; each cycle send messages to all partitions and trigger a rebalance by adding a new consumer
				// cycles * messagesPerPartitionPerCycle * numPartitions * 2 is the total number of messages
				for cycle := 1; cycle <= cycles; cycle++ {
					suite.Logger.InfoWith("Starting cycle", "cycle", cycle)
					produceMessages(cycle, 1)

					// trigger a rebalance by adding another consumer in the same group
					// use a unique name each time to avoid clashes and ensure new container
					newFnName := fmt.Sprintf("rebalance-many-extra-%d", cycle)
					deployFunctionWithKafkaConfig.functionName = newFnName
					newCreateFunctionOptions := suite.getDeployOptionsWithKafka(deployFunctionWithKafkaConfig)
					newCreateFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{sharedVolume}

					suite.DeployFunction(newCreateFunctionOptions, func(newDeployResult *platform.CreateFunctionResult) bool {
						suite.Require().NotNil(newDeployResult, "Unexpected empty second deploy results")

						// produce again messages on all partitions, to ensure there is enough work to do after the rebalance
						// this also increases the likelihood that the rebalance completes while there is still work to do
						// for the original function
						produceMessages(cycle, 2)

						err := common.RetryUntilSuccessful(30*time.Second,
							2*time.Second,
							func() bool {
								receivedBodies := suite.resolveReceivedEventBodies(deployResult)
								// wait until we see at least one new message in the original function
								return len(receivedBodies) > 0
							})
						suite.Require().NoError(err, fmt.Sprintf("Failed to get initial events for cycle %d", cycle))

						return true
					})
				}

				// make sure all messages are received by checking the file content
				offsetTarget := len(messageHistory)
				err = common.RetryUntilSuccessful(120*time.Second,
					10*time.Second,
					func() bool {
						numMessages := suite.getLenRecodedEventsAndEnsureNoDuplicates(sharedFilePath)
						return numMessages == offsetTarget
					})
				suite.Require().NoError(err, "Timed out waiting for committed offsets")
				return true
			})

			// validate that all messages were ACKed
			offsetTarget := len(messageHistory)
			err = common.RetryUntilSuccessful(30*time.Second,
				2*time.Second,
				func() bool {
					return suite.validateNumberOfCommittedOffsets(consumerGroup, topic, testCase.partitionNum, offsetTarget)
				})
			suite.Require().NoError(err, "Timed out waiting for committed offsets")

			suite.getLenRecodedEventsAndEnsureNoDuplicates(sharedFilePath)
		})
	}
}

func (suite *testSuite) getDeployOptionsWithKafka(config *kafkaDeployOptionsConfig) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions(config.functionName, config.functionPath)
	createFunctionOptions.FunctionConfig.Spec.Runtime = config.runtime
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{"nuclio.io/kafka-log-level": "10"}
	createFunctionOptions.FunctionConfig.Spec.Platform = suite.getBasePlatformSpec()
	createFunctionOptions.FunctionConfig.Spec.Triggers = suite.getKafkaTriggerSpec(config)
	return createFunctionOptions
}

func (suite *testSuite) startContainers() {
	// Use Zookeeper container IP for Kafka connection so we don't rely on Docker DNS (flaky on GH runners).
	zooKeeperIPs, err := suite.DockerClient.GetContainerIPAddresses(suite.zooKeeperContainerID)
	suite.Require().NoError(err, "Failed to get Zookeeper container IP")
	suite.Require().NotEmpty(zooKeeperIPs, "Zookeeper container has no IP")
	suite.zooKeeperConnect = zooKeeperIPs[0] + ":2181"

	// start broker container; retry once on failure (e.g. transient GH runner issues)
	imageName, runOptions := suite.GetContainerRunInfo()
	var waitErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			if suite.BrokerContainerID != "" {
				_ = suite.DockerClient.RemoveContainer(suite.BrokerContainerID)
				suite.BrokerContainerID = ""
			}
			suite.Logger.InfoWith("Retrying Kafka broker startup after previous failure (e.g. runner glitch)")
			time.Sleep(2 * time.Second)
		}
		suite.BrokerContainerID = suite.RunContainer(imageName, runOptions)
		// DEBUG: temporary – helps diagnose GH runner flakiness
		suite.Logger.InfoWith("Kafka broker start attempt",
			"attempt", attempt,
			"maxAttempts", 2,
			"containerID", suite.BrokerContainerID,
			"containerName", suite.brokerContainerName)
		waitErr = suite.WaitForBroker()
		if waitErr == nil {
			break
		}
		if attempt == 2 {
			suite.Require().NoError(waitErr, "Error waiting for broker to be ready")
		}
	}

	// Use broker container IP for trigger URL so processor containers do not rely on Docker DNS (flaky on GH runners).
	brokerIPs, err := suite.DockerClient.GetContainerIPAddresses(suite.BrokerContainerID)
	suite.Require().NoError(err, "Failed to get broker container IP")
	suite.Require().NotEmpty(brokerIPs, "Broker container has no IP")
	suite.brokerTriggerURL = brokerIPs[0] + ":9090"

	suite.Logger.InfoWith("Creating broker resources",
		"brokerHost", suite.BrokerHost)
}

func (suite *testSuite) getBasePlatformSpec() functionconfig.Platform {
	return functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}
}

func (suite *testSuite) getKafkaTriggerSpec(config *kafkaDeployOptionsConfig) map[string]functionconfig.Trigger {
	kafkaTrigger := functionconfig.Trigger{
		Kind: "kafka-cluster",
		URL:  suite.brokerTriggerURL,
		Attributes: map[string]interface{}{
			"topics":        []string{config.topic},
			"consumerGroup": config.consumerGroup,
			"initialOffset": "latest",
		},
	}

	if config.explicitAck != "" {
		kafkaTrigger.ExplicitAckMode = config.explicitAck
	}
	if config.workerAllocationMode != "" {
		kafkaTrigger.Attributes["workerAllocationMode"] = string(config.workerAllocationMode)
	}
	if config.numWorkers != 0 {
		kafkaTrigger.NumWorkers = config.numWorkers
	}
	if config.workerTerminationTimeout != "" {
		kafkaTrigger.WorkerTerminationTimeout = config.workerTerminationTimeout
	}
	return map[string]functionconfig.Trigger{
		"my-kafka": kafkaTrigger,
	}
}

func (suite *testSuite) getLenRecodedEventsAndEnsureNoDuplicates(filePath string) int {
	content, err := os.ReadFile(filePath)
	suite.Require().NoError(err, "Failed to read file")

	// Regex to capture body values: "body": "<value>"
	re := regexp.MustCompile(`"body"\s*:\s*"([^"]+)"`)

	matches := re.FindAllStringSubmatch(string(content), -1)
	suite.Require().NotNil(matches, "No body values found")

	seen := make(map[string]bool)
	duplicates := make([]string, 0)

	for _, m := range matches {
		body := m[1]
		if seen[body] {
			duplicates = append(duplicates, body)
		} else {
			seen[body] = true
		}
	}

	suite.Require().Len(duplicates, 0, fmt.Sprintf("Found duplicate messages: %v", duplicates))
	return len(matches)
}

// getZookeeperContainerLogs returns Zookeeper container logs for debugging when the broker fails.
// Returns a placeholder string if logs cannot be fetched (e.g. container already removed).
func (suite *testSuite) getZookeeperContainerLogs() string {
	logs, err := suite.DockerClient.GetContainerLogs(suite.zooKeeperContainerName)
	if err != nil {
		return fmt.Sprintf("(failed to get Zookeeper logs: %v)", err)
	}
	if logs == "" {
		return "(Zookeeper container had no logs)"
	}
	return logs
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "gcr.io/iguazio/kafka", &dockerclient.RunOptions{
		ContainerName: suite.brokerContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        false, // keep container on exit so we can capture logs on failure
		Ports: map[int]int{

			// broker
			suite.brokerPort: suite.brokerPort,
		},
		Env: map[string]string{
			// Disable JVM container support to avoid NPE in CgroupV2Subsystem on GH runners (cgroup v2).
			"JAVA_TOOL_OPTIONS":                    "-XX:-UseContainerSupport",
			"KAFKA_ZOOKEEPER_CONNECT":              suite.zooKeeperConnect,
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
	return "gcr.io/iguazio/zookeeper", &dockerclient.RunOptions{
		ContainerName: suite.zooKeeperContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        false, // keep container on exit so we can capture logs on failure
		Ports: map[int]int{
			dockerclient.RunOptionsRandomPort: 2181,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	return suite.publishMessageToTopicOnSpecificShard(topic, body, 0)
}

func (suite *testSuite) publishMessageToTopicOnSpecificShard(topic string, body string, partitionID int32) error {
	producerMessage := sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(fmt.Sprintf("key-%d", partitionID)),
		Value:     sarama.StringEncoder(body),
		Partition: partitionID,
	}

	suite.Logger.InfoWith("Producing", "topic", topic, "body", body)

	partition, offset, err := suite.producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

	return nil
}

func (suite *testSuite) resolveReceivedEventBodies(deployResult *platform.CreateFunctionResult) []string { // nolint: unused

	receivedEvents, err := triggertest.GetEventRecorderReceivedEvents(suite.Logger, suite.BrokerHost, deployResult.Port)
	suite.Require().NoError(err)
	var receivedBodies []string

	// compare only bodies due to a deficiency in CompareNoOrder
	for _, receivedEvent := range receivedEvents {

		// some brokers need data to be able to read the stream. these write "ignore", so we ignore that
		if receivedEvent.Body != "ignore" {
			receivedBodies = append(receivedBodies, receivedEvent.Body)
		}
	}

	return receivedBodies
}

func (suite *testSuite) validateNumberOfCommittedOffsets(consumerGroup string, topic string, numPartitions int, expectedNumberOfCommittedOffsets int) bool {
	if topic == "" {
		topic = suite.topic
	}
	if numPartitions == 0 {
		numPartitions = int(suite.NumPartitions)
	}
	numberOfCommittedOffsets, err := suite.getNumberOfCommittedOffsetsFromBroker(consumerGroup, topic, numPartitions)
	suite.Logger.DebugWith("Number of committed offsets",
		"topic", topic,
		"consumerGroup", consumerGroup,
		"numberOfCommittedOffsets", numberOfCommittedOffsets,
		"expectedNumberOfCommittedOffsets", expectedNumberOfCommittedOffsets)

	if err != nil {
		return false
	}
	return int(numberOfCommittedOffsets) == expectedNumberOfCommittedOffsets
}

func (suite *testSuite) getNumberOfCommittedOffsetsFromBroker(consumerGroup, topic string, partitions int) (int64, error) {
	// Create an OffsetFetchRequest
	request := &sarama.OffsetFetchRequest{
		ConsumerGroup: consumerGroup,
		Version:       3,
	}

	for partition := 0; partition < partitions; partition++ {
		request.AddPartition(topic, int32(partition))
	}

	// Send the request to the broker
	response, err := suite.broker.FetchOffset(request)
	if err != nil {
		return -1, errors.Errorf("failed to fetch offsets: %s", err.Error())
	}

	// Sum committed offsets across all partitions
	var totalOffset int64 = 0
	for partition := 0; partition < partitions; partition++ {
		block := response.GetBlock(topic, int32(partition))
		if block == nil {
			return -1, errors.Errorf("No offset block returned for topic %s partition %d", topic, partition)
		}
		if block.Err != sarama.ErrNoError {
			return -1, errors.Errorf("Error in offset block for partition %d: %v", partition, block.Err)
		}
		if block.Offset != -1 {
			totalOffset += block.Offset
		}
	}

	return totalOffset, nil
}

func (suite *testSuite) getLastCommitOffsetFromFunction(port int) int {
	body := map[string]string{
		"resource": "last_committed_offset",
	}

	marshalledBody, err := json.Marshal(body)
	suite.Require().NoError(err, "Failed to marshal body")

	httpRequest := &triggertest.Request{
		Method: http.MethodGet,
		Port:   port,
		Body:   string(marshalledBody),
	}
	response, err := suite.SendHTTPRequest(httpRequest)
	suite.Require().NoError(err, "Failed to send request")
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	responseBodyBytes, err := io.ReadAll(response.Body)
	suite.Require().NoError(err, "Failed to read response body")
	suite.Logger.DebugWith("Got response", "response", response, "responseBody", string(responseBodyBytes))

	responseBody := map[string]interface{}{}
	err = json.Unmarshal(responseBodyBytes, &responseBody)
	suite.Require().NoError(err, "Failed to unmarshal response body")

	lastCommittedOffset, exists := responseBody["last_committed_offset"]
	suite.Require().True(exists, "Failed to find last committed offset")

	lastCommittedOffsetString, ok := lastCommittedOffset.(string)
	suite.Require().True(ok, "Failed to convert last committed offset to string")

	lastCommittedOffsetInt, err := strconv.Atoi(lastCommittedOffsetString)
	suite.Require().NoError(err, "Failed to convert last committed offset to int")

	return lastCommittedOffsetInt
}

func (suite *testSuite) getQueueSize(port int) int {
	body := map[string]string{
		"resource": "queue_size",
	}

	marshalledBody, err := json.Marshal(body)
	suite.Require().NoError(err, "Failed to marshal body")

	httpRequest := &triggertest.Request{
		Method: http.MethodGet,
		Port:   port,
		Body:   string(marshalledBody),
	}
	response, err := suite.SendHTTPRequest(httpRequest)
	suite.Require().NoError(err, "Failed to send request")
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	responseBodyBytes, err := io.ReadAll(response.Body)
	suite.Require().NoError(err, "Failed to read response body")
	suite.Logger.DebugWith("Got response", "response", response, "responseBody", string(responseBodyBytes))

	responseBody := map[string]interface{}{}
	err = json.Unmarshal(responseBodyBytes, &responseBody)
	suite.Require().NoError(err, "Failed to unmarshal response body")

	queueSize, exists := responseBody["queue_size"]
	suite.Require().True(exists, "Failed to find queue size")

	queueSizeFloat, ok := queueSize.(float64)
	suite.Require().True(ok, "Failed to convert queue size to int")

	return int(queueSizeFloat)
}

func (suite *testSuite) waitForFunctionQueueSize(port, expectedQueueSize int, timeout time.Duration) int {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-timeoutTimer.C:
			suite.Fail("Timeout waiting for queue size")
		default:
			queueSize := suite.getQueueSize(port)
			if queueSize == expectedQueueSize {
				return queueSize
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	testSuiteInstance := &testSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
