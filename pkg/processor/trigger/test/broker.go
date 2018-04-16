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
	"os"
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

// AbstractBrokerSuite is common for broker tests
type AbstractBrokerSuite struct {
	processorsuite.TestSuite
	ContainerID   string
	brokerSuite   BrokerSuite
	BrokerHost    string
	FunctionPaths map[string]string
}

// NewAbstractBrokerSuite return a new AbstractBrokerSuite
func NewAbstractBrokerSuite(brokerSuite BrokerSuite) *AbstractBrokerSuite {
	brokerHost := "localhost"

	// Check if dockerized, if so set url to given NUCLIO_TEST_HOST
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		brokerHost = os.Getenv("NUCLIO_TEST_HOST")
	}

	newAbstractBrokerSuite := &AbstractBrokerSuite{
		brokerSuite:   brokerSuite,
		BrokerHost:    brokerHost,
		FunctionPaths: map[string]string{},
	}

	// use the python event recorder
	newAbstractBrokerSuite.FunctionPaths["python"] = path.Join(newAbstractBrokerSuite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"python",
		"event_recorder.py")

	return newAbstractBrokerSuite
}

// SetupSuite sets up the suite
func (suite *AbstractBrokerSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	// get container information
	imageName, runOptions := suite.brokerSuite.GetContainerRunInfo()

	suite.Logger.InfoWith("Starting broker", "imageName", imageName)

	// start the broker
	if imageName != "" {
		var err error
		suite.ContainerID, err = suite.DockerClient.RunContainer(imageName, runOptions)
		suite.Require().NoError(err, "Failed to start broker container")

		// wait for the broker to be ready
		err = suite.brokerSuite.WaitForBroker()
		suite.Require().NoError(err, "Error waiting for broker to be ready")
	}
}

// TearDownSuite tears does the suite
func (suite *AbstractBrokerSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.ContainerID != "" && os.Getenv("NUCLIO_TEST_KEEP_DOCKER") == "" {
		suite.DockerClient.RemoveContainer(suite.ContainerID) // nolint: errcheck
	}
}

// WaitForBroker waits until the broker is ready
func (suite *AbstractBrokerSuite) WaitForBroker() error {
	time.Sleep(5 * time.Second)

	return nil
}

// GetContainerRunInfo return the broker container information
func (suite *AbstractBrokerSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "", nil
}
