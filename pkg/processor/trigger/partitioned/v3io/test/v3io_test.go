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
	"os"
	"strconv"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"
	"github.com/nuclio/nuclio/pkg/processor/util/v3io"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	v3iohttp "github.com/v3io/v3io-go-http"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	container      *v3iohttp.Container
	address        string
	containerAlias string
	streamPath     string
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		address:        os.Getenv("NUCLIO_V3IO_TEST_ADDRESS"),
		containerAlias: os.Getenv("NUCLIO_V3IO_TEST_CONTAINER_ALIAS"),
		streamPath:     "v3io-stream-test-" + xid.New().String(),
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

func (suite *testSuite) SetupSuite() {
	var err error

	suite.AbstractBrokerSuite.SetupSuite()

	suite.Logger.Info("Creating broker resources")

	suite.container, err = v3ioutil.CreateContainer(suite.Logger,
		suite.address,
		suite.containerAlias,
		2)

	suite.Require().NoError(err)

	numPartitions := 3

	// create stream
	err = suite.container.Sync.CreateStream(&v3iohttp.CreateStreamInput{
		Path:                 suite.streamPath + "/",
		ShardCount:           numPartitions,
		RetentionPeriodHours: 1,
	})

	suite.Require().NoError(err)

	// write initial data
	for partitionId := 0; partitionId < numPartitions; partitionId++ {
		suite.publishMessageToTopic(strconv.Itoa(partitionId), "ignore")
	}
}

func (suite *testSuite) TearDownSuite() {
	suite.AbstractBrokerSuite.TearDownSuite()

	// delete stream
	suite.container.Sync.DeleteStream(&v3iohttp.DeleteStreamInput{
		Path: suite.streamPath + "/",
	})
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = functionconfig.Trigger{
		Kind: "v3ioStream",
		URL:  fmt.Sprintf("http://%s/%s/%s", suite.address, suite.containerAlias, suite.streamPath),
		Attributes: map[string]interface{}{
			"partitions":          []int{0, 1, 2},
			"numContainerWorkers": 2,
			"seekTo":              "earliest",
			"readBatchSize":       64,
			"pollingIntervalMs":   250,
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			"0": {3},
			"1": {3},
			"2": {3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	partitionID, err := strconv.Atoi(topic)
	suite.Require().NoError(err)

	_, err = suite.container.Sync.PutRecords(&v3iohttp.PutRecordsInput{
		Path: suite.streamPath + "/",
		Records: []*v3iohttp.StreamRecord{
			{&partitionID, []byte(body)},
		},
	})

	return err
}

func TestIntegrationSuite(t *testing.T) {

	// don't run this suite unless commented (requires an iguazio system)
	return

	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
