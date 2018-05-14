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
	"fmt"

	"github.com/nuclio/nuclio/pkg/functionconfig"
)

type Configuration struct {
	Kind        string    `json:"kind,omitempty"`
	WebAdmin    WebServer `json:"webAdmin,omitempty"`
	HealthCheck WebServer `json:"healthCheck,omitempty"`
	Logger      Logger    `json:"logger,omitempty"`
	Metrics     Metrics   `json:"metrics,omitempty"`
}

func (config *Configuration) GetSystemLoggerSinks() (map[string]LoggerSinkWithLevel, error) {
	return config.getLoggerSinksWithLevel(config.Logger.System)
}

func (config *Configuration) GetFunctionLoggerSinks(functionConfig *functionconfig.Config) (map[string]LoggerSinkWithLevel, error) {
	var loggerSinkBindings []LoggerSinkBinding

	// if the function specifies logger sinks, use that. otherwise use the default platform-specified logger sinks
	if len(functionConfig.Spec.LoggerSinks) > 0 {
		for _, loggerSink := range functionConfig.Spec.LoggerSinks {
			loggerSinkBindings = append(loggerSinkBindings, LoggerSinkBinding{
				Level: loggerSink.Level,
				Sink:  loggerSink.Sink,
			})
		}
	} else {
		loggerSinkBindings = config.Logger.Functions
	}

	return config.getLoggerSinksWithLevel(loggerSinkBindings)
}

func (config *Configuration) GetSystemMetricSinks() (map[string]MetricSink, error) {
	return config.getMetricSinks(config.Metrics.System)
}

func (config *Configuration) GetFunctionMetricSinks() (map[string]MetricSink, error) {
	return config.getMetricSinks(config.Metrics.Functions)
}

func (config *Configuration) getMetricSinks(metricSinkNames []string) (map[string]MetricSink, error) {
	metricSinks := map[string]MetricSink{}

	for _, metricSinkName := range metricSinkNames {
		metricSink, metricSinkFound := config.Metrics.Sinks[metricSinkName]
		if !metricSinkFound {
			return nil, fmt.Errorf("Failed to find metric sink %s", metricSinkName)
		}

		metricSinks[metricSinkName] = metricSink
	}

	return metricSinks, nil
}

func (config *Configuration) getLoggerSinksWithLevel(loggerSinkBindings []LoggerSinkBinding) (map[string]LoggerSinkWithLevel, error) {
	result := map[string]LoggerSinkWithLevel{}

	// iterate over system bindings, look for logger sink by name
	for _, sinkBinding := range loggerSinkBindings {

		// get sink by name
		sink, sinkFound := config.Logger.Sinks[sinkBinding.Sink]
		if !sinkFound {
			return nil, fmt.Errorf("Failed to find logger sink %s", sinkBinding.Sink)
		}

		result[sinkBinding.Sink] = LoggerSinkWithLevel{
			Level: sinkBinding.Level,
			Sink:  sink,
		}
	}

	return result, nil
}
