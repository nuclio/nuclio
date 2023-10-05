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

package platformconfig

import (
	"context"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg/scalertypes"
	autosv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	Kind                      string                           `json:"kind,omitempty"`
	WebAdmin                  WebServer                        `json:"webAdmin,omitempty"`
	HealthCheck               WebServer                        `json:"healthCheck,omitempty"`
	Logger                    Logger                           `json:"logger,omitempty"`
	Metrics                   Metrics                          `json:"metrics,omitempty"`
	ScaleToZero               ScaleToZero                      `json:"scaleToZero,omitempty"`
	AutoScale                 AutoScale                        `json:"autoScale,omitempty"`
	SupportedAutoScaleMetrics []functionconfig.AutoScaleMetric `json:"supportedAutoScaleMetrics,omitempty"`
	AutoScaleMetricsMode      AutoScaleMetricsMode             `json:"autoScaleMetricsMode,omitempty"`
	CronTriggerCreationMode   CronTriggerCreationMode          `json:"cronTriggerCreationMode,omitempty"`
	FunctionAugmentedConfigs  []LabelSelectorAndConfig         `json:"functionAugmentedConfigs,omitempty"`
	FunctionReadinessTimeout  *string                          `json:"functionReadinessTimeout,omitempty"`
	FunctionInvocationTimeout *string                          `json:"functionInvocationTimeout,omitempty"`
	IngressConfig             IngressConfig                    `json:"ingressConfig,omitempty"`
	Kube                      PlatformKubeConfig               `json:"kube,omitempty"`
	Local                     PlatformLocalConfig              `json:"local,omitempty"`
	ImageRegistryOverrides    ImageRegistryOverridesConfig     `json:"imageRegistryOverrides,omitempty"`
	Runtime                   *runtimeconfig.Config            `json:"runtime,omitempty"`
	ProjectsLeader            *ProjectsLeader                  `json:"projectsLeader,omitempty"`
	ManagedNamespaces         []string                         `json:"managedNamespaces,omitempty"`
	IguazioSessionCookie      string                           `json:"iguazioSessionCookie,omitempty"`
	Opa                       opa.Config                       `json:"opa,omitempty"`
	StreamMonitoring          StreamMonitoringConfig           `json:"streamMonitoring,omitempty"`
	SensitiveFields           SensitiveFieldsConfig            `json:"sensitiveFields,omitempty"`

	ContainerBuilderConfiguration *containerimagebuilderpusher.ContainerBuilderConfiguration `json:"containerBuilderConfiguration,omitempty"`

	// stores the encoded FunctionReadinessTimeout as time.Duration
	functionReadinessTimeout *time.Duration

	// stores the encoded FunctionInvocationTimeout as time.Duration
	functionInvocationTimeout *time.Duration
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
		config.Kind = common.KubePlatformName
	} else {
		config.Kind = common.LocalPlatformName
	}

	// enrich opa configuration
	config.enrichOpaConfig()

	// enrich local platform configuration
	config.enrichLocalPlatform()

	if config.Logger.Sinks == nil {
		config.Logger.Sinks = platformConfigurationReader.GetDefaultConfiguration().Logger.Sinks
	}
	if config.Logger.Functions == nil {
		config.Logger.Functions = platformConfigurationReader.GetDefaultConfiguration().Logger.Functions
	}
	if config.Logger.System == nil {
		config.Logger.System = platformConfigurationReader.GetDefaultConfiguration().Logger.System
	}

	// default cron trigger creation mode to processor
	// TODO: move under `config.Kube`
	if config.CronTriggerCreationMode == "" {
		config.CronTriggerCreationMode = ProcessorCronTriggerCreationMode
	}

	if config.Kube.DefaultServiceType == "" {
		config.Kube.DefaultServiceType = DefaultServiceType
	}

	if config.Kube.PreemptibleNodes != nil {
		if config.Kube.PreemptibleNodes.DefaultMode == "" {
			config.Kube.PreemptibleNodes.DefaultMode = functionconfig.RunOnPreemptibleNodesPrevent
		}
	}

	if config.FunctionReadinessTimeout == nil {
		encodedReadinessTimeoutDuration := (DefaultFunctionReadinessTimeoutSeconds * time.Second).String()
		config.FunctionReadinessTimeout = &encodedReadinessTimeoutDuration
	}

	if config.FunctionInvocationTimeout == nil {
		encodedInvocationTimeoutDuration := (DefaultFunctionInvocationTimeoutSeconds * time.Second).String()
		config.FunctionInvocationTimeout = &encodedInvocationTimeoutDuration
	}

	if config.ScaleToZero.MultiTargetStrategy == "" {
		config.ScaleToZero.MultiTargetStrategy = scalertypes.MultiTargetStrategyRandom
	}

	// fall back to legacy default
	if !AutoScaleMetricsModeIsValid(config.AutoScaleMetricsMode) {
		config.AutoScaleMetricsMode = AutoScaleMetricsModeLegacy
	}

	if config.StreamMonitoring.WebapiURL == "" {
		config.StreamMonitoring.WebapiURL = DefaultStreamMonitoringWebapiURL
	}

	if config.StreamMonitoring.V3ioRequestConcurrency == 0 {
		config.StreamMonitoring.V3ioRequestConcurrency = DefaultV3ioRequestConcurrency
	}

	functionReadinessTimeout, err := time.ParseDuration(*config.FunctionReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function readiness timeout")
	}
	config.functionReadinessTimeout = &functionReadinessTimeout

	functionInvocationTimeout, err := time.ParseDuration(*config.FunctionInvocationTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function readiness timeout")
	}
	config.functionInvocationTimeout = &functionInvocationTimeout

	config.SensitiveFields.CompileSensitiveFieldsRegex()

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

