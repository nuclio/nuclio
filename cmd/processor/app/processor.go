package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	_ "github.com/nuclio/nuclio/cmd/processor/app/event_source/generator"
	_ "github.com/nuclio/nuclio/cmd/processor/app/event_source/http"
	_ "github.com/nuclio/nuclio/cmd/processor/app/event_source/rabbit_mq"
	_ "github.com/nuclio/nuclio/cmd/processor/app/runtime/golang"
	_ "github.com/nuclio/nuclio/cmd/processor/app/runtime/shell"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/logger/formatted"
)

type Processor struct {
	logger        logger.Logger
	configuration map[string]*viper.Viper
	workers       []worker.Worker
	eventSources  []event_source.EventSource
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
		return nil, newProcessor.logger.Report(err, "Failed to create event sources")
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

	// support only formatted for now
	if configuration.GetString("kind") != "formatted" {
		return nil, errors.New("Unsupported logger kind")
	}

	// list of output configurations, to be populated from config
	outputs := []interface{}{}

	// create a list of objects (string/interface) from the outputs
	for _, outputConfiguration := range p.getObjectSlice(configuration, "outputs") {

		// for each output configuration, create an output config
		switch outputConfiguration["kind"].(string) {
		case "stdout":
			outputs = append(outputs, &formatted.StdoutOutputConfig{
				formatted.OutputConfig{outputConfiguration["level"].(string)},
				outputConfiguration["colors"].(string),
			})
		}
	}

	// create an output logger
	formattedLogger := formatted.NewLogger("nuclio", outputs)

	// TODO: from configuration
	formattedLogger.SetLevel(formatted.Debug)

	// return as logger
	return formattedLogger, nil
}

func (p *Processor) createEventSources() ([]event_source.EventSource, error) {
	eventSources := []event_source.EventSource{}

	// get configuration (root of event sources)
	eventSourceConfigurations := p.configuration["event_sources"].GetStringMap("")

	for eventSourceID := range eventSourceConfigurations {

		// create an event source based on event source configuration and runtime configuration
		eventSource, err := event_source.RegistrySingleton.NewEventSource(p.logger,
			p.configuration["event_sources"].Sub(eventSourceID),
			p.configuration["function"])

		if err != nil {
			return nil, p.logger.Report(err, "Failed to create event sources")
		}

		// append to event sources (can be nil - ignore unknown event sources)
		if eventSource != nil {
			eventSources = append(eventSources, eventSource)
		}
	}

	return eventSources, nil
}

// this function extracts a list of objects from a viper instance. there may be a better way to do this with viper
// but i've yet to find it (TODO: post issue?)
func (p *Processor) getObjectSlice(configuration *viper.Viper, key string) []map[string]interface{} {
	objectsAsMapStringInterface := []map[string]interface{}{}

	// get as slice of interfaces
	objectsAsInterfaces := configuration.Get(key).([]interface{})

	// iterate over objects as interfaces
	for _, objectAsInterface := range objectsAsInterfaces {
		objectAsMapStringInterface := map[string]interface{}{}

		// convert each object to a map of its fields (interface/interface)
		objectFieldsAsMapInterfaceInterface := objectAsInterface.(map[interface{}]interface{})

		// iterate over fields, convert key to string and keep value as interface, shove to
		// objectAsMapStringInterface
		for objectFieldKey, objectFieldValue := range objectFieldsAsMapInterfaceInterface {
			objectAsMapStringInterface[objectFieldKey.(string)] = objectFieldValue
		}

		// add object to map
		objectsAsMapStringInterface = append(objectsAsMapStringInterface, objectAsMapStringInterface)
	}

	return objectsAsMapStringInterface
}
