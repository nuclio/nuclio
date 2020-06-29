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

package triggertest

import (
	"path"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
)

// BrokerSuite tests a broker by producing messages
type BrokerSuite interface {

	// GetContainerRunInfo returns information about the broker container
	GetContainerRunInfo() (string, *dockerclient.RunOptions)

	// WaitForBroker waits until the broker is ready
	WaitForBroker() error
}

type AbstractBrokerSuite struct {
	processorsuite.TestSuite
	brokerContainerID string
	brokerSuite       BrokerSuite
	BrokerHost        string
	FunctionPaths     map[string]string
}

func NewAbstractBrokerSuite(brokerSuite BrokerSuite) *AbstractBrokerSuite {
	newAbstractBrokerSuite := &AbstractBrokerSuite{
		brokerSuite:   brokerSuite,
		FunctionPaths: map[string]string{},
	}

	newAbstractBrokerSuite.BrokerHost = newAbstractBrokerSuite.GetTestHost()

	// use the python event recorder
	newAbstractBrokerSuite.FunctionPaths["python"] = path.Join(newAbstractBrokerSuite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"python",
		"event_recorder.py")

	return newAbstractBrokerSuite
}

func (suite *AbstractBrokerSuite) SetupSuite() {
	var err error

	// call parent
	suite.TestSuite.SetupSuite()

	// get container information
	imageName, runOptions := suite.brokerSuite.GetContainerRunInfo()

	suite.Logger.InfoWith("Starting broker", "imageName", imageName, "BrokerHost", suite.BrokerHost)

	// start the broker
	if imageName != "" {
		suite.brokerContainerID, err = suite.DockerClient.RunContainer(imageName, runOptions)
		suite.Require().NoError(err, "Failed to start broker container")

		// wait for the broker to be ready
		err = suite.brokerSuite.WaitForBroker()
		suite.Require().NoError(err, "Error waiting for broker to be ready")
	}
}

func (suite *AbstractBrokerSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.brokerContainerID != "" {
		suite.DockerClient.RemoveContainer(suite.brokerContainerID) // nolint: errcheck
	}
}

// WaitForBroker waits until the broker is ready
func (suite *AbstractBrokerSuite) WaitForBroker() error {
	time.Sleep(5 * time.Second)

	return nil
}

func (suite *AbstractBrokerSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "", nil
}
