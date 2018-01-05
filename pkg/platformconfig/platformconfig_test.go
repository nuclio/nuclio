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

package platformconfig

import (
	"bytes"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/nuclio/nuclio/test/compare"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type PlatformConfigTestSuite struct {
	suite.Suite
	logger nuclio.Logger
	reader *Reader
}

func (suite *PlatformConfigTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.reader, _ = NewReader(suite.logger)
}

func (suite *PlatformConfigTestSuite) TestReadConfiguration() {
	configurationContents := `
webAdmin:
  enabled: true
  listenAddress: :8081
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
      attributes:
        dontCrash: true
  system:
  - level: debug
    sink: prod-es
  - level: info
    sink: stdout
  functions:
  - level: info
    sink: stdout
metrics:
  sinks:
    mypush:
      driver: prometheusPush
      url: 10.0.0.1:30
      attributes:
        someInterval: 10s
  enabled: true
  defaultSink: mypush
`

	var readConfiguration, expectedConfiguration Config

	// init expected
	expectedConfiguration.WebAdmin.Enabled = true
	expectedConfiguration.WebAdmin.ListenAddress = ":8081"

	// logger
	expectedConfiguration.Logger.System = append(expectedConfiguration.Logger.System, LoggerSinkBinding{
		Level: "debug",
		Sink:  "prod-es",
	})

	expectedConfiguration.Logger.System = append(expectedConfiguration.Logger.System, LoggerSinkBinding{
		Level: "info",
		Sink:  "stdout",
	})

	expectedConfiguration.Logger.Functions = append(expectedConfiguration.Logger.Functions, LoggerSinkBinding{
		Level: "info",
		Sink:  "stdout",
	})

	// logger sinks
	expectedConfiguration.Logger.Sinks = map[string]LoggerSink{}

	expectedConfiguration.Logger.Sinks["stdout"] = LoggerSink{
		Driver: "stdout",
	}

	expectedConfiguration.Logger.Sinks["staging-es"] = LoggerSink{
		Driver: "elasticsearch",
		URL:    "http://10.0.0.1:9200",
	}

	expectedConfiguration.Logger.Sinks["prod-es"] = LoggerSink{
		Driver: "elasticsearch",
		URL:    "http://20.0.1:9200",
		Attributes: map[string]interface{}{
			"dontCrash": true,
		},
	}

	// metric
	expectedConfiguration.Metrics.Enabled = true
	expectedConfiguration.Metrics.DefaultSink = "mypush"

	// metric sinks
	expectedConfiguration.Metrics.Sinks = map[string]MetricSink{}

	expectedConfiguration.Metrics.Sinks["mypush"] = MetricSink{
		Driver: "prometheusPush",
		URL:    "10.0.0.1:30",
		Attributes: map[string]interface{}{
			"someInterval": "10s",
		},
	}

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	suite.Require().True(compare.CompareNoOrder(expectedConfiguration, readConfiguration))
}

func (suite *PlatformConfigTestSuite) TestGetSystemLoggerSinks() {
	configurationContents := `
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
  system:
  - level: debug
    sink: prod-es
  - level: info
    sink: stdout
`

	var readConfiguration Config

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	systemLoggerSinks, err := readConfiguration.GetSystemLoggerSinks()
	suite.Require().NoError(err)

	expectedSystemLoggerSinks := []LoggerSinkWithLevel{
		{
			Level: "debug",
			Sink: LoggerSink{
				Driver: "elasticsearch",
				URL:    "http://20.0.1:9200",
			},
		},
		{
			Level: "info",
			Sink: LoggerSink{
				Driver: "stdout",
			},
		},
	}

	suite.Require().True(compare.CompareNoOrder(expectedSystemLoggerSinks, systemLoggerSinks))
}

func (suite *PlatformConfigTestSuite) TestGetSystemLoggerSinksInvalidSink() {
	configurationContents := `
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
  system:
  - level: debug
    sink: blah
  - level: info
    sink: stdout
`

	var readConfiguration Config

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	_, err = readConfiguration.GetSystemLoggerSinks()
	suite.Require().Error(err)
}

func (suite *PlatformConfigTestSuite) TestGetFunctionLoggerSinksNoFunctionConfig() {
	configurationContents := `
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: stdout
`

	var readConfiguration Config

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	functionLoggerSinks, err := readConfiguration.GetFunctionLoggerSinks(functionconfig.NewConfig())
	suite.Require().NoError(err)

	expectedFunctionLoggerSinks := []LoggerSinkWithLevel{
		{
			Level: "info",
			Sink: LoggerSink{
				Driver: "stdout",
			},
		},
	}

	suite.Require().True(compare.CompareNoOrder(expectedFunctionLoggerSinks, functionLoggerSinks))
}


func (suite *PlatformConfigTestSuite) TestGetFunctionLoggerSinksWithFunctionConfig() {
	configurationContents := `
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: stdout
`

	var readConfiguration Config

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	functionConfig := functionconfig.NewConfig()
	functionConfig.Spec.LoggerSinks = []functionconfig.LoggerSink{
		{
			Level: "debug",
			Sink: "staging-es",
		},
		{
			Level: "warn",
			Sink: "stdout",
		},
	}

	functionLoggerSinks, err := readConfiguration.GetFunctionLoggerSinks(functionConfig)
	suite.Require().NoError(err)

	expectedFunctionLoggerSinks := []LoggerSinkWithLevel{
		{
			Level: "warn",
			Sink: LoggerSink{
				Driver: "stdout",
			},
		},
		{
			Level: "debug",
			Sink: LoggerSink{
				Driver: "elasticsearch",
				URL:    "http://10.0.0.1:9200",
			},
		},
	}

	suite.Require().True(compare.CompareNoOrder(expectedFunctionLoggerSinks, functionLoggerSinks))
}


func (suite *PlatformConfigTestSuite) TestGetFunctionLoggerSinksInvalidSink() {
	configurationContents := `
logger:
  sinks:
    stdout:
      driver: stdout
    staging-es:
      driver: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      driver: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: blah
`

	var readConfiguration Config

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	_, err = readConfiguration.GetFunctionLoggerSinks(functionconfig.NewConfig())
	suite.Require().Error(err)
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(PlatformConfigTestSuite))
}
