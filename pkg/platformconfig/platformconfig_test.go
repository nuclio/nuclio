//go:build test_unit

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
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
functionReadinessTimeout: 10s
webAdmin:
  enabled: true
  listenAddress: :8081
kube:
  defaultFunctionTolerations:
  - key: somekey
    value: somevalue
    effect: NoSchedule
  defaultFunctionNodeSelector:
    defaultFunctionNodeSelectorKey: defaultFunctionNodeSelectorValue
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

	var readConfiguration, expectedConfiguration Config

	// init expected
	trueValue := true
	expectedConfiguration.WebAdmin.Enabled = &trueValue
	expectedConfiguration.WebAdmin.ListenAddress = ":8081"

	tenSecondsStr := "10s"
	expectedConfiguration.FunctionReadinessTimeout = &tenSecondsStr

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

	expectedConfiguration.Kube.DefaultFunctionNodeSelector = map[string]string{
		"defaultFunctionNodeSelectorKey": "defaultFunctionNodeSelectorValue",
	}

	expectedConfiguration.Kube.DefaultFunctionTolerations = []corev1.Toleration{
		{
			Key:    "somekey",
			Value:  "somevalue",
			Effect: corev1.TaintEffectNoSchedule,
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

	suite.Require().Empty(cmp.Diff(&expectedConfiguration, &readConfiguration, cmpopts.IgnoreUnexported(Config{})))
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

	var readConfiguration Config

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

	suite.Require().Empty(cmp.Diff(&expectedSystemLoggerSinks, &systemLoggerSinks, cmpopts.IgnoreUnexported(Config{})))
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

	var readConfiguration Config

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

	suite.Require().Empty(cmp.Diff(&expectedFunctionLoggerSinks, &functionLoggerSinks, cmpopts.IgnoreUnexported(Config{})))
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

	var readConfiguration Config

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

	suite.Require().Empty(cmp.Diff(&expectedFunctionLoggerSinks, &functionLoggerSinks, cmpopts.IgnoreUnexported(Config{})))
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

	var readConfiguration Config

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

	var readConfiguration Config

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

	suite.Require().Empty(cmp.Diff(&expectedSystemMetricSinks, &systemMetricSinks, cmpopts.IgnoreUnexported(Config{})))
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

	var readConfiguration Config

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

	var readConfiguration Config

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

	suite.Require().Empty(cmp.Diff(&expectedFunctionMetricSinks, &functionMetricSinks, cmpopts.IgnoreUnexported(Config{})))
}

func (suite *PlatformConfigTestSuite) TestFunctionAugmentedConfigs() {
	var readConfiguration Config
	zero := 0
	ten := 10
	minReadySeconds := 90
	configurationContents := fmt.Sprintf(`
functionAugmentedConfigs:
- labelSelector:
    matchLabels:
      nuclio.io/class: function
  kubernetes:
    deployment:
      spec:
        minReadySeconds: %d
- functionConfig:
    spec:
      minReplicas: %d
      maxReplicas: %d
`, minReadySeconds, zero, ten)

	// read configuration
	err := suite.reader.Read(bytes.NewBufferString(configurationContents), "yaml", &readConfiguration)
	suite.Require().NoError(err)

	expectedFunctionAugmentedConfigs := []LabelSelectorAndConfig{
		{

			// all function matches `nuclio.io/class: function` should have deployment spec of MinReadySeconds: 90
			v1.LabelSelector{
				MatchLabels: map[string]string{
					"nuclio.io/class": "function",
				},
			},
			functionconfig.Config{},
			Kubernetes{
				Deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						MinReadySeconds: 90,
					},
				},
			},
		},
		{

			// set min replicas to 0 and max replicas to 10 for all functions
			v1.LabelSelector{},
			functionconfig.Config{
				Spec: functionconfig.Spec{MinReplicas: &zero, MaxReplicas: &ten},
			},
			Kubernetes{},
		},
	}

	suite.Require().Empty(cmp.Diff(&expectedFunctionAugmentedConfigs, &readConfiguration.FunctionAugmentedConfigs, cmpopts.IgnoreUnexported(Config{})))
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(PlatformConfigTestSuite))
}
