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

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/nuclio-sdk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spf13/viper"
)

type triggerProvider interface {
	GetTriggers() []trigger.Trigger
}

type MetricPusher struct {
	logger              nuclio.Logger
	metricRegistry      *prometheus.Registry
	jobName             string
	instanceName        string
	pushGatewayURL      string
	pushIntervalSeconds int
	gatherers           []Gatherer
	enabled             bool
}

func NewMetricPusher(parentLogger nuclio.Logger,
	triggerProvider triggerProvider,
	configuration *viper.Viper) (*MetricPusher, error) {

	newMetricPusher := &MetricPusher{
		logger:         parentLogger.GetChild("metrics"),
		metricRegistry: prometheus.NewRegistry(),
	}

	// read configuration
	if err := newMetricPusher.readConfiguration(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPusher.createGatherers(triggerProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to register metrics")
	}

	newMetricPusher.logger.InfoWith("Metrics pusher created",
		"enabeld", newMetricPusher.enabled,
		"jobName", newMetricPusher.jobName,
		"instanceName", newMetricPusher.instanceName,
		"pushGatewayURL", newMetricPusher.pushGatewayURL,
		"pushInterval", newMetricPusher.pushIntervalSeconds)

	return newMetricPusher, nil
}

func (mp *MetricPusher) Start() error {
	if !mp.enabled {
		mp.logger.InfoWith("Disabled, will not push metrics")
		return nil
	}

	go mp.periodicallyPushMetrics()

	return nil
}

func (mp *MetricPusher) readConfiguration(configuration *viper.Viper) error {
	pushGatewayURLDefault := os.Getenv("NUCLIO_PROM_PUSH_GATEWAY_URL")
	if pushGatewayURLDefault == "" {
		pushGatewayURLDefault = "http://prometheus-prometheus-pushgateway:9091"
	}

	pushInterval := os.Getenv("NUCLIO_PROM_PUSH_INTERVAL")
	if pushInterval == "" {
		pushInterval = "10"
	}

	configuration.SetDefault("job_name", os.Getenv("NUCLIO_FUNCTION_NAME"))
	configuration.SetDefault("instance", os.Getenv("NUCLIO_FUNCTION_INSTANCE"))
	configuration.SetDefault("push_gateway_url", pushGatewayURLDefault)
	configuration.SetDefault("push_interval", pushInterval)
	configuration.SetDefault("enabled", true)

	mp.enabled = configuration.GetBool("enabled")
	mp.pushGatewayURL = configuration.GetString("push_gateway_url")
	mp.jobName = configuration.GetString("job_name")
	mp.instanceName = configuration.GetString("instance")
	mp.pushIntervalSeconds = configuration.GetInt("push_interval")

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
		time.Sleep(time.Duration(mp.pushIntervalSeconds) * time.Second)

		// gather the metrics from the triggers - this will update the metrics
		// from counters internally held by triggers and their child objects
		mp.gather()

		// AddFromGatherer is used here rather than FromGatherer to not delete a
		// previously pushed success timestamp in case of a failure of this
		// backup.
		push.AddFromGatherer(mp.jobName, nil, mp.pushGatewayURL, mp.metricRegistry)

		// TODO: log a warning here when prometheus is configured via a platform configuration
		// mp.logger.WarnWith("Failed to push metrics", "err", err)
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
