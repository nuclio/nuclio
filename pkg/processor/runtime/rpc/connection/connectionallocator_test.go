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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TestConnectionAllocatorSuite struct {
	suite.Suite
	ctx                 context.Context
	connectionAllocator *ConnectionAllocator

	logger     logger.Logger
	mockServer *httptest.Server
}

type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "testClass" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "testKind" }
func (ti *TestTriggerInfoProvider) GetName() string  { return "testName" }

// SetupTest runs before every test
func (suite *TestConnectionAllocatorSuite) SetupTest() {
	var err error
	suite.ctx = context.Background()
	suite.logger, err = nucliozap.NewNuclioZapTest("abstract-connection")
	suite.Require().NoError(err)

	connectionManagerConfiguration := NewManagerConfigration(
		true,
		false,
		UnixSocket,
		func(writer io.Writer) encoder.EventEncoder {
			return encoder.NewEventMsgPackEncoder(suite.logger, writer)
		},
		&runtime.Statistics{},
		0,
		functionconfig.AsyncTriggerWorkMode,
		0,
	)
	runtimeConfiguration := runtime.Configuration{
		Mode: functionconfig.AsyncTriggerWorkMode,
		AsyncConfig: &functionconfig.AsyncConfig{
			MinConnectionsNumber:   1,
			MaxConnectionsNumber:   1,
			ConnectionCreationMode: functionconfig.ConnectionCreationModeStatic,
		},
	}

	connectionManager, err := NewConnectionManager(
		suite.logger,
		runtimeConfiguration,
		connectionManagerConfiguration)
	suite.connectionAllocator = connectionManager.(*ConnectionAllocator)
	suite.Require().NoError(err)
}

func (suite *TestConnectionAllocatorSuite) TestReplaceConnection() {
	// Start mock server
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	suite.connectionAllocator.serverAddress = suite.mockServer.Listener.Addr().String()

	// Prepare the connection allocator
	err := suite.connectionAllocator.startEventConnections()
	suite.Require().NoError(err)

	// allocate connection
	connection, err := suite.connectionAllocator.Allocate(1 * time.Minute)
	suite.Require().NoError(err)
	suite.Require().Equal(connection.GetStatus(), status.Ready)

	// initialise a basic event
	event := &nuclio.AbstractEvent{}
	event.SetTriggerInfoProvider(&TestTriggerInfoProvider{})

	// send event to connection and check response; we expect failure because the mock server is closed below
	ctx, cancelFunc := context.WithTimeout(suite.ctx, 30*time.Second)
	go func() {
		// send event to connection
		response, err := connection.ProcessEvent(event, suite.logger)
		suite.Require().Error(err)
		suite.Require().Equal(nil, response)
		cancelFunc()
	}()
	// close the mock server, this simulates a disconnect from wrapper side
	suite.mockServer.Close()

	// wait for the connection to be closed
	<-ctx.Done()

	// status should be restart required
	suite.Require().Equal(status.RestartRequired, connection.GetStatus())

	// starting mock server again to be able to establish new connection
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	suite.connectionAllocator.serverAddress = suite.mockServer.Listener.Addr().String()

	// during the release process, connection should be recreated
	suite.connectionAllocator.Release(connection)

	// try to allocate again, not only should it be allocatable, but also the status should be ready
	connection, err = suite.connectionAllocator.Allocate(1 * time.Minute)
	suite.Require().NoError(err)

	suite.Require().Equal(connection.GetStatus(), status.Ready)

	ctx, cancelFunc = context.WithTimeout(suite.ctx, 30*time.Second)
	go func() {
		response, err := connection.ProcessEvent(event, suite.logger)
		suite.Require().NoError(err)
		suite.Require().Equal("hello", string(response.GetBody().([]byte)))
		cancelFunc()
	}()
	connection.(*Connection).resultChan <- &result.BatchedResults{
		Results: []*result.Result{{DecodedBody: []byte("hello")}},
		Err:     nil,
	}
	<-ctx.Done()
}

func TestConnectionAllocator(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, &TestConnectionAllocatorSuite{})
}
