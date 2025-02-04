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
package golang

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type runtimeTestSuite struct {
	suite.Suite
	logger                 logger.Logger
	processorConfiguration *processor.Configuration
	runtimeConfiguration   *runtime.Configuration
	handler                handler
}

func (suite *runtimeTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.processorConfiguration = &processor.Configuration{
		Config:         *functionconfig.NewConfig(),
		PlatformConfig: &platformconfig.Config{},
	}

	suite.runtimeConfiguration = &runtime.Configuration{
		Configuration:  suite.processorConfiguration,
		FunctionLogger: suite.logger,
	}
	abstractHandler := abstractHandler{
		logger: suite.logger,
		entrypoint: func(*nuclio.Context, nuclio.Event) (interface{}, error) {
			time.Sleep(5 * time.Second)
			return "success", nil
		},
	}
	suite.handler = &pluginHandlerLoader{
		abstractHandler,
	}
}

func (suite *runtimeTestSuite) TestRestartRuntime() {
	golangRuntime, err := NewRuntime(suite.logger, suite.runtimeConfiguration, suite.handler)
	suite.Require().NoError(err)

	processingEventResult := make(chan error, 1)
	defer close(processingEventResult)
	go func() {
		var err error
		_, err = golangRuntime.ProcessEvent(nil, suite.logger)
		processingEventResult <- err
	}()
	// wait until processing starts, otherwise runtime will restart before event processing start
	time.Sleep(1 * time.Second)

	err = golangRuntime.Restart()
	suite.Require().NoError(err)

	select {
	case result := <-processingEventResult:
		suite.Require().Error(result, "Event processing was cancelled")
	// Security guard to prevent waiting indefinitely for a result in the channel
	// 1 second timeout is sufficient here.
	case <-time.After(1 * time.Second):
		suite.Require().Fail("Event processing wasn't stopped during runtime restart")
	}
	suite.Require().Equal(uint64(0), golangRuntime.GetStatistics().DurationMilliSecondsCount)
	suite.Require().Equal(uint64(0), golangRuntime.GetStatistics().DurationMilliSecondsSum)

	// when goroutine is stopped, it should not calculate metrics
	time.Sleep(5 * time.Second)
	suite.Require().Equal(uint64(0), golangRuntime.GetStatistics().DurationMilliSecondsCount)
	suite.Require().Equal(uint64(0), golangRuntime.GetStatistics().DurationMilliSecondsSum)
}

func (suite *runtimeTestSuite) TestProcessEvent() {
	golangRuntime, err := NewRuntime(suite.logger, suite.runtimeConfiguration, suite.handler)
	suite.Require().NoError(err)

	res, err := golangRuntime.ProcessEvent(nil, suite.logger)
	suite.Require().NoError(err)
	suite.Require().Equal(res, "success")
}

func (suite *runtimeTestSuite) TestPanicHandler() {
	abstractHandler := abstractHandler{
		logger: suite.logger,
		entrypoint: func(*nuclio.Context, nuclio.Event) (interface{}, error) {
			panic("PANIC")
		},
	}
	panicHandler := &pluginHandlerLoader{
		abstractHandler,
	}
	golangRuntime, err := NewRuntime(suite.logger, suite.runtimeConfiguration, panicHandler)
	suite.Require().NoError(err)

	res, err := golangRuntime.ProcessEvent(nil, suite.logger)
	suite.Require().Error(err)
	suite.Require().Equal(nil, res)
	suite.Require().Equal(err.Error(), "Caught panic: PANIC")
}

func TestRuntimeTestSuite(t *testing.T) {
	suite.Run(t, new(runtimeTestSuite))
}
