//go:build test_unit

/*
Copyright 2025 The Nuclio Authors.

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
package connection

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"
	triggertest "github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TestConnectionSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *TestConnectionSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("abstract-connection")
	suite.Require().NoError(err)
}

func (suite *TestConnectionSuite) TestStreamProcessing() {
	mockManager := &MockConnectionManager{}
	managerConfiguration := suite.getManagerConfiguration()

	mockManager.On("GetConfig").Return(managerConfiguration).Twice()
	connection := NewAbstractEventConnection(suite.logger, mockManager)

	var buffer bytes.Buffer
	connection.encoder = encoder.NewEventJSONEncoder(suite.logger, &buffer)
	defer connection.stop() //nolint: errcheck

	testStreamValue := "test-stream"
	testStreamValueBase64 := base64.StdEncoding.EncodeToString([]byte(testStreamValue))
	responseStream := nuclio.NewResponseStream("", nil, 200)
	sendMessageErrChan := make(chan error, 1)
	defer close(sendMessageErrChan)
	go func() {
		var err error
		// send stream messages to the connection's result channel
		messages := []result.Result{result.NewStreamStart(responseStream), result.NewBodyOnlyFromBase64([]byte(testStreamValueBase64)), result.NewBodyOnlyFromBase64([]byte(testStreamValueBase64)),
			&result.StreamEnd{}}
		for index, message := range messages {
			if err = suite.sendToChanOrFail(connection.resultChan, message, 5*time.Second); err != nil {
				sendMessageErrChan <- errors.Wrap(err, fmt.Sprintf("Failed to send message, index: %d", index))
				return
			}
		}
		sendMessageErrChan <- nil
	}()
	// process the event, which is a stream start
	processingResult, err := connection.ProcessEvent(&triggertest.TestEvent{}, suite.logger)
	suite.Require().NoError(err)
	suite.Require().Equal(processingResult.GetProcessingResult(), responseStream)

	// ensure that logger is still set
	suite.Require().Equal(suite.logger, connection.functionLogger)

	stream := processingResult.(*result.StreamStart)
	streamProcessErrChan := make(chan error, 1)
	defer close(streamProcessErrChan)
	go func() {
		// process the stream and writes the results to the io.writer
		streamProcessErrChan <- connection.ProcessStream(stream)
	}()

	body := stream.GetBody().(io.ReadCloser)
	data, err := io.ReadAll(body)
	suite.Require().NoError(err)
	suite.Require().Equal(fmt.Sprintf("%s%s", testStreamValue, testStreamValue), string(data))

	streamProcessErr := <-streamProcessErrChan
	suite.Require().NoError(streamProcessErr)

	sendMessageErr := <-sendMessageErrChan
	suite.Require().NoError(sendMessageErr)

	// after stream processing is finished, the stream should be closed and functionLogger should be nil
	suite.Require().Equal(nil, connection.functionLogger)
}

func (suite *TestConnectionSuite) getManagerConfiguration() ManagerConfigration {
	return ManagerConfigration{
		Kind:                        SocketAllocatorManagerKind,
		SupportControlCommunication: true,
		WaitForStart:                true,
		eventTimeout:                10 * time.Second,
		GetEventEncoderFunc: func(w io.Writer) encoder.EventEncoder {
			return encoder.NewEventJSONEncoder(nil, w)
		},
	}
}

func (suite *TestConnectionSuite) sendToChanOrFail(resultChan chan result.Result, message result.Result, timeout time.Duration) error {
	select {
	case resultChan <- message:
		return nil
	case <-time.After(timeout):
		return errors.New("sending message to a channel timed out")
	}
}

func TestConnection(t *testing.T) {
	suite.Run(t, &TestConnectionSuite{})
}
