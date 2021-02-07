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

	"github.com/nuclio/nuclio/pkg/common"
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
	brokerSuite BrokerSuite

	BrokerHost                 string
	FunctionPaths              map[string]string
	BrokerContainerID          string
	BrokerContainerNetworkName string
	SkipStartBrokerContainer   bool
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

	// call parent
	suite.TestSuite.SetupSuite()

	// ensure network
	suite.EnsureDockerNetworkExisting()

	// get container information
	imageName, runOptions := suite.brokerSuite.GetContainerRunInfo()

	// start the broker
	if imageName != "" && !suite.SkipStartBrokerContainer {
		suite.Logger.InfoWith("Starting broker container",
			"imageName", imageName,
			"BrokerHost", suite.BrokerHost)
		suite.StartBrokerContainer(imageName, runOptions)
	}
}

func (suite *AbstractBrokerSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.BrokerContainerID != "" {
		err := suite.DockerClient.RemoveContainer(suite.BrokerContainerID)
		suite.NoError(err)
	}

	if suite.BrokerContainerNetworkName != "" {
		err := suite.DockerClient.DeleteNetwork(suite.BrokerContainerNetworkName)
		suite.NoError(err)
	}
}

func (suite *AbstractBrokerSuite) StartBrokerContainer(imageName string,
	runOptions *dockerclient.RunOptions) {
	var err error
	suite.BrokerContainerID = suite.RunContainer(imageName, runOptions)

	// wait for the broker to be ready
	err = suite.brokerSuite.WaitForBroker()
	suite.Require().NoError(err, "Error waiting for broker to be ready")
}

func (suite *AbstractBrokerSuite) RunContainer(imageName string,
	runOptions *dockerclient.RunOptions) string {
	containerID, err := suite.DockerClient.RunContainer(imageName, runOptions)
	suite.Require().NoError(err, "Failed to start broker container")
	return containerID
}

// WaitForBroker waits until the broker is ready
func (suite *AbstractBrokerSuite) WaitForBroker() error {
	time.Sleep(5 * time.Second)

	return nil
}

func (suite *AbstractBrokerSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "", nil
}

func (suite *AbstractBrokerSuite) EnsureDockerNetworkExisting() {

	// nothing to do here
	if suite.BrokerContainerNetworkName == "" {
		return
	}

	// these are pre-defined docker networks and must exists, skip
	if common.StringInSlice(suite.BrokerContainerNetworkName, []string{"none", "host", "bridge"}) {
		return
	}

	if err := suite.DockerClient.CreateNetwork(&dockerclient.CreateNetworkOptions{
		Name: suite.BrokerContainerNetworkName,
	}); err != nil {
		if !common.MatchStringPatterns([]string{
			`is a pre-defined network`,
			`already exists`,
		}, err.Error()) {
			suite.Require().Failf("Failed to create network %s. Err: %s",
				suite.BrokerContainerNetworkName, err.Error())
		}
	}
}
