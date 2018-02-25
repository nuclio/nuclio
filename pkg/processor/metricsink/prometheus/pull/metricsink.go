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

package prometheuspull

import (
	"net/http"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"
	"github.com/nuclio/nuclio/pkg/processor/metricsink/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	prometheusclient "github.com/prometheus/client_golang/prometheus"
)

type MetricSink struct {
	*metricsink.AbstractMetricSink
	configuration  *Configuration
	metricRegistry *prometheusclient.Registry
	gatherers      []prometheus.Gatherer
}

func newMetricSink(parentLogger logger.Logger,
	configuration *Configuration,
	metricProvider metricsink.MetricProvider) (*MetricSink, error) {
	loggerInstance := parentLogger.GetChild(configuration.Name)

	newAbstractMetricSink, err := metricsink.NewAbstractMetricSink(loggerInstance,
		"promethuesPull",
		configuration.Name,
		metricProvider)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract metric sink")
	}

	newMetricPuller := &MetricSink{
		AbstractMetricSink: newAbstractMetricSink,
		configuration:      configuration,
		metricRegistry:     prometheusclient.NewRegistry(),
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPuller.createGatherers(metricProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to create gatherers")
	}

	newMetricPuller.Logger.InfoWith("Created",
		"jobName", configuration.JobName,
		"instanceName", configuration.InstanceName,
		"listenAddr", configuration.URL)

	return newMetricPuller, nil
}

func (ms *MetricSink) Start() error {
	if !*ms.configuration.Enabled {
		ms.Logger.DebugWith("Disabled, not starting")

		return nil
	}

	ms.Logger.DebugWith("Starting")

	// register a handler for metrics
	http.Handle("/metrics", promhttp.HandlerFor(ms.metricRegistry, promhttp.HandlerOpts{}))

	// start listening
	if err := http.ListenAndServe(ms.configuration.URL, nil); err != nil {
		return errors.Wrapf(err, "Failed to listen on %s", ms.configuration.URL)
	}

	return nil
}

func (ms *MetricSink) createGatherers(metricProvider metricsink.MetricProvider) error {

	for _, trigger := range metricProvider.GetTriggers() {

		// create a gatherer for the trigger
		triggerGatherer, err := prometheus.NewTriggerGatherer(ms.configuration.InstanceName,
			trigger,
			ms.metricRegistry)

		if err != nil {
			return errors.Wrap(err, "Failed to create trigger gatherer")
		}

		ms.gatherers = append(ms.gatherers, triggerGatherer)

		// now add workers
		for _, worker := range trigger.GetWorkers() {
			workerGatherer, err := prometheus.NewWorkerGatherer(ms.configuration.InstanceName,
				trigger,
				worker,
				ms.metricRegistry)

			if err != nil {
				return errors.Wrap(err, "Failed to create worker gatherer")
			}

			ms.gatherers = append(ms.gatherers, workerGatherer)
		}
	}

	return nil
}

func (ms *MetricSink) gather() error {

	for _, gatherer := range ms.gatherers {
		if err := gatherer.Gather(); err != nil {
			return err
		}
	}

	return nil
}
