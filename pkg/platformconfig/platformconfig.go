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
	"os"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"

	"github.com/nuclio/errors"
	"github.com/v3io/scaler/pkg/scalertypes"
)

type Config struct {
	Kind                     string                       `json:"kind,omitempty"`
	WebAdmin                 WebServer                    `json:"webAdmin,omitempty"`
	HealthCheck              WebServer                    `json:"healthCheck,omitempty"`
	Logger                   Logger                       `json:"logger,omitempty"`
	Metrics                  Metrics                      `json:"metrics,omitempty"`
	ScaleToZero              ScaleToZero                  `json:"scaleToZero,omitempty"`
	AutoScale                AutoScale                    `json:"autoScale,omitempty"`
	CronTriggerCreationMode  CronTriggerCreationMode      `json:"cronTriggerCreationMode,omitempty"`
	FunctionAugmentedConfigs []LabelSelectorAndConfig     `json:"functionAugmentedConfigs,omitempty"`
	FunctionReadinessTimeout *string                      `json:"functionReadinessTimeout,omitempty"`
	IngressConfig            IngressConfig                `json:"ingressConfig,omitempty"`
	Kube                     PlatformKubeConfig           `json:"kube,omitempty"`
	Local                    PlatformLocalConfig          `json:"local,omitempty"`
	ImageRegistryOverrides   ImageRegistryOverridesConfig `json:"imageRegistryOverrides,omitempty"`
	Runtime                  *runtimeconfig.Config        `json:"runtime,omitempty"`
	ProjectsLeader           *ProjectsLeader              `json:"projectsLeader,omitempty"`
	ManagedNamespaces        []string                     `json:"managedNamespaces,omitempty"`
	IguazioSessionCookie     string                       `json:"iguazioSessionCookie,omitempty"`
	Opa                      opa.Config                   `json:"opa,omitempty"`

	ContainerBuilderConfiguration *containerimagebuilderpusher.ContainerBuilderConfiguration `json:"containerBuilderConfiguration,omitempty"`

	// stores the encoded FunctionReadinessTimeout as time.Duration
	functionReadinessTimeout *time.Duration
}

func NewPlatformConfig(configurationPath string) (*Config, error) {

	// read or get default platform config
	platformConfigurationReader, err := NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform configuration reader")
	}

	config, err := platformConfigurationReader.ReadFileOrDefault(configurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read platform configuration")
	}

	// determine config kind
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) != 0 && len(os.Getenv("KUBERNETES_SERVICE_PORT")) != 0 {
		config.Kind = "kube"
	} else {
		config.Kind = "local"
	}

	// enrich opa configuration
	config.enrichOpaConfig()

	// enrich local platform configuration
	config.enrichLocalPlatform()

	// default cron trigger creation mode to processor
	// TODO: move under `config.Kube`
	if config.CronTriggerCreationMode == "" {
		config.CronTriggerCreationMode = ProcessorCronTriggerCreationMode
	}

	if config.Kube.DefaultServiceType == "" {
		config.Kube.DefaultServiceType = DefaultServiceType
	}

	if config.FunctionReadinessTimeout == nil {
		encodedReadinessTimeoutDuration := (DefaultFunctionReadinessTimeoutSeconds * time.Second).String()
		config.FunctionReadinessTimeout = &encodedReadinessTimeoutDuration
	}

	if config.ScaleToZero.MultiTargetStrategy == "" {
		config.ScaleToZero.MultiTargetStrategy = scalertypes.MultiTargetStrategyRandom
	}

	functionReadinessTimeout, err := time.ParseDuration(*config.FunctionReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function readiness timeout")
	}
	config.functionReadinessTimeout = &functionReadinessTimeout

	return config, nil
}

func (c *Config) GetSystemLoggerSinks() (map[string]LoggerSinkWithLevel, error) {
	return c.getLoggerSinksWithLevel(c.Logger.System)
}

