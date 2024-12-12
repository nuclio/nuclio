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

package triggertest

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
)

type Request struct {
	Port     int
	Url      string
	Path     string
	Method   string
	Body     string
	Loglevel string
	Headers  map[string]interface{}
}

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

	HttpClient *http.Client
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

	newAbstractBrokerSuite.FunctionPaths["golang"] = path.Join(newAbstractBrokerSuite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"golang",
		"event_recorder.go")

	newAbstractBrokerSuite.FunctionPaths["java"] = path.Join(newAbstractBrokerSuite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"java",
		"Handler.java")

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

	// create http client
	suite.HttpClient = &http.Client{
		Timeout: 10 * time.Second,
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

func (suite *AbstractBrokerSuite) SendHTTPRequest(request *Request) (*http.Response, error) {
	host := suite.GetTestHost()

	suite.Logger.DebugWith("Sending request",
		"Host", host,
		"Port", request.Port,
		"Path", request.Path,
		"Headers", request.Headers,
		"BodyLength", len(request.Body),
		"LogLevel", request.Loglevel)

	// Send request to proper url
	if request.Url == "" {
		request.Url = fmt.Sprintf("http://%s:%d%s", host, request.Port, request.Path)
	}

	if request.Path == "" {
		request.Path = "/"
	}

	// create a request
	httpRequest, err := http.NewRequest(request.Method, request.Url, strings.NewReader(request.Body))
	suite.Require().NoError(err)

	// if there are request headers, add them
	if request.Headers != nil {
		for headerName, headerValue := range request.Headers {
			httpRequest.Header.Add(headerName, fmt.Sprintf("%v", headerValue))
		}
	} else {
		httpRequest.Header.Add("Content-Type", "text/plain")
	}

	// if there is a log level, add the header
	if request.Loglevel != "" {
		httpRequest.Header.Add(headers.LogLevel, request.Loglevel)
	}

	// invoke the function
	return suite.HttpClient.Do(httpRequest)
}
