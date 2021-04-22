// +build test_integration
// +build test_iguazio

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

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	// kafka clients
	v3ioContext   v3io.Context
	v3ioContainer v3io.Container
	numWorkers    int
	shardCount    int

	// v3io trigger attributes
	url           string
	accessKey     string
	containerName string
	streamPath    string
	consumerGroup string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// Change below
	suite.url = "https://somewhere:8444"
	suite.accessKey = "some-access-key"
	// END OF change

	suite.numWorkers = 8
	suite.shardCount = 1

	// v3io trigger attributes
	suite.containerName = "bigdata"
	suite.streamPath = fmt.Sprintf("test-nuclio-%s/", xid.New().String())   // must have `/` at the end
	suite.consumerGroup = fmt.Sprintf("test_nuclio_%s", xid.New().String()) // use `_` not `-`

	// we use an Iguazio system, not a containerzed broker
	suite.AbstractBrokerSuite.SkipStartBrokerContainer = true
	suite.AbstractBrokerSuite.SetupSuite()

	// create broker
	suite.v3ioContext, err = v3iohttp.NewContext(suite.Logger, &v3iohttp.NewContextInput{
		NumWorkers: suite.numWorkers,
	})
	suite.Require().NoError(err, "Failed to create v3io context")

	v3ioSession, err := suite.v3ioContext.NewSession(&v3io.NewSessionInput{
		URL:       suite.url,
		AccessKey: suite.accessKey,
	})
	suite.Require().NoError(err, "Failed to create v3io session")

	suite.v3ioContainer, err = v3ioSession.NewContainer(&v3io.NewContainerInput{
		ContainerName: suite.containerName,
	})
	suite.Require().NoError(err, "Failed to create v3io container")

	err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 suite.streamPath,
		ShardCount:           suite.shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")
}

func (suite *testSuite) TearDownSuite() {
	suite.Logger.DebugWith("Deleting stream sync", "streamPath", suite.streamPath)
	err := suite.v3ioContainer.DeleteStreamSync(&v3io.DeleteStreamInput{
		Path: suite.streamPath,
	})
	suite.Require().NoError(err)
	suite.AbstractBrokerSuite.TearDownSuite()
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"test-nuclio-v3io": {
			Kind:     "v3ioStream",
			URL:      suite.url,
			Password: suite.accessKey,
			Attributes: map[string]interface{}{
				"seekTo":        "earliest", // avoid race condition with `latest` missed by function
				"containerName": suite.containerName,
				"streamPath":    suite.streamPath,
				"consumerGroup": suite.consumerGroup,
			},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.streamPath: {
				NumMessages: suite.numWorkers,
			},
		},
		nil,
		suite.writingMessageToStream)
}

// GetContainerRunInfo returns nothing as Iguazio WebAPI has no Docker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "", nil
}

func (suite *testSuite) writingMessageToStream(streamPath string, body string) error {
	suite.Logger.InfoWith("Publishing message to stream",
		"streamPath", streamPath,
		"body", body)
	response, err := suite.v3ioContainer.PutRecordsSync(&v3io.PutRecordsInput{
		Path: streamPath,
		Records: []*v3io.StreamRecord{
			{
				Data: []byte(body),
			},
		},
	})
	suite.Require().NoError(err)

	suite.Logger.InfoWith("Successfully wrote a message to stream",
		"response", response,
		"streamPath", streamPath)
	response.Release()
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
