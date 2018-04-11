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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"
	"github.com/nuclio/nuclio/pkg/processor/metricsink/prometheus"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
	"github.com/nuclio/logger"
)

type MetricSink struct {
	*metricsink.AbstractMetricSink
	configuration *Configuration
	gatherers     []prometheus.Gatherer
	client        appinsights.TelemetryClient
}

func newMetricSink(parentLogger logger.Logger,
	configuration *Configuration,
	metricProvider metricsink.MetricProvider) (*MetricSink, error) {
	loggerInstance := parentLogger.GetChild(configuration.Name)

	newAbstractMetricSink, err := metricsink.NewAbstractMetricSink(loggerInstance,
		"appinsights",
		configuration.Name,
		metricProvider)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract metric sink")
	}

	// create application insights client
	telemetryConfig := appinsights.NewTelemetryConfiguration(configuration.InstrumentationKey)
	telemetryConfig.MaxBatchSize = configuration.MaxBatchSize
	telemetryConfig.MaxBatchInterval = configuration.parsedMaxBatchInterval
	client := appinsights.NewTelemetryClientFromConfig(telemetryConfig)

	newMetricPuller := &MetricSink{
		AbstractMetricSink: newAbstractMetricSink,
		configuration:      configuration,
		client:             client,
	}

	// create a bunch of gatherer
	if err := newMetricPuller.createGatherers(metricProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to create gatherers")
	}

	newMetricPuller.Logger.InfoWith("Created")

	return newMetricPuller, nil
}

func (ms *MetricSink) Start() error {
	if !*ms.configuration.Enabled {
		ms.Logger.DebugWith("Disabled, not starting")

		return nil
	}

	// set when stop() is called and channel is closed
	done := false
	defer close(ms.StoppedChannel)

	ms.Logger.DebugWith("Pushing periodically",
		"interval", ms.configuration.parsedInterval)

	for !done {

		select {
		case <-time.After(ms.configuration.parsedInterval):

			// gather the metrics from the triggers - this will update the metrics
			// from counters internally held by triggers and their child objects
			if err := ms.gather(); err != nil {
				return errors.Wrap(err, "Failed to gather")
			}

		case <-ms.StopChannel:
			done = true
		}
	}

	return nil
}

func (ms *MetricSink) Stop() chan struct{} {

	// call parent
	return ms.AbstractMetricSink.Stop()
}

func (ms *MetricSink) createGatherers(metricProvider metricsink.MetricProvider) error {

	for _, trigger := range metricProvider.GetTriggers() {

		// create a gatherer for the trigger
		triggerGatherer, err := newTriggerGatherer(trigger, ms.client)

		if err != nil {
			return errors.Wrap(err, "Failed to create trigger gatherer")
		}

		ms.gatherers = append(ms.gatherers, triggerGatherer)

		// now add workers
		for _, worker := range trigger.GetWorkers() {
			workerGatherer, err := newWorkerGatherer(trigger, worker, ms.client)

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
