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
	"encoding/json"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	commonhealthcheck "github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/healthcheck"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	// load all runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/dotnetcore"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/java"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/nodejs"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/ruby"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/timeout"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	// load all triggers
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/http"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kafka"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kickstart"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kinesis"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/mqtt/basic"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/mqtt/iotcore"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/nats"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/partitioned/eventhub"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/pubsub"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/rabbitmq"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/v3iostream"
	"github.com/nuclio/nuclio/pkg/processor/util/clock"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/version-go"
	"golang.org/x/sync/errgroup"
)

// Processor is responsible to process events
type Processor struct {
	logger                logger.Logger
	functionLogger        logger.Logger
	triggers              []trigger.Trigger
	webAdminServer        *webadmin.Server
	healthCheckServer     commonhealthcheck.Server
	metricSinks           []metricsink.MetricSink
	namedWorkerAllocators map[string]worker.Allocator
	eventTimeoutWatcher   *timeout.EventTimeoutWatcher
	startComplete         bool
	stop                  chan bool
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string, platformConfigurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{
		namedWorkerAllocators: map[string]worker.Allocator{},
		stop:                  make(chan bool, 1),
	}

	// get platform configuration
	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get platform configuration")
	}

	processorConfiguration, err := newProcessor.readConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// create the function logger
	newProcessor.logger, err = loggersink.CreateFunctionLogger("processor",
		&processorConfiguration.Config,
		platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	// for now, use the same logger for both the processor and user handler
	newProcessor.functionLogger = newProcessor.logger
	newProcessor.logger.InfoWith("Starting processor", "version", version.Get())

	indentedProcessorConfiguration, _ := json.MarshalIndent(processorConfiguration, "", "    ")
	indentedPlatformConfiguration, _ := json.MarshalIndent(platformConfiguration, "", "    ")

	newProcessor.logger.DebugWith("Read configuration",
		"config", string(indentedProcessorConfiguration),
		"platformConfig", string(indentedPlatformConfiguration))

	// save platform configuration in process configuration
	processorConfiguration.PlatformConfig = platformConfiguration

	if processorConfiguration.Spec.EventTimeout != "" {
		clock.SetResolution(1 * time.Second)
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

	if len(processorConfiguration.Spec.EventTimeout) > 0 {

		// This is checked by the configuration reader, but just in case
		eventTimeout, timeoutErr := processorConfiguration.Spec.GetEventTimeout()
		if timeoutErr != nil {
			return nil, errors.Wrap(timeoutErr, "Bad EventTimeout")
		}

		if startErr := newProcessor.startTimeoutWatcher(eventTimeout); startErr != nil {
			return nil, errors.Wrap(startErr, "Can't start timeout watcher")
		}
	}

	// create the web interface
	newProcessor.webAdminServer, err = newProcessor.createWebAdminServer(platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create web interface server")
	}

	// create metric pusher
	newProcessor.metricSinks, err = newProcessor.createMetricSinks(processorConfiguration, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create metric sinks")
	}

	return newProcessor, nil
}

// Start starts the processor
func (p *Processor) Start() error {
	p.logger.DebugWith("Starting triggers", "triggers", p.triggers)

	// iterate over all triggers and start them
	for _, triggerInstance := range p.triggers {
		if err := triggerInstance.Start(nil); err != nil {
			p.logger.ErrorWith("Failed to start trigger",
				"kind", triggerInstance.GetKind(),
				"err", err.Error())
			return errors.Wrap(err, "Failed to start trigger")
		}
	}

	// start the web interface
	if err := p.webAdminServer.Start(); err != nil {
		return errors.Wrap(err, "Failed to start web interface")
	}

	// start pushing metrics
	for _, metricPusher := range p.metricSinks {
		if err := metricPusher.Start(); err != nil {
			return errors.Wrap(err, "Failed to start metric pushing")
		}
	}

	// indicate that we're done starting
	p.startComplete = true

	p.logger.Debug("Processor started")

	<-p.stop // Wait for stop
	p.logger.Info("Processor quitting")

	time.Sleep(5 * time.Second) // Give triggers etc time to finish

	return nil
}

// GetTriggers returns triggers
func (p *Processor) GetTriggers() []trigger.Trigger {
	return p.triggers
}

// GetWorkers returns workers
func (p *Processor) GetWorkers() []*worker.Worker {
	var workers []*worker.Worker

	// iterate over the processor's triggers
	for _, triggerInstance := range p.triggers {
		workers = append(workers, triggerInstance.GetWorkers()...)
	}

	return workers
}

// GetStatus returns the processor's status based on its workers' readiness
func (p *Processor) GetStatus() status.Status {
	workers := p.GetWorkers()

	// if no workers exist yet, return initializing
	if !p.startComplete {
		return status.Initializing
	}

	// if any worker isn't ready yet, return initializing
	for _, workerInstance := range workers {
		if workerInstance.GetStatus() != status.Ready {
			return status.Initializing
		}
	}

	// otherwise we're ready
	return status.Ready
}

// Stop stops the processor
func (p *Processor) Stop() {
	p.stop <- true
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

func (p *Processor) createTriggers(processorConfiguration *processor.Configuration) ([]trigger.Trigger, error) {
	var triggers []trigger.Trigger

	// create error group
	errGroup := errgroup.Group{}
	platformKind := processorConfiguration.PlatformConfig.Kind

	for triggerName, triggerConfiguration := range processorConfiguration.Spec.Triggers {
		triggerName, triggerConfiguration := triggerName, triggerConfiguration

		// skipping cron triggers when platform kind is "kube" and k8s cron jobs are enabled- k8s cron jobs will be created instead
		if triggerConfiguration.Kind == "cron" &&
			platformKind == "kube" &&
			processorConfiguration.PlatformConfig.CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {

			p.logger.DebugWith("Skipping cron trigger creation inside the processor",
				"triggerName", triggerName,
				"platformKind", platformKind)

			continue
		}

		errGroup.Go(func() error {

			// create an event source based on event source configuration and runtime configuration
			triggerInstance, err := trigger.RegistrySingleton.NewTrigger(p.logger,
				triggerConfiguration.Kind,
				triggerName,
				&triggerConfiguration,
				&runtime.Configuration{
					Configuration:  processorConfiguration,
					FunctionLogger: p.functionLogger,
				},
				p.namedWorkerAllocators)

			if err != nil {
				return errors.Wrapf(err, "Failed to create triggers")
			}

			// append to triggers (can be nil - ignore unknown triggers)
			if triggerInstance != nil {
				triggers = append(triggers, triggerInstance)
			}

			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, errors.Wrap(err, "Failed to create triggers")
	}

	return triggers, nil
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
		URL:        common.GetEnvOrDefaultString("NUCLIO_DEFAULT_HTTP_TRIGGER_URL", ":8080"),
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
		},
		p.namedWorkerAllocators)
}

func (p *Processor) createWebAdminServer(platformConfiguration *platformconfig.Config) (*webadmin.Server, error) {

	// if enabled not passed, default to true
	if platformConfiguration.WebAdmin.Enabled == nil {
		trueValue := true
		platformConfiguration.WebAdmin.Enabled = &trueValue
	}

	if platformConfiguration.WebAdmin.ListenAddress == "" {
		platformConfiguration.WebAdmin.ListenAddress = ":8081"
	}

	// create the server
	return webadmin.NewServer(p.logger, p, &platformConfiguration.WebAdmin)
}

func (p *Processor) createAndStartHealthCheckServer(platformConfiguration *platformconfig.Config) (
	commonhealthcheck.Server, error) {

	// if enabled not passed, default to true
	if platformConfiguration.HealthCheck.Enabled == nil {
		trueValue := true
		platformConfiguration.HealthCheck.Enabled = &trueValue
	}

	if platformConfiguration.HealthCheck.ListenAddress == "" {
		platformConfiguration.HealthCheck.ListenAddress = ":8082"
	}

	// create the server
	server, err := healthcheck.NewProcessorServer(p.logger, p, &platformConfiguration.HealthCheck)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create health check server")
	}

	// start the web interface
	if err := server.Start(); err != nil {
		return nil, errors.Wrap(err, "Failed to start health check server")
	}

	return server, nil
}

func (p *Processor) createMetricSinks(processorConfiguration *processor.Configuration,
	platformConfiguration *platformconfig.Config) ([]metricsink.MetricSink, error) {
	metricSinksConfiguration, err := platformConfiguration.GetFunctionMetricSinks()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function metric sinks configuration")
	}

	var metricSinks []metricsink.MetricSink

	for metricSinkName, metricSinkConfiguration := range metricSinksConfiguration {
		newMetricSinkInstance, err := metricsink.RegistrySingleton.NewMetricSink(p.logger,
			processorConfiguration,
			metricSinkConfiguration.Kind,
			metricSinkName,
			&metricSinkConfiguration,
			p)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create metric sink")
		}

		metricSinks = append(metricSinks, newMetricSinkInstance)
	}

	if len(metricSinks) == 0 {
		p.logger.Warn("No metric sinks configured, metrics will not be published")
	}

	return metricSinks, nil
}

func (p *Processor) startTimeoutWatcher(eventTimeout time.Duration) error {
	var err error

	if eventTimeout == 0 {
		eventTimeout = clock.DefaultResolution
	}

	p.logger.InfoWith("Starting event timeout watcher", "timeout", eventTimeout.String())
	p.eventTimeoutWatcher, err = timeout.NewEventTimeoutWatcher(p.logger, eventTimeout, p)

	if err != nil {
		errorMessage := "Can't start event timeout watcher"
		p.logger.ErrorWith(errorMessage, "error", err)
		return errors.Wrap(err, errorMessage)
	}

	return nil
}