func (c *Config) GetFunctionLoggerSinks(functionConfig *functionconfig.Config) (map[string]LoggerSinkWithLevel, error) {
	var loggerSinkBindings []LoggerSinkBinding
	switch {

	// if user specified only one logger sink and did not specify its name, this is the way to specify the level
	// and use platform configuration
	case len(functionConfig.Spec.LoggerSinks) == 1 && functionConfig.Spec.LoggerSinks[0].Sink == "":
		for _, loggerSinkBinding := range c.Logger.Functions {
			loggerSinkBindings = append(loggerSinkBindings, LoggerSinkBinding{
				Sink:  loggerSinkBinding.Sink,
				Level: functionConfig.Spec.LoggerSinks[0].Level,
			})
		}

	// if the function specifies logger sinks, use that. otherwise use the default platform-specified logger sinks
	case len(functionConfig.Spec.LoggerSinks) > 0:
		for _, loggerSink := range functionConfig.Spec.LoggerSinks {
			loggerSinkBindings = append(loggerSinkBindings, LoggerSinkBinding{
				Level: loggerSink.Level,
				Sink:  loggerSink.Sink,
			})
		}
	default:
		loggerSinkBindings = c.Logger.Functions
	}

	return c.getLoggerSinksWithLevel(loggerSinkBindings)
}

func (c *Config) GetDefaultFunctionReadinessTimeout() time.Duration {

	// provided by the platform-c
	if c.functionReadinessTimeout != nil {
		return *c.functionReadinessTimeout
	}

	// no configuration were explicitly given, return default
	return DefaultFunctionReadinessTimeoutSeconds * time.Second
}

func (c *Config) GetSystemMetricSinks() (map[string]MetricSink, error) {
	return c.getMetricSinks(c.Metrics.System)
}

func (c *Config) GetFunctionMetricSinks() (map[string]MetricSink, error) {
	return c.getMetricSinks(c.Metrics.Functions)
}

func (c *Config) getMetricSinks(metricSinkNames []string) (map[string]MetricSink, error) {
	metricSinks := map[string]MetricSink{}

	for _, metricSinkName := range metricSinkNames {
		metricSink, metricSinkFound := c.Metrics.Sinks[metricSinkName]
		if !metricSinkFound {
			return nil, errors.Errorf("Failed to find metric sink %s", metricSinkName)
		}

		metricSinks[metricSinkName] = metricSink
	}

	return metricSinks, nil
}

func (c *Config) getLoggerSinksWithLevel(loggerSinkBindings []LoggerSinkBinding) (map[string]LoggerSinkWithLevel, error) {
	LoggerSinksWithLevel := map[string]LoggerSinkWithLevel{}

	// iterate over system bindings, look for logger sink by name
	for _, sinkBinding := range loggerSinkBindings {

		// get sink by name
		sink, sinkFound := c.Logger.Sinks[sinkBinding.Sink]
		if !sinkFound {
			return nil, errors.Errorf("Failed to find logger sink %s", sinkBinding.Sink)
		}

		LoggerSinksWithLevel[sinkBinding.Sink] = LoggerSinkWithLevel{
			Level:    sinkBinding.Level,
			Sink:     sink,
			redactor: common.GetRedactorInstance(nil),
		}
	}

	return LoggerSinksWithLevel, nil
}

func (c *Config) enrichLocalPlatform() {

	// if set via envvar, override given configuration
	switch strings.ToLower(os.Getenv("NUCLIO_CHECK_FUNCTION_CONTAINERS_HEALTHINESS")) {
	case "false":
		c.Local.FunctionContainersHealthinessEnabled = false
	case "true":
		c.Local.FunctionContainersHealthinessEnabled = true
	}

	if c.Local.FunctionContainersHealthinessInterval == 0 {
		c.Local.FunctionContainersHealthinessInterval = time.Second * 30
	}

	if c.Local.FunctionContainersHealthinessTimeout == 0 {
		c.Local.FunctionContainersHealthinessTimeout = time.Second * 5
	}
}

func (c *Config) enrichOpaConfig() {
	if c.Opa.Address == "" {
		c.Opa.Address = "127.0.0.1:8181"
	}

	if c.Opa.ClientKind == "" {
		c.Opa.ClientKind = opa.DefaultClientKind
	}

	if c.Opa.RequestTimeout == 0 {
		c.Opa.RequestTimeout = opa.DefaultRequestTimeOut
	}

	if c.Opa.PermissionQueryPath == "" {
		c.Opa.PermissionQueryPath = opa.DefaultPermissionQueryPath
	}

	if c.Opa.PermissionFilterPath == "" {
		c.Opa.PermissionFilterPath = opa.DefaultPermissionFilterPath
	}
}
