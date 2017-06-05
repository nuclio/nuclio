package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/generator"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/http"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/rabbit_mq"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime/golang"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime/shell"
	"github.com/nuclio/nuclio/cmd/processor/app/web_interface/rest"
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

	// first, create a proper runtime for this configuration
	runtimeConfiguration, err := newProcessor.createRuntimeConfiguration(newProcessor.configuration["function"])
	if err != nil {
		return nil, newProcessor.logger.Report(err, "Failed to create worker configuration")
	}

	// create event sources
	newProcessor.eventSources, err = newProcessor.createEventSources(runtimeConfiguration)
	if err != nil {
		return nil, newProcessor.logger.Report(err, "Failed to create event sources")
	}

	return &newProcessor, nil
}

func (p *Processor) Start() error {

	// TODO: Read port from configuration
	rest.StartHTTPD(":8080", p.eventSources)

	// iterate over all event sources and start them
	for _, eventSource := range p.eventSources {
		eventSource.Start(nil)
	}

	// TODO: shutdown
	select {}

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

func (p *Processor) createRuntimeConfiguration(configuration *viper.Viper) (interface{}, error) {
	var runtimeConfiguration interface{}
	var baseConfiguration *runtime.Configuration

	// create based on kind
	if configuration.GetString("kind") == "shell" {
		shellConfiguration := shell_runtime.Configuration{
			ScriptPath: configuration.GetString("path"),
			ScriptArgs: configuration.GetStringSlice("args"),
		}

		// to be populated in a bit
		baseConfiguration = &shellConfiguration.Configuration

		// set worker configuration
		runtimeConfiguration = &shellConfiguration
	} else if configuration.GetString("kind") == "golang" {
		golangConfiguration := golang_runtime.Configuration{
			EventHandlerName: configuration.GetString("name"),
		}

		// to be populated in a bit
		baseConfiguration = &golangConfiguration.Configuration

		// set worker configuration
		runtimeConfiguration = &golangConfiguration
	}

	// populate common configuration
	baseConfiguration.Name = configuration.GetString("name")
	baseConfiguration.Description = configuration.GetString("description")
	baseConfiguration.Version = configuration.GetString("version")

	// return the worker configuration
	return runtimeConfiguration, nil
}

func (p *Processor) createEventSources(runtimeConfiguration interface{}) ([]event_source.EventSource, error) {
	eventSources := []event_source.EventSource{}

	// get configuration (root of event sources)
	eventSourceConfigurations := p.configuration["event_sources"].GetStringMap("")

	for eventSourceID := range eventSourceConfigurations {
		var err error
		var eventSource event_source.EventSource

		// get a sub parser so we don't have to format stuff
		eventSourceConfiguration := p.configuration["event_sources"].Sub(eventSourceID)

		// get the kind of the event source
		eventSourceKind := eventSourceConfiguration.GetString("kind")

		// check which kind
		switch eventSourceKind {
		case "http":
			eventSource, err = p.createHttpEventSource(eventSourceConfiguration, runtimeConfiguration)
		case "generator":
			eventSource, err = p.createGeneratorEventSource(eventSourceConfiguration, runtimeConfiguration)
		case "rabbit-mq":
			eventSource, err = p.createRabbitMqEventSource(eventSourceConfiguration, runtimeConfiguration)
		default:
			p.logger.With(logger.Fields{
				"ID":   eventSourceID,
				"sync": eventSourceKind,
			}).Warn("Ignored event source. Unsupported kind")
		}

		if err != nil {
			return nil, p.logger.Report(err, "Failed to create event sources")
		}

		// append to event sources (can be nil - ignore unknown event sources)
		if eventSource != nil {
			eventSource.SetConfig(eventSourceConfiguration.AllSettings())
			eventSources = append(eventSources, eventSource)
		}
	}

	return eventSources, nil
}

func (p *Processor) createHttpEventSource(eventSourceConfiguration *viper.Viper,
	runtimeConfiguration interface{}) (event_source.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", "1")
	eventSourceConfiguration.SetDefault("listen_address", ":1967")

	// create logger parent
	logger := p.logger.GetChild("http")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := p.createFixedPoolWorkerAllocator(logger, numWorkers, runtimeConfiguration)
	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	httpEventSource, err := http.NewEventSource(logger,
		workerAllocator,
		eventSourceConfiguration.GetString("listen_address"))
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create HTTP event source")
	}

	return httpEventSource, nil
}

