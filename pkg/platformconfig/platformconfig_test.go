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
	"github.com/nuclio/nuclio/test/compare"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type PlatformConfigTestSuite struct {
	suite.Suite
	logger logger.Logger
	reader *Reader
}

func (suite *PlatformConfigTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.reader, _ = NewReader()
}

func (suite *PlatformConfigTestSuite) TestReadConfiguration() {
	configurationContents := `
webAdmin:
  enabled: true
  listenAddress: :8081
logger:
  sinks:
    stdout:
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
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
      kind: prometheusPush
      url: 10.0.0.1:30
      attributes:
        interval: "10s"
  system:
  - mypush
  functions:
  - mypush
`

	var readConfiguration, expectedConfiguration Configuration

	// init expected
	trueValue := true
	expectedConfiguration.WebAdmin.Enabled = &trueValue
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
		Kind: "stdout",
	}

	expectedConfiguration.Logger.Sinks["staging-es"] = LoggerSink{
		Kind: "elasticsearch",
		URL:  "http://10.0.0.1:9200",
	}

	expectedConfiguration.Logger.Sinks["prod-es"] = LoggerSink{
		Kind: "elasticsearch",
		URL:  "http://20.0.1:9200",
		Attributes: map[string]interface{}{
			"dontCrash": true,
		},
	}

	// metric
	expectedConfiguration.Metrics.System = []string{"mypush"}
	expectedConfiguration.Metrics.Functions = []string{"mypush"}

	// metric sinks
	expectedConfiguration.Metrics.Sinks = map[string]MetricSink{}

	expectedConfiguration.Metrics.Sinks["mypush"] = MetricSink{
		Kind: "prometheusPush",
		URL:  "10.0.0.1:30",
		Attributes: map[string]interface{}{
			"interval": "10s",
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
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
      url: http://20.0.1:9200
  system:
  - level: debug
    sink: prod-es
  - level: info
    sink: stdout
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	systemLoggerSinks, err := readConfiguration.GetSystemLoggerSinks()
	suite.Require().NoError(err)

	expectedSystemLoggerSinks := map[string]LoggerSinkWithLevel{
		"prod-es": {
			Level: "debug",
			Sink: LoggerSink{
				Kind: "elasticsearch",
				URL:  "http://20.0.1:9200",
			},
		},
		"stdout": {
			Level: "info",
			Sink: LoggerSink{
				Kind: "stdout",
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
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
      url: http://20.0.1:9200
  system:
  - level: debug
    sink: blah
  - level: info
    sink: stdout
`

	var readConfiguration Configuration

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
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: stdout
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	functionLoggerSinks, err := readConfiguration.GetFunctionLoggerSinks(functionconfig.NewConfig())
	suite.Require().NoError(err)

	expectedFunctionLoggerSinks := map[string]LoggerSinkWithLevel{
		"stdout": {
			Level: "info",
			Sink: LoggerSink{
				Kind: "stdout",
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
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: stdout
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	functionConfig := functionconfig.NewConfig()
	functionConfig.Spec.LoggerSinks = []functionconfig.LoggerSink{
		{
			Level: "debug",
			Sink:  "staging-es",
		},
		{
			Level: "warn",
			Sink:  "stdout",
		},
	}

	functionLoggerSinks, err := readConfiguration.GetFunctionLoggerSinks(functionConfig)
	suite.Require().NoError(err)

	expectedFunctionLoggerSinks := map[string]LoggerSinkWithLevel{
		"stdout": {
			Level: "warn",
			Sink: LoggerSink{
				Kind: "stdout",
			},
		},
		"staging-es": {
			Level: "debug",
			Sink: LoggerSink{
				Kind: "elasticsearch",
				URL:  "http://10.0.0.1:9200",
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
      kind: stdout
    staging-es:
      kind: elasticsearch
      url: http://10.0.0.1:9200
    prod-es:
      kind: elasticsearch
      url: http://20.0.1:9200
  functions:
  - level: info
    sink: blah
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	_, err = readConfiguration.GetFunctionLoggerSinks(functionconfig.NewConfig())
	suite.Require().Error(err)
}

func (suite *PlatformConfigTestSuite) TestGetSystemMetricSinks() {
	configurationContents := `
metrics:
  sinks:
    pushSink:
      kind: prometheusPush
      url: 10.0.0.1:30
      attributes:
        interval: "10s"
    pullSink:
      kind: prometheusPull
  system:
  - pushSink
  functions:
  - pullSink
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	systemMetricSinks, err := readConfiguration.GetSystemMetricSinks()
	suite.Require().NoError(err)

	expectedSystemMetricSinks := map[string]MetricSink{
		"pushSink": {
			Kind: "prometheusPush",
			URL:  "10.0.0.1:30",
			Attributes: map[string]interface{}{
				"interval": "10s",
			},
		},
	}

	suite.Require().True(compare.CompareNoOrder(expectedSystemMetricSinks, systemMetricSinks))
}

func (suite *PlatformConfigTestSuite) TestGetSystemMetricSinksInvalidSink() {
	configurationContents := `
metrics:
  sinks:
    pushSink:
      kind: prometheusPush
      url: 10.0.0.1:30
      attributes:
        interval: "10s"
    pullSink:
      kind: prometheusPull
  system:
  - blah
  functions:
  - pullSink
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	_, err = readConfiguration.GetSystemMetricSinks()
	suite.Require().Error(err)
}

func (suite *PlatformConfigTestSuite) TestGetFunctionMetricSinks() {
	configurationContents := `
metrics:
  sinks:
    pushSink:
      kind: prometheusPush
      url: 10.0.0.1:30
      attributes:
        interval: "10s"
    pullSink:
      kind: prometheusPull
  system:
  - pushSink
  functions:
  - pullSink
`

	var readConfiguration Configuration

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	functionMetricSinks, err := readConfiguration.GetFunctionMetricSinks()
	suite.Require().NoError(err)

	expectedFunctionMetricSinks := map[string]MetricSink{
		"pullSink": {
			Kind: "prometheusPull",
		},
	}

	suite.Require().True(compare.CompareNoOrder(expectedFunctionMetricSinks, functionMetricSinks))
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(PlatformConfigTestSuite))
}