func (c *Config) GetDefaultFunctionInvocationTimeout() time.Duration {

	// provided by the platform-c
	if c.functionInvocationTimeout != nil {
		return *c.functionInvocationTimeout
	}

	// no configuration were explicitly given, return default
	return DefaultFunctionInvocationTimeoutSeconds * time.Second
}

func (c *Config) GetFunctionReadinessTimeoutOrDefault(functionReadinessTimeoutSeconds int) int {
	if functionReadinessTimeoutSeconds == 0 {
		return int(c.GetDefaultFunctionReadinessTimeout().Seconds())
	}
	return functionReadinessTimeoutSeconds
}

func (c *Config) GetSystemMetricSinks() (map[string]MetricSink, error) {
	return c.getMetricSinks(c.Metrics.System)
}

func (c *Config) GetFunctionMetricSinks() (map[string]MetricSink, error) {
	return c.getMetricSinks(c.Metrics.Functions)
}

func (c *Config) GetDefaultSupportedAutoScaleMetrics() []functionconfig.AutoScaleMetric {
	return []functionconfig.AutoScaleMetric{

		// Resource metrics
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: string(v1.ResourceCPU),
			},
			SourceType:  autosv2.ResourceMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypePercentage,
		},
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: string(v1.ResourceMemory),
			},
			SourceType:  autosv2.ResourceMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypePercentage,
		},
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: "gpu",
			},
			SourceType:  autosv2.PodsMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypePercentage,
		},

		// Stream metrics
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: "nuclio_processor_stream_high_water_mark_processed_lag",
			},
			SourceType:  autosv2.ExternalMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypeInt,
		},
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: "nuclio_processor_stream_high_water_mark_committed_lag",
			},
			SourceType:  autosv2.ExternalMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypeInt,
		},

		// Event metrics
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: "nuclio_processor_worker_pending_allocation_current",
			},
			SourceType:  autosv2.ExternalMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypeInt,
		},
		{
			ScaleResource: functionconfig.ScaleResource{
				MetricName: "nuclio_processor_worker_allocation_wait_duration_ms_sum",
			},
			SourceType:  autosv2.ExternalMetricSourceType,
			DisplayType: functionconfig.AutoScaleMetricTypeInt,
		},
	}
}

func (c *Config) GetDefaultWindowSizePresets() []string {
	return []string{
		"1m",
		"2m",
		"5m",
		"10m",
		"30m",
	}
}

// EnrichContainerResources enriches an object's requests and limits with the default
// resources defined in the platform config, only if they are not already configured
func (c *Config) EnrichContainerResources(ctx context.Context,
	logger logger.Logger,
	resources *v1.ResourceRequirements,
	isSideCar bool) {

	defaultContainerResources := c.Kube.DefaultFunctionPodResources
	if isSideCar {
		defaultContainerResources = c.Kube.DefaultSidecarResources
	}

	logger.DebugWithCtx(ctx,
		"Populating resources with default values",
		"defaultContainerResources", defaultContainerResources)

	if resources.Requests == nil {
		resources.Requests = make(v1.ResourceList)
	}

	if cpuRequest, exists := resources.Requests["cpu"]; !exists || cpuRequest.IsZero() {
		resources.Requests["cpu"] = common.ParseQuantityOrDefault(defaultContainerResources.Requests.CPU,
			"25m",
			logger)
	}
	if memoryRequest, exists := resources.Requests["memory"]; !exists || memoryRequest.IsZero() {
		resources.Requests["memory"] = common.ParseQuantityOrDefault(defaultContainerResources.Requests.Memory,
			"1Mi",
			logger)
	}

	// only set limits if this is not a sidecar
	if !isSideCar {
		if resources.Limits == nil {
			resources.Limits = make(v1.ResourceList)
		}
		if cpuLimit, exists := resources.Limits["cpu"]; !exists || cpuLimit.IsZero() {
			cpuQuantity, err := apiresource.ParseQuantity(defaultContainerResources.Limits.CPU)
			if err == nil {
				resources.Limits["cpu"] = cpuQuantity
			}
		}
		if memoryLimit, exists := resources.Limits["memory"]; !exists || memoryLimit.IsZero() {
			memoryQuantity, err := apiresource.ParseQuantity(defaultContainerResources.Limits.Memory)
			if err == nil {
				resources.Limits["memory"] = memoryQuantity
			}
		}
	}

	logger.DebugWithCtx(ctx,
		"Populated resources with default values",
		"resources", resources)
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
	c.Local.FunctionContainersHealthinessEnabled = common.GetEnvOrDefaultBool(
		"NUCLIO_CHECK_FUNCTION_CONTAINERS_HEALTHINESS", true)

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

func (c *Config) EnableSensitiveFieldMasking() {
	c.SensitiveFields.MaskSensitiveFields = true
}

func (c *Config) DisableSensitiveFieldMasking() {
	c.SensitiveFields.MaskSensitiveFields = false
}