func (p *Processor) createRabbitMqEventSource(eventSourceConfiguration *viper.Viper,
	runtimeConfiguration interface{}) (event_source.EventSource, error) {

	// create logger parent
	logger := p.logger.GetChild("rabbit_mq")

	// create worker allocator
	workerAllocator, err := p.createSingletonPoolWorkerAllocator(logger, runtimeConfiguration)
	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := rabbit_mq.NewEventSource(p.logger,
		workerAllocator,
		eventSourceConfiguration.GetString("url"),
		eventSourceConfiguration.GetString("exchange"),
	)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create rabbit-mq event source")
	}

	return generatorEventSource, nil
}

func (p *Processor) createGeneratorEventSource(eventSourceConfiguration *viper.Viper,
	runtimeConfiguration interface{}) (event_source.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", "1")
	eventSourceConfiguration.SetDefault("min_delay_ms", "3000")
	eventSourceConfiguration.SetDefault("max_delay_ms", "3000")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create logger parent
	logger := p.logger.GetChild("generator")

	// create worker allocator
	workerAllocator, err := p.createFixedPoolWorkerAllocator(logger, numWorkers, runtimeConfiguration)
	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := generator.NewEventSource(logger,
		workerAllocator,
		numWorkers,
		eventSourceConfiguration.GetInt("min_delay_ms"),
		eventSourceConfiguration.GetInt("max_delay_ms"))
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create generator event source")
	}

	return generatorEventSource, nil
}

func (p *Processor) createRuntime(logger logger.Logger,
	runtimeConfiguration interface{}) (runtime.Runtime, error) {

	// based on configuration
	switch configuration := runtimeConfiguration.(type) {
	case *shell_runtime.Configuration:
		return shell_runtime.NewRuntime(logger, configuration)
	case *golang_runtime.Configuration:
		return golang_runtime.NewRuntime(logger, configuration)
	}

	return nil, errors.New("Unknown runtime configuration type")
}

func (p *Processor) createWorker(logger logger.Logger,
	workerIndex int,
	runtimeConfiguration interface{}) (*worker.Worker, error) {

	// create logger parent
	workerLogger := logger.GetChild(fmt.Sprintf("w%d", workerIndex))

	// create a runtime for the worker
	runtimeInstance, err := p.createRuntime(workerLogger, runtimeConfiguration)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create runtime")
	}

	return worker.NewWorker(workerLogger, workerIndex, runtimeInstance), nil
}

func (p *Processor) createWorkers(logger logger.Logger,
	numWorkers int,
	runtimeConfiguration interface{}) ([]*worker.Worker, error) {
	workers := make([]*worker.Worker, numWorkers)

	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		worker, err := p.createWorker(logger, workerIndex, runtimeConfiguration)
		if err != nil {
			return nil, p.logger.Report(err, "Failed to create worker")
		}

		workers[workerIndex] = worker
	}

	return workers, nil
}

func (p *Processor) createFixedPoolWorkerAllocator(logger logger.Logger,
	numWorkers int,
	runtimeConfiguration interface{}) (worker.WorkerAllocator, error) {

	// create the workers
	workers, err := p.createWorkers(logger, numWorkers, runtimeConfiguration)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create HTTP event source")
	}

	// create an allocator
	workerAllocator, err := worker.NewFixedPoolWorkerAllocator(p.logger, workers)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
}

func (p *Processor) createSingletonPoolWorkerAllocator(logger logger.Logger,
	runtimeConfiguration interface{}) (worker.WorkerAllocator, error) {

	// create the workers
	workerInstance, err := p.createWorker(logger, 0, runtimeConfiguration)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create HTTP event source")
	}

	// create an allocator
	workerAllocator, err := worker.NewSingletonWorkerAllocator(p.logger, workerInstance)
	if err != nil {
		return nil, p.logger.Report(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
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
