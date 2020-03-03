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
	"bytes"
	"context"
	"net/http"
	"os"
	"sync"
	"text/template"

	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"
	"github.com/nuclio/nuclio/pkg/processor/metricsink/prometheus"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricSink struct {
	*metricsink.AbstractMetricSink
	configuration         *Configuration
	metricRegistry        *prometheusclient.Registry
	metricRegistryHandler http.Handler
	gatherers             []prometheus.Gatherer
	httpServer            *http.Server
	instanceName          string
	gatherLock            sync.Locker
}

func newMetricSink(parentLogger logger.Logger,
	processorConfiguration *processor.Configuration,
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
		gatherLock:         &sync.Mutex{},
	}

	newMetricPuller.instanceName, err = newMetricPuller.getInstanceName(processorConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get instance name")
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPuller.createGatherers(metricProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to create gatherers")
	}

	newMetricPuller.Logger.InfoWith("Created",
		"env", os.Getenv("NUCLIO_FUNCTION_INSTANCE"),
		"instanceName", newMetricPuller.instanceName,
		"listenAddr", configuration.URL,
		"gatherers", len(newMetricPuller.gatherers))

	return newMetricPuller, nil
}

func (ms *MetricSink) Start() error {
	if !*ms.configuration.Enabled {
		ms.Logger.DebugWith("Disabled, not starting")

		return nil
	}

	// create server so that we can stop it
	ms.httpServer = &http.Server{Addr: ms.configuration.URL, Handler: nil}

	// listen in the background
	go ms.listen() // nolint: errcheck

	return nil
}

func (ms *MetricSink) Stop() chan struct{} {

	// shut down the server if we created it
	if ms.httpServer != nil {
		ms.httpServer.Shutdown(context.TODO()) // nolint: errcheck
	}

	// call parent
	return ms.AbstractMetricSink.Stop()
}

func (ms *MetricSink) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {

	// gather all metrics from the processor. reads all the primitive data into the prometheus counters
	if err := ms.gather(); err != nil {
		ms.Logger.WarnWith("Failure detected while gathering metrics", "err", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	// proxy to the registry handler
	ms.metricRegistryHandler.ServeHTTP(responseWriter, request)
}

func (ms *MetricSink) listen() error {
	ms.Logger.DebugWith("Listening", "addr", ms.configuration.URL)

	// save the handler that the registry provides
	ms.metricRegistryHandler = promhttp.HandlerFor(ms.metricRegistry, promhttp.HandlerOpts{})

	// register ourselves as the handler. we wrap ms.metricRegistryHandler
	http.Handle("/metrics", ms)

	// start listening
	if err := ms.httpServer.ListenAndServe(); err != nil {
		return errors.Wrapf(err, "Failed to listen on %s", ms.configuration.URL)
	}

	return nil
}

func (ms *MetricSink) createGatherers(metricProvider metricsink.MetricProvider) error {

	for _, trigger := range metricProvider.GetTriggers() {

		// create a gatherer for the trigger
		triggerGatherer, err := prometheus.NewTriggerGatherer(ms.instanceName,
			trigger,
			ms.Logger,
			ms.metricRegistry)

		if err != nil {
			return errors.Wrap(err, "Failed to create trigger gatherer")
		}

		ms.gatherers = append(ms.gatherers, triggerGatherer)

		// now add workers
		for _, worker := range trigger.GetWorkers() {
			workerGatherer, err := prometheus.NewWorkerGatherer(ms.instanceName,
				trigger,
				ms.Logger,
				worker,
				ms.metricRegistry)

			if err != nil {
				return errors.Wrap(err, "Failed to create worker gatherer")
			}

			ms.gatherers = append(ms.gatherers, workerGatherer)
		}
	}

	ms.Logger.DebugWith("Created trigger and worker gatherers")

	return nil
}

func (ms *MetricSink) gather() error {

	// protect against concurrent gatherings, trigger and worker diffs are not atomic (swapping cur <-> prev)
	ms.gatherLock.Lock()
	defer ms.gatherLock.Unlock()

	for _, gatherer := range ms.gatherers {
		if err := gatherer.Gather(); err != nil {
			return err
		}
	}

	return nil
}

func (ms *MetricSink) getInstanceName(processorConfiguration *processor.Configuration) (string, error) {
	instanceNameTemplate, err := template.New("instanceName").Parse(ms.configuration.InstanceName)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create instanceName template")
	}

	var instanceNameTemplateBuffer bytes.Buffer
	err = instanceNameTemplate.Execute(&instanceNameTemplateBuffer, &map[string]interface{}{
		"Namespace": processorConfiguration.Config.Meta.Namespace,
		"Name":      processorConfiguration.Config.Meta.Name,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to execute instanceName template")
	}

	return instanceNameTemplateBuffer.String(), nil

}
