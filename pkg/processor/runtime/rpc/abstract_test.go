//go:build test_integration

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

package rpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testRuntime struct {
	*AbstractRuntime
	wrapperProcess *os.Process
	eventConn      net.Conn
	controlConn    net.Conn
}

// NewRuntime returns a new Python runtime
func newTestRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (*testRuntime, error) {
	var err error

	newTestRuntime := &testRuntime{}

	newTestRuntime.AbstractRuntime, err = NewAbstractRuntime(parentLogger.GetChild("Logger"),
		configuration,
		newTestRuntime)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	newTestRuntime.AbstractRuntime.ControlMessageBroker = NewRpcControlMessageBroker(nil, parentLogger, nil)

	return newTestRuntime, nil
}

func (r *testRuntime) RunWrapper(eventSocketPaths []string, controlSocketPath string) (*os.Process, error) {
	if len(eventSocketPaths) > 1 {
		return nil, fmt.Errorf("test runtime doesn't support multiple socket processing yet")
	}
	var err error
	cmd := exec.Command("sleep", "999999")
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	r.wrapperProcess = cmd.Process

	var eventSocketPath string
	if len(eventSocketPaths) == 1 {
		eventSocketPath = eventSocketPaths[0]
	}

	// Connect to runtime
	r.eventConn, err = net.Dial("unix", eventSocketPath)
	if err != nil {
		return nil, err
	}

	if controlSocketPath != "" {
		r.controlConn, err = net.Dial("unix", controlSocketPath)
		if err != nil {
			return nil, err
		}
	}

	return cmd.Process, nil
}

func (r *testRuntime) GetEventEncoder(writer io.Writer) EventEncoder {
	return NewEventJSONEncoder(r.Logger, writer)
}

type RuntimeSuite struct {
	suite.Suite
	testRuntimeInstance *testRuntime
}

func (suite *RuntimeSuite) TestRestart() {
	var err error

	loggerInstance := suite.createLogger()
	configInstance := suite.createConfig(loggerInstance)

	suite.testRuntimeInstance, err = newTestRuntime(loggerInstance, configInstance)
	suite.Require().NoError(err, "Can't create runtime")

	err = suite.testRuntimeInstance.Start()
	suite.Require().NoError(err, "Can't start runtime")

	time.Sleep(1 * time.Second)

	oldPid := suite.testRuntimeInstance.wrapperProcess.Pid
	err = suite.testRuntimeInstance.Restart()
	suite.Require().NoError(err, "Can't restart runtime")
	suite.Require().NotEqual(oldPid, suite.testRuntimeInstance.wrapperProcess.Pid, "Wrapper process didn't change")
}

func (suite *RuntimeSuite) TestSubscribeToControlMessage() {
	var err error
	messageKind := controlcommunication.ControlMessageKind("test")

	loggerInstance := suite.createLogger()
	configInstance := suite.createConfig(loggerInstance)

	suite.testRuntimeInstance, err = newTestRuntime(loggerInstance, configInstance)
	suite.Require().NoError(err, "Can't create runtime")

	err = suite.testRuntimeInstance.Start()
	suite.Require().NoError(err, "Can't start runtime")

	time.Sleep(1 * time.Second)

	// create channel for consumer
	controlMessageChannel := make(chan *controlcommunication.ControlMessage)

	// subscribe to test message kind
	err = suite.testRuntimeInstance.GetControlMessageBroker().Subscribe(messageKind, controlMessageChannel)
	suite.Require().NoError(err, "Can't subscribe to control message")

	// create control message
	controlMessage := &controlcommunication.ControlMessage{
		Kind: messageKind,
		Attributes: map[string]interface{}{
			"test": "test",
		},
	}

	done := make(chan bool)

	// wait for control message in a goroutine
	go func() {
		receivedControlMessage := <-controlMessageChannel
		suite.Require().Equal(controlMessage, receivedControlMessage, "Received control message doesn't match")
		done <- true
	}()

	// send control message
	err = suite.testRuntimeInstance.GetControlMessageBroker().SendToConsumers(controlMessage)
	suite.Require().NoError(err, "Can't send control message")

	// wait for goroutine to finish
	testDone := <-done
	suite.Require().True(testDone, "Goroutine didn't finish")
}

