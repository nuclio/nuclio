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
	"github.com/nuclio/nuclio/pkg/errors"
	// load all runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/statistics"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	// load all triggers
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/generator"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/http"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kafka"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/kinesis"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/nats"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/rabbitmq"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

// Processor is responsible to process events
type Processor struct {
	logger         nuclio.Logger
	functionLogger nuclio.Logger
	configuration  *viper.Viper
	workers        []worker.Worker
	eventSources   []trigger.Trigger
	webAdminServer *webadmin.Server
	metricsPusher  *statistics.MetricPusher
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{}
	newProcessor.configuration, err = newProcessor.readConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// create loggers for both the processor and the function invoked by the processor - they may
	// be headed to two different places
	newProcessor.logger,
		newProcessor.functionLogger,
		err = newProcessor.createLoggers(newProcessor.getSubConfiguration("logger"))

	if err != nil {
		return nil, errors.New("Failed to create logger")
	}

	// create event sources
	newProcessor.eventSources, err = newProcessor.createTriggers()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create event sources")
	}

	// create the web interface
	newProcessor.webAdminServer, err = newProcessor.createWebAdminServer()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create web interface server")
	}

	// create metric pusher
	newProcessor.metricsPusher, err = newProcessor.createMetricPusher()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create metric pusher")
	}

	return newProcessor, nil
}

// Start starts the processor
func (p *Processor) Start() error {

	// iterate over all event sources and start them
	for _, eventSource := range p.eventSources {
		eventSource.Start(nil)
	}

	// start the web interface
	err := p.webAdminServer.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start web interface")
	}

	// start pushing metrics
	err = p.metricsPusher.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start metric pushing")
	}

	// TODO: shutdown
	select {}
}

// get event sources
func (p *Processor) GetTriggers() []trigger.Trigger {
	return p.eventSources
}

func (p *Processor) readConfiguration(configurationPath string) (*viper.Viper, error) {

	// if no configuration file passed use defaults all around
	if configurationPath == "" {
		return nil, nil
	}

	// read root configuration
	configuration := viper.New()
	configuration.SetConfigFile(configurationPath)

	// read the root configuration file
	if err := configuration.ReadInConfig(); err != nil {
		return nil, err
	}

	return configuration, nil
}

// returns the processor logger and the function logger. For now, they are one of the same
func (p *Processor) createLoggers(configuration *viper.Viper) (nuclio.Logger, nuclio.Logger, error) {
	newLogger, err := nucliozap.NewNuclioZapCmd("processor", nucliozap.DebugLevel)

	// TODO: create the loggers from configuration
	return newLogger, newLogger, err
}

func (p *Processor) createTriggers() ([]trigger.Trigger, error) {
	eventSources := []trigger.Trigger{}
	eventSourceConfigurations := make(map[string]interface{})

	// get the runtime configuration
	runtimeConfiguration, err := p.getRuntimeConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get runtime configuration")
	}

	// get configuration (root of event sources) if event sources exists in configuration. if it doesn't
	// just skip and default event sources will be created
	eventSourceConfigurationsViper := p.getSubConfiguration("triggers")
	if eventSourceConfigurationsViper != nil {
		eventSourceConfigurations = eventSourceConfigurationsViper.GetStringMap("")
	}

	for eventSourceID := range eventSourceConfigurations {
		var eventSource trigger.Trigger
		eventSourceConfiguration := p.getSubConfiguration("triggers").Sub(eventSourceID)

		// set the ID of the event source
		eventSourceConfiguration.Set("id", eventSourceID)

		// create an event source based on event source configuration and runtime configuration
		eventSource, err = trigger.RegistrySingleton.NewTrigger(p.logger,
			eventSourceConfiguration.GetString("kind"),
			eventSourceConfiguration,
			runtimeConfiguration)

		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create event sources")
		}

		// append to event sources (can be nil - ignore unknown event sources)
		if eventSource != nil {
			eventSources = append(eventSources, eventSource)
		}
	}

	// create default event source, given the event sources already created by configuration
	defaultTriggers, err := p.createDefaultTriggers(eventSources, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default event sources")
	}

	// augment with default event sources, if any were created
	eventSources = append(eventSources, defaultTriggers...)

	return eventSources, nil
}

func (p *Processor) createDefaultTriggers(existingTriggers []trigger.Trigger,
	runtimeConfiguration *viper.Viper) ([]trigger.Trigger, error) {
	createdTriggers := []trigger.Trigger{}

	// if there's already an http event source in the list of existing, do nothing
	if p.hasHTTPTrigger(existingTriggers) {
		return createdTriggers, nil
	}

	httpTrigger, err := p.createDefaultHTTPTrigger(runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default HTTP event source")
	}

	return append(createdTriggers, httpTrigger), nil
}

func (p *Processor) hasHTTPTrigger(eventSources []trigger.Trigger) bool {
	for _, existingTrigger := range eventSources {
		if existingTrigger.GetKind() == "http" {
			return true
		}
	}

	return false
}

func (p *Processor) createDefaultHTTPTrigger(runtimeConfiguration *viper.Viper) (trigger.Trigger, error) {
	listenAddress := ":8080"

	p.logger.DebugWith("Creating default HTTP event source",
		"num_workers", 1,
		"listen_address", listenAddress)

	// populate default HTTP configuration
	httpConfiguration := viper.New()
	httpConfiguration.Set("ID", "default_http")
	httpConfiguration.Set("num_workers", 1)
	httpConfiguration.Set("listen_address", listenAddress)

	return trigger.RegistrySingleton.NewTrigger(p.logger,
		"http",
		httpConfiguration,
		runtimeConfiguration)
}

func (p *Processor) getRuntimeConfiguration() (*viper.Viper, error) {
	runtimeConfiguration := p.getSubConfiguration("function")

	// set the function logger as a configuration, to be read by the runtimes
	runtimeConfiguration.Set("function_logger", p.functionLogger)

	// set the data binding configuration
	runtimeConfiguration.Set("dataBindings", p.getSubConfiguration("dataBindings"))

	return runtimeConfiguration, nil
}

func (p *Processor) createWebAdminServer() (*webadmin.Server, error) {

	// create the server
	return webadmin.NewServer(p.logger, p, p.getSubConfiguration("web_admin"))
}

func (p *Processor) createMetricPusher() (*statistics.MetricPusher, error) {

	// create the pusher
	return statistics.NewMetricPusher(p.logger, p, p.getSubConfiguration("metrics"))
}

func (p *Processor) getSubConfiguration(key string) *viper.Viper {

	if subViper := p.configuration.Sub(key); subViper != nil {
		return subViper
	}

	// return an empty viper
	return viper.New()
}
