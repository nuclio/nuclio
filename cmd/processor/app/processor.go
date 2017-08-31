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

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/zap"

	// Load all sources and runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/generator"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/http"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/rabbitmq"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Processor is responsible to process events
type Processor struct {
	logger        nuclio.Logger
	configuration map[string]*viper.Viper
	workers       []worker.Worker
	eventSources  []eventsource.EventSource
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string) (*Processor, error) {
	var err error

	newProcessor := Processor{}
	newProcessor.configuration, err = config.ReadProcessorConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// initialize a logger
	newProcessor.logger, err = newProcessor.createLogger(newProcessor.configuration["logger"])
	if err != nil {
		return nil, errors.New("Failed to create logger")
	}

	// create event sources
	newProcessor.eventSources, err = newProcessor.createEventSources()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create event sources")
	}

	return &newProcessor, nil
}

// Start starts the processor
func (p *Processor) Start() error {

	// iterate over all event sources and start them
	for _, eventSource := range p.eventSources {
		eventSource.Start(nil)
	}

	// TODO: shutdown
	select {}
}

func (p *Processor) createLogger(configuration *viper.Viper) (nuclio.Logger, error) {

	// TODO: configuration stuff
	return nucliozap.NewNuclioZap("processor", nucliozap.DebugLevel)
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
		eventSourceConfiguration := p.configuration["event_sources"].Sub(eventSourceID)

		// set the ID of the event source
		eventSourceConfiguration.Set("id", eventSourceID)

		// create an event source based on event source configuration and runtime configuration
		eventSource, err := eventsource.RegistrySingleton.NewEventSource(p.logger,
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
	httpConfiguration.Set("num_workers", 1)
	httpConfiguration.Set("listen_address", listenAddress)

	return eventsource.RegistrySingleton.NewEventSource(p.logger,
		"http",
		httpConfiguration,
		runtimeConfiguration)
}

func (p *Processor) getRuntimeConfiguration() (*viper.Viper, error) {
	runtimeConfiguration := p.configuration["function"]

	// get function name
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

	return runtimeConfiguration, nil
}
