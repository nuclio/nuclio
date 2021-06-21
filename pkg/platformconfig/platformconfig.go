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

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/runtimeconfig"

	"github.com/nuclio/errors"
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

	functionReadinessTimeout, err := time.ParseDuration(*config.FunctionReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function readiness timeout")
	}
	config.functionReadinessTimeout = &functionReadinessTimeout

	return config, nil
}

func (config *Config) GetSystemLoggerSinks() (map[string]LoggerSinkWithLevel, error) {
	return config.getLoggerSinksWithLevel(config.Logger.System)
}

func (config *Config) GetFunctionLoggerSinks(functionConfig *functionconfig.Config) (map[string]LoggerSinkWithLevel, error) {
	var loggerSinkBindings []LoggerSinkBinding
	switch {

	// if user specified only one logger sink and did not specify its name, this is the way to specify the level
	// and use platform configuration
	case len(functionConfig.Spec.LoggerSinks) == 1 && functionConfig.Spec.LoggerSinks[0].Sink == "":
		for _, loggerSinkBinding := range config.Logger.Functions {
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
		loggerSinkBindings = config.Logger.Functions
	}

	return config.getLoggerSinksWithLevel(loggerSinkBindings)
}

func (config *Config) GetDefaultFunctionReadinessTimeout() time.Duration {

	// provided by the platform-config
	if config.functionReadinessTimeout != nil {
		return *config.functionReadinessTimeout
	}

	// no configuration were explicitly given, return default
	return DefaultFunctionReadinessTimeoutSeconds * time.Second
}

func (config *Config) GetSystemMetricSinks() (map[string]MetricSink, error) {
	return config.getMetricSinks(config.Metrics.System)
}

func (config *Config) GetFunctionMetricSinks() (map[string]MetricSink, error) {
	return config.getMetricSinks(config.Metrics.Functions)
}

func (config *Config) getMetricSinks(metricSinkNames []string) (map[string]MetricSink, error) {
	metricSinks := map[string]MetricSink{}

	for _, metricSinkName := range metricSinkNames {
		metricSink, metricSinkFound := config.Metrics.Sinks[metricSinkName]
		if !metricSinkFound {
			return nil, errors.Errorf("Failed to find metric sink %s", metricSinkName)
		}

		metricSinks[metricSinkName] = metricSink
	}

	return metricSinks, nil
}

func (config *Config) getLoggerSinksWithLevel(loggerSinkBindings []LoggerSinkBinding) (map[string]LoggerSinkWithLevel, error) {
	result := map[string]LoggerSinkWithLevel{}

	// iterate over system bindings, look for logger sink by name
	for _, sinkBinding := range loggerSinkBindings {

		// get sink by name
		sink, sinkFound := config.Logger.Sinks[sinkBinding.Sink]
		if !sinkFound {
			return nil, errors.Errorf("Failed to find logger sink %s", sinkBinding.Sink)
		}

		result[sinkBinding.Sink] = LoggerSinkWithLevel{
			Level: sinkBinding.Level,
			Sink:  sink,
		}
	}

	return result, nil
}

func (config *Config) enrichLocalPlatform() {

	// if set via envvar, override given configuration
	switch strings.ToLower(os.Getenv("NUCLIO_CHECK_FUNCTION_CONTAINERS_HEALTHINESS")) {
	case "false":
		config.Local.FunctionContainersHealthinessEnabled = false
	case "true":
		config.Local.FunctionContainersHealthinessEnabled = true
	}

	if config.Local.FunctionContainersHealthinessInterval == 0 {
		config.Local.FunctionContainersHealthinessInterval = time.Second * 30
	}

	if config.Local.FunctionContainersHealthinessTimeout == 0 {
		config.Local.FunctionContainersHealthinessTimeout = time.Second * 5
	}
}

func (config *Config) enrichOpaConfig() {
	if config.Opa.Address == "" {
		config.Opa.Address = "127.0.0.1:8181"
	}

	if config.Opa.ClientKind == "" {
		config.Opa.ClientKind = opa.DefaultClientKind
	}

	if config.Opa.RequestTimeout == 0 {
		config.Opa.RequestTimeout = opa.DefaultRequestTimeOut
	}

	if config.Opa.PermissionQueryPath == "" {
		config.Opa.PermissionQueryPath = opa.DefaultPermissionQueryPath
	}
}
