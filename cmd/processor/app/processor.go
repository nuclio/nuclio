package app

import (
	"fmt"
	"path/filepath"

	"github.com/nuclio/nuclio-zap"
	"github.com/nuclio/nuclio-logger/logger"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/generator"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/http"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/poller/v3ioitempoller"
	_ "github.com/nuclio/nuclio/pkg/processor/eventsource/rabbitmq"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Processor struct {
	logger        logger.Logger
	configuration map[string]*viper.Viper
	workers       []worker.Worker
	eventSources  []eventsource.EventSource
}

func NewProcessor(configurationPath string) (*Processor, error) {
	var err error

	newProcessor := Processor{
		configuration: map[string]*viper.Viper{},
	}

	// try to read configuration
	if err := newProcessor.readConfiguration(configurationPath); err != nil {
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

func (p *Processor) Start() error {

	// iterate over all event sources and start them
	for _, eventSource := range p.eventSources {
		eventSource.Start(nil)
	}

	// TODO: shutdown
	select {}

	return nil
}

func (p *Processor) readConfiguration(configurationPath string) error {

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
	for _, sectionName := range []string{"event_sources", "function", "web_admin", "logger"} {

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

func (p *Processor) createLogger(configuration *viper.Viper) (logger.Logger, error) {

	// TODO: configuration stuff
	return nucliozap.NewNuclioZap("processor")
}

func (p *Processor) createEventSources() ([]eventsource.EventSource, error) {
	eventSources := []eventsource.EventSource{}
	eventSourceConfigurations := make(map[string]interface{})
	runtimeConfiguration := p.configuration["function"]

	if runtimeConfiguration == nil {
		return nil, errors.New(`Configuration file must contain a "function" section`)
	}

	// set some defaults for function (runtime) configuration
	runtimeConfiguration.SetDefault("kind", "golang")

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

	httpEventSource, err := p.createDefaultHttpEventSource(runtimeConfiguration)
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

func (p *Processor) createDefaultHttpEventSource(runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {
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
