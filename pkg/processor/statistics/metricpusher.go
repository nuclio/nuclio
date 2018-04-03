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

package statistics

import (
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type triggerProvider interface {
	GetTriggers() []trigger.Trigger
}

type MetricPusher struct {
	logger         logger.Logger
	metricRegistry *prometheus.Registry
	jobName        string
	instanceName   string
	pushGatewayURL string
	pushInterval   time.Duration
	gatherers      []Gatherer
}

func NewMetricPusher(parentLogger logger.Logger,
	triggerProvider triggerProvider,
	metricSinkConfiguration *platformconfig.MetricSink) (*MetricPusher, error) {

	newMetricPusher := &MetricPusher{
		logger:         parentLogger.GetChild("metrics"),
		metricRegistry: prometheus.NewRegistry(),
	}

	// read configuration
	if err := newMetricPusher.readConfiguration(metricSinkConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPusher.createGatherers(triggerProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to register metrics")
	}

	newMetricPusher.logger.InfoWith("Metrics pusher created",
		"jobName", newMetricPusher.jobName,
		"instanceName", newMetricPusher.instanceName,
		"pushGatewayURL", newMetricPusher.pushGatewayURL,
		"pushInterval", newMetricPusher.pushInterval)

	return newMetricPusher, nil
}

func (mp *MetricPusher) Start() error {
	go mp.periodicallyPushMetrics()

	return nil
}

func (mp *MetricPusher) readConfiguration(metricSinkConfiguration *platformconfig.MetricSink) error {
	var err error
	mp.pushGatewayURL = metricSinkConfiguration.URL

	intervalString := common.MapStringInterfaceGetOrDefault(metricSinkConfiguration.Attributes,
		"interval",
		"10s").(string)

	mp.pushInterval, err = time.ParseDuration(intervalString)
	if err != nil {
		return errors.Wrap(err, "Failed to parse metric interval")
	}

	mp.jobName = common.MapStringInterfaceGetOrDefault(metricSinkConfiguration.Attributes,
		"jobName",
		os.Getenv("NUCLIO_FUNCTION_NAME")).(string)

	mp.instanceName = common.MapStringInterfaceGetOrDefault(metricSinkConfiguration.Attributes,
		"instance",
		os.Getenv("NUCLIO_FUNCTION_INSTANCE")).(string)

	return nil
}

func (mp *MetricPusher) createGatherers(triggerProvider triggerProvider) error {

	for _, trigger := range triggerProvider.GetTriggers() {

		// create a gatherer for the trigger
		triggerGatherer, err := newTriggerGatherer(mp.instanceName, trigger, mp.metricRegistry)
		if err != nil {
			return errors.Wrap(err, "Failed to create trigger gatherer")
		}

		mp.gatherers = append(mp.gatherers, triggerGatherer)

		// now add workers
		for _, worker := range trigger.GetWorkers() {
			workerGatherer, err := newWorkerGatherer(mp.instanceName, trigger, worker, mp.metricRegistry)
			if err != nil {
				return errors.Wrap(err, "Failed to create worker gatherer")
			}

			mp.gatherers = append(mp.gatherers, workerGatherer)
		}
	}

	return nil
}

func (mp *MetricPusher) periodicallyPushMetrics() {

	for {

		// every mp.pushInterval seconds
		time.Sleep(mp.pushInterval)

		// gather the metrics from the triggers - this will update the metrics
		// from counters internally held by triggers and their child objects
		if err := mp.gather(); err != nil {
			mp.logger.WarnWith("Failed to gather metrics", "err", err)
		}

		// AddFromGatherer is used here rather than FromGatherer to not delete a
		// previously pushed success timestamp in case of a failure of this
		// backup.
		if err := push.AddFromGatherer(mp.jobName, nil, mp.pushGatewayURL, mp.metricRegistry); err != nil {
			mp.logger.WarnWith("Failed to push metrics", "err", err)
		}
	}
}

func (mp *MetricPusher) gather() error {

	for _, gatherer := range mp.gatherers {
		if err := gatherer.Gather(); err != nil {
			return err
		}
	}

	return nil
}
