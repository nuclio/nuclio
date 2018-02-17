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

package app

import (
	"os"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	// load all data bindings
	_ "github.com/nuclio/nuclio/pkg/processor/databinding/eventhub"
	_ "github.com/nuclio/nuclio/pkg/processor/databinding/v3io"
	"github.com/nuclio/nuclio/pkg/processor/healthcheck"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	// load all runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/nodejs"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/statistics"
	"github.com/nuclio/nuclio/pkg/processor/status"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	// load all triggers
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/eventhubs"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/http"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kafka"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kinesis"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/nats"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/rabbitmq"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

// Processor is responsible to process events
type Processor struct {
	logger            logger.Logger
	functionLogger    logger.Logger
	triggers          []trigger.Trigger
	webAdminServer    *webadmin.Server
	healthCheckServer *healthcheck.Server
	metricsPushers    []*statistics.MetricPusher
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string, platformConfigurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{}

	// read platform configuration
	platformConfiguration, platformConfigurationFileRead, err := newProcessor.readPlatformConfiguration(platformConfigurationPath)
	if err != nil {
		return nil, errors.New("Failed to read platform configuration")
	}

	// create loggers for both the processor and the function invoked by the processor - they may
	// be headed to two different places
	newProcessor.logger,
		newProcessor.functionLogger,
		err = newProcessor.createLoggers(platformConfiguration)

	if err != nil {
		return nil, errors.New("Failed to create logger")
	}

	// log whether we're running a default configuration
	if !platformConfigurationFileRead {
		newProcessor.logger.WarnWith("Platform configuration not found, using defaults", "path", platformConfigurationPath)
	}

	processorConfiguration, err := newProcessor.readConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// create and start the health check server before creating anything else, so it can serve probes ASAP
	newProcessor.healthCheckServer, err = newProcessor.createAndStartHealthCheckServer(platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create and start health check server")
	}

	// create triggers
	newProcessor.triggers, err = newProcessor.createTriggers(processorConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create triggers")
	}

	// create the web interface
	newProcessor.webAdminServer, err = newProcessor.createWebAdminServer(platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create web interface server")
	}

	// create metric pusher
	newProcessor.metricsPushers, err = newProcessor.createMetricPushers(platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create metric pusher")
	}

	return newProcessor, nil
}

// Start starts the processor
func (p *Processor) Start() error {

	// iterate over all triggers and start them
	for _, trigger := range p.triggers {
		trigger.Start(nil)
	}

	// start the web interface
	err := p.webAdminServer.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start web interface")
	}

	// start pushing metrics
	for _, metricPusher := range p.metricsPushers {
		err := metricPusher.Start()
		if err != nil {
			return errors.Wrap(err, "Failed to start metric pushing")
		}
	}

	// TODO: shutdown
	select {}
}

// get triggers
func (p *Processor) GetTriggers() []trigger.Trigger {
	return p.triggers
}

// get workers
func (p *Processor) GetWorkers() []*worker.Worker {
	var workers []*worker.Worker

	// iterate over the processor's triggers
	for _, trigger := range p.triggers {

		workers = append(workers, trigger.GetWorkers()...)
	}

	return workers
}

// returns the processor's status based on its workers' readiness
func (p *Processor) GetStatus() status.Status {
	workers := p.GetWorkers()

	// if no workers exist yet, return initializing
	if len(workers) == 0 {
		return status.Initializing
	}

	// if any worker isn't ready yet, return initializing
	for _, worker := range workers {
		if worker.GetStatus() != status.Ready {
			return status.Initializing
		}
	}

	// otherwise we're ready
	return status.Ready
}

func (p *Processor) readConfiguration(configurationPath string) (*processor.Configuration, error) {
	var processorConfiguration processor.Configuration

	processorConfigurationReader, err := processorconfig.NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration file reader")
	}

	functionconfigFile, err := os.Open(configurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open configuration file")
	}

	if err := processorConfigurationReader.Read(functionconfigFile, &processorConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to open configuration file")
	}

	return &processorConfiguration, nil
}

func (p *Processor) readPlatformConfiguration(configurationPath string) (*platformconfig.Configuration, bool, error) {
	var platformConfiguration platformconfig.Configuration

	platformConfigurationReader, err := platformconfig.NewReader()
	if err != nil {
		return nil, false, errors.Wrap(err, "Failed to create platform configuration reader")
	}

	// if there's no configuration file, return a default configuration. otherwise try to parse it
	platformConfigurationFile, err := os.Open(configurationPath)
	if err != nil {
		return p.getDefaultPlatformConfiguration(), false, nil
	}

	if err := platformConfigurationReader.Read(platformConfigurationFile, "yaml", &platformConfiguration); err != nil {
		return nil, false, errors.Wrap(err, "Failed to open configuration file")
	}

	return &platformConfiguration, true, nil
}

