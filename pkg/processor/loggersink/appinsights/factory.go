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

package stdout

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/loggersink"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
	"github.com/nuclio/logger"
	"github.com/nuclio/logger-appinsights"
)

type factory struct{}

func (f *factory) Create(name string,
	loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) (logger.Logger, error) {

	configuration, err := NewConfiguration(name, loggerSinkConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create prometheus pull configuration")
	}

	// create telemetry client
	telemetryClientConfig := appinsights.NewTelemetryConfiguration(configuration.InstrumentationKey)
	telemetryClientConfig.MaxBatchInterval = configuration.parsedMaxBatchInterval
	telemetryClientConfig.MaxBatchSize = configuration.MaxBatchSize

	// create a telemetry client
	telemetryClient := appinsights.NewTelemetryClientFromConfig(telemetryClientConfig)

	return appinsightslogger.NewLogger(telemetryClient, "processor", configuration.Level)
}

// register factory
func init() {
	loggersink.RegistrySingleton.Register("appinsights", &factory{})
}
