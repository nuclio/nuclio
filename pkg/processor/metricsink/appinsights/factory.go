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

package appinsights

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"

	"github.com/nuclio/logger"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	name string,
	metricSinkConfiguration *platformconfig.MetricSink,
	metricProvider metricsink.MetricProvider) (metricsink.MetricSink, error) {

	// create logger
	appinsightsLogger := parentLogger.GetChild("appinsights")

	configuration, err := NewConfiguration(name, metricSinkConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create prometheus pull configuration")
	}

	// create the metric sink
	appinsightsMetricSink, err := newMetricSink(appinsightsLogger, configuration, metricProvider)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create prometheus pull metric sink")
	}

	return appinsightsMetricSink, nil
}

// register factory
func init() {
	metricsink.RegistrySingleton.Register("appinsights", &factory{})
}
