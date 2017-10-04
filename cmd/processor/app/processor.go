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
	"fmt"
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/statistics"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/zap"

	// Load all sources and runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/generator"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/http"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/kafka"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/kinesis"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/nats"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/rabbitmq"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

// Processor is responsible to process events
type Processor struct {
	logger         nuclio.Logger
	functionLogger nuclio.Logger
	configuration  map[string]*viper.Viper
	workers        []worker.Worker
	eventSources   []eventsource.EventSource
	webAdminServer *webadmin.Server
	metricsPusher  *statistics.MetricPusher
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{}
	newProcessor.configuration, err = config.ReadProcessorConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// create loggers for both the processor and the function invoked by the processor - they may
	// be headed to two different places
	newProcessor.logger,
		newProcessor.functionLogger,
		err = newProcessor.createLoggers(newProcessor.configuration["logger"])

	if err != nil {
		return nil, errors.New("Failed to create logger")
	}

	// create event sources
	newProcessor.eventSources, err = newProcessor.createEventSources()
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
func (p *Processor) GetEventSources() []eventsource.EventSource {
	return p.eventSources
}

func (p *Processor) readConfiguration(configurationPath string) error {

	// if no configuration file passed use defaults all around
	if configurationPath == "" {
		return nil
	}

	// read root configuration
	p.configuration["root"] = viper.New()
	p.configuration["root"].SetConfigFile(configurationPath)

	// read the root configuration file
	if err := p.configuration["root"].ReadInConfig(); err != nil {
		return err
	}

	// get the directory of the root configuration file, we'll need it since all section
	// configuration files are relative to that
	rootConfigurationDir := filepath.Dir(configurationPath)

	// read the configuration file sections, which may be in separate configuration files or inline
	for _, sectionName := range config.Sections {

		// try to get <section name>.config_path (e.g. function.config_path)
		sectionConfigPath := p.configuration["root"].GetString(fmt.Sprintf("%s.config_path", sectionName))

		// if it exists, create a viper and read it
		if sectionConfigPath != "" {
			p.configuration[sectionName] = viper.New()
			p.configuration[sectionName].SetConfigFile(filepath.Join(rootConfigurationDir, sectionConfigPath))

			// do the read
			if err := p.configuration[sectionName].ReadInConfig(); err != nil {
				return err
			}
		} else {

			// the section is a sub of the root
			p.configuration[sectionName] = p.configuration["root"].Sub(sectionName)
		}
	}

	return nil
}

// returns the processor logger and the function logger. For now, they are one of the same
func (p *Processor) createLoggers(configuration *viper.Viper) (nuclio.Logger, nuclio.Logger, error) {
	newLogger, err := nucliozap.NewNuclioZapCmd("processor", nucliozap.DebugLevel)

	// TODO: create the loggers from configuration
	return newLogger, newLogger, err
}

func (p *Processor) createEventSources() ([]eventsource.EventSource, error) {
	eventSources := []eventsource.EventSource{}
	eventSourceConfigurations := make(map[string]interface{})

	// get the runtime configuration
	runtimeConfiguration, err := p.getRuntimeConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get runtime configuration")
	}

	// get configuration (root of event sources) if event sources exists in configuration. if it doesn't
	// just skip and default event sources will be created
	eventSourceConfigurationsViper := p.configuration["event_sources"]
	if eventSourceConfigurationsViper != nil {
		eventSourceConfigurations = eventSourceConfigurationsViper.GetStringMap("")
	}

	for eventSourceID := range eventSourceConfigurations {
		var eventSource eventsource.EventSource
		eventSourceConfiguration := p.configuration["event_sources"].Sub(eventSourceID)

		// set the ID of the event source
		eventSourceConfiguration.Set("id", eventSourceID)

		// create an event source based on event source configuration and runtime configuration
		eventSource, err = eventsource.RegistrySingleton.NewEventSource(p.logger,
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
	defaultEventSources, err := p.createDefaultEventSources(eventSources, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default event sources")
	}

	// augment with default event sources, if any were created
	eventSources = append(eventSources, defaultEventSources...)

	return eventSources, nil
}

func (p *Processor) createDefaultEventSources(existingEventSources []eventsource.EventSource,
	runtimeConfiguration *viper.Viper) ([]eventsource.EventSource, error) {
	createdEventSources := []eventsource.EventSource{}

	// if there's already an http event source in the list of existing, do nothing
	if p.hasHTTPEventSource(existingEventSources) {
		return createdEventSources, nil
	}

	httpEventSource, err := p.createDefaultHTTPEventSource(runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default HTTP event source")
	}

	return append(createdEventSources, httpEventSource), nil
}

func (p *Processor) hasHTTPEventSource(eventSources []eventsource.EventSource) bool {
	for _, existingEventSource := range eventSources {
		if existingEventSource.GetKind() == "http" {
			return true
		}
	}

	return false
}

func (p *Processor) createDefaultHTTPEventSource(runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {
	listenAddress := ":8080"

	p.logger.DebugWith("Creating default HTTP event source",
		"num_workers", 1,
		"listen_address", listenAddress)

	// populate default HTTP configuration
	httpConfiguration := viper.New()
	httpConfiguration.Set("ID", "default_http")
	httpConfiguration.Set("num_workers", 1)
	httpConfiguration.Set("listen_address", listenAddress)

	return eventsource.RegistrySingleton.NewEventSource(p.logger,
		"http",
		httpConfiguration,
		runtimeConfiguration)
}

func (p *Processor) getRuntimeConfiguration() (*viper.Viper, error) {
	runtimeConfiguration := p.configuration["function"]

	if runtimeConfiguration == nil {
		p.logger.Debug("No runtime configuration, using default")

		// initialize with a new viper
		runtimeConfiguration = viper.New()

		// try to read env var. if env doesn't exist, the function selection logic will
		// just choose the first registered function
		runtimeConfiguration.SetDefault("name", os.Getenv("NUCLIO_FUNCTION_NAME"))
	}

	// by default use golang
	runtimeConfiguration.SetDefault("kind", "golang")

	// set the function logger as a configuration, to be read by the runtimes
	runtimeConfiguration.Set("function_logger", p.functionLogger)

	return runtimeConfiguration, nil
}

func (p *Processor) createWebAdminServer() (*webadmin.Server, error) {

	// create the server
	return webadmin.NewServer(p.logger, p, p.configuration["web_admin"])
}

func (p *Processor) createMetricPusher() (*statistics.MetricPusher, error) {

	// create the pusher
	return statistics.NewMetricPusher(p.logger, p, p.configuration["metrics"])
}