func (suite *RuntimeSuite) TestReadControlMessage() {
	var err error

	loggerInstance := suite.createLogger()
	configInstance := suite.createConfig(loggerInstance)

	suite.testRuntimeInstance, err = newTestRuntime(loggerInstance, configInstance)
	suite.Require().NoError(err, "Can't create runtime")

	err = suite.testRuntimeInstance.Start()
	suite.Require().NoError(err, "Can't start runtime")

	// create control message
	controlMessage := &controlcommunication.ControlMessage{
		Kind: "testKind",
		Attributes: map[string]interface{}{
			"test": "test",
		},
	}

	// encode control message in json
	byteMessage, err := json.Marshal(controlMessage)
	suite.Require().NoError(err, "Can't encode control message")

	// add new line character to the end of the message
	byteMessage = append(byteMessage, '\n')

	// read control message from buffer
	buf := bufio.NewReader(bytes.NewReader(byteMessage))
	reslovedControlMessage, err := suite.testRuntimeInstance.ControlMessageBroker.ReadControlMessage(buf)

	// check if control message was read correctly
	suite.Require().NoError(err, "Can't read control message")
	suite.Require().Equal(controlMessage, reslovedControlMessage, "Read control message doesn't match")
}

func (suite *RuntimeSuite) TestUnmarshalResponseData() {
	for _, testCase := range []struct {
		name               string
		data               []byte
		unmarshalledResult []*result
	}{
		{
			name: "single-result",
			data: []byte("{\"body\": \"123\", \"content_type\": \"123\", \"headers\": {}, \"status_code\": 200, \"body_encoding\": \"text\"}"),
			unmarshalledResult: []*result{{
				StatusCode:   200,
				ContentType:  "123",
				Body:         "123",
				BodyEncoding: "text",
				DecodedBody:  []uint8{49, 50, 51},
				Headers:      map[string]interface{}{},
			}},
		},
		{
			name: "batch-result",
			data: []byte("[{\"body\": \"123\", \"content_type\": \"123\", \"headers\": {}, \"status_code\": 200, \"body_encoding\": \"text\"}]"),
			unmarshalledResult: []*result{{
				StatusCode:   200,
				ContentType:  "123",
				Body:         "123",
				BodyEncoding: "text",
				DecodedBody:  []uint8{49, 50, 51},
				Headers:      map[string]interface{}{},
			}},
		},
	} {
		suite.Run(testCase.name, func() {
			unmarshalledResults := newBatchedResults()
			unmarshalResponseData(suite.createLogger(), testCase.data, unmarshalledResults)
			suite.Require().Equal(unmarshalledResults.results, testCase.unmarshalledResult)
		})
	}
}

func (suite *RuntimeSuite) TearDownTest() {
	if suite.testRuntimeInstance != nil && suite.testRuntimeInstance.wrapperProcess != nil {
		suite.testRuntimeInstance.Stop() // nolint: errcheck
	}
}

func (suite *RuntimeSuite) createLogger() logger.Logger {
	loggerInstance, err := nucliozap.NewNuclioZapTest("rpc-runtime-test")
	suite.Require().NoError(err, "Can't create Logger")

	return loggerInstance
}

func (suite *RuntimeSuite) createConfig(loggerInstance logger.Logger) *runtime.Configuration {
	return &runtime.Configuration{
		FunctionLogger: loggerInstance,
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{
					Namespace: "test",
				},
				Spec: functionconfig.Spec{},
			},
			PlatformConfig: &platformconfig.Config{
				Kind: "docker",
			},
		},
	}
}

func TestRuntime(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(RuntimeSuite))
}