// returns the processor logger and the function logger. For now, they are one of the same
func (p *Processor) createLoggers(platformConfiguration *platformconfig.Configuration) (logger.Logger, logger.Logger, error) {
	newLogger, err := nucliozap.NewNuclioZapCmd("processor", nucliozap.DebugLevel)

	// TODO: create the loggers from configuration
	return newLogger, newLogger, err
}

func (p *Processor) createTriggers(processorConfiguration *processor.Configuration) ([]trigger.Trigger, error) {
	var triggers []trigger.Trigger

	for triggerName, triggerConfiguration := range processorConfiguration.Spec.Triggers {

		// create an event source based on event source configuration and runtime configuration
		triggerInstance, err := trigger.RegistrySingleton.NewTrigger(p.logger,
			triggerConfiguration.Kind,
			triggerName,
			&triggerConfiguration,
			&runtime.Configuration{
				Configuration:  processorConfiguration,
				FunctionLogger: p.functionLogger,
			})

		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create triggers")
		}

		// append to triggers (can be nil - ignore unknown triggers)
		if triggerInstance != nil {
			triggers = append(triggers, triggerInstance)
		}
	}

	// create default event source, given the triggers already created by configuration
	defaultTriggers, err := p.createDefaultTriggers(processorConfiguration, triggers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default triggers")
	}

	// augment with default triggers, if any were created
	triggers = append(triggers, defaultTriggers...)

	return triggers, nil
}

func (p *Processor) createDefaultTriggers(processorConfiguration *processor.Configuration,
	existingTriggers []trigger.Trigger) ([]trigger.Trigger, error) {
	createdTriggers := []trigger.Trigger{}

	// if there's already an http event source in the list of existing, do nothing
	if p.hasHTTPTrigger(existingTriggers) {
		return createdTriggers, nil
	}

	httpTrigger, err := p.createDefaultHTTPTrigger(processorConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default HTTP event source")
	}

	return append(createdTriggers, httpTrigger), nil
}

func (p *Processor) hasHTTPTrigger(triggers []trigger.Trigger) bool {
	for _, existingTrigger := range triggers {
		if existingTrigger.GetKind() == "http" {
			return true
		}
	}

	return false
}

func (p *Processor) createDefaultHTTPTrigger(processorConfiguration *processor.Configuration) (trigger.Trigger, error) {
	defaultHTTPTriggerConfiguration := functionconfig.Trigger{
		Class:      "sync",
		Kind:       "http",
		MaxWorkers: 1,
		URL:        ":8080",
	}

	p.logger.DebugWith("Creating default HTTP event source",
		"configuration", &defaultHTTPTriggerConfiguration)

	return trigger.RegistrySingleton.NewTrigger(p.logger,
		"http",
		"http",
		&defaultHTTPTriggerConfiguration,
		&runtime.Configuration{
			Configuration:  processorConfiguration,
			FunctionLogger: p.functionLogger,
		})
}

func (p *Processor) createWebAdminServer(platformConfiguration *platformconfig.Configuration) (*webadmin.Server, error) {

	// create the server
	return webadmin.NewServer(p.logger, p, &platformConfiguration.WebAdmin)
}

func (p *Processor) createAndStartHealthCheckServer(platformConfiguration *platformconfig.Configuration) (*healthcheck.Server, error) {

	// create the server
	server, err := healthcheck.NewServer(p.logger, p, &platformConfiguration.HealthCheck)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create health check server")
	}

	// start it
	if err = server.Start(); err != nil {
		return nil, errors.Wrap(err, "Failed to start health check server")
	}

	return server, nil
}

func (p *Processor) createMetricPushers(platformConfiguration *platformconfig.Configuration) ([]*statistics.MetricPusher, error) {
	metricSinks, err := platformConfiguration.GetFunctionMetricSinks()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function metric sinks")
	}

	var metricPushers []*statistics.MetricPusher

	for _, metricSink := range metricSinks {
		metricPusher, err := statistics.NewMetricPusher(p.logger, p, &metricSink)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create metric pusher")
		}

		metricPushers = append(metricPushers, metricPusher)
	}

	if len(metricPushers) == 0 {
		p.logger.Warn("No metric sinks configured, metrics will not be published")
	}

	return metricPushers, nil
}

func (p *Processor) getDefaultPlatformConfiguration() *platformconfig.Configuration {
	return &platformconfig.Configuration{
		WebAdmin: platformconfig.WebServer{
			Enabled:       true,
			ListenAddress: ":8081",
		},
		HealthCheck: platformconfig.WebServer{
			Enabled:       true,
			ListenAddress: ":8082",
		},
		Logger: platformconfig.Logger{

			// create an stdout sink and bind everything to it @ debug level
			Sinks: map[string]platformconfig.LoggerSink{
				"stdout": {Driver: "stdout"},
			},

			System: []platformconfig.LoggerSinkBinding{
				{Level: "debug", Sink: "stdout"},
			},

			Functions: []platformconfig.LoggerSinkBinding{
				{Level: "debug", Sink: "stdout"},
			},
		},
	}
}
