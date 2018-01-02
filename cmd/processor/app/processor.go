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
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	// load all runtimes
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/nodejs"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/statistics"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	// load all triggers
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
)

// Processor is responsible to process events
type Processor struct {
	logger                 nuclio.Logger
	functionLogger         nuclio.Logger
	processorConfiguration *processor.Configuration
	workers                []worker.Worker
	triggers               []trigger.Trigger
	webAdminServer         *webadmin.Server
	metricsPusher          *statistics.MetricPusher
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{}

	// create loggers for both the processor and the function invoked by the processor - they may
	// be headed to two different places
	newProcessor.logger,
		newProcessor.functionLogger,
		err = newProcessor.createLoggers()

	if err != nil {
		return nil, errors.New("Failed to create logger")
	}

	newProcessor.processorConfiguration, err = newProcessor.readConfiguration(configurationPath)
	if err != nil {
		return nil, err
	}

	// create triggers
	newProcessor.triggers, err = newProcessor.createTriggers()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create triggers")
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

	// iterate over all triggers and start them
	for _, trigger := range p.triggers {
		trigger.Start(nil)
	}

	// start the web interface
	//err := p.webAdminServer.Start()
	//if err != nil {
	//	return errors.Wrap(err, "Failed to start web interface")
	//}

	// start pushing metrics
	//err = p.metricsPusher.Start()
	//if err != nil {
	//	return errors.Wrap(err, "Failed to start metric pushing")
	//}

	// TODO: shutdown
	select {}
}

// get triggers
func (p *Processor) GetTriggers() []trigger.Trigger {
	return p.triggers
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

// returns the processor logger and the function logger. For now, they are one of the same
func (p *Processor) createLoggers() (nuclio.Logger, nuclio.Logger, error) {
	newLogger, err := nucliozap.NewNuclioZapCmd("processor", nucliozap.DebugLevel)

	// TODO: create the loggers from configuration
	return newLogger, newLogger, err
}

func (p *Processor) createTriggers() ([]trigger.Trigger, error) {
	var triggers []trigger.Trigger

	for triggerName, triggerConfiguration := range p.processorConfiguration.Spec.Triggers {

		// create an event source based on event source configuration and runtime configuration
		triggerInstance, err := trigger.RegistrySingleton.NewTrigger(p.logger,
			triggerConfiguration.Kind,
			triggerName,
			&triggerConfiguration,
			&runtime.Configuration{
				Configuration:  p.processorConfiguration,
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
	defaultTriggers, err := p.createDefaultTriggers(triggers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create default triggers")
	}

	// augment with default triggers, if any were created
	triggers = append(triggers, defaultTriggers...)

	return triggers, nil
}

func (p *Processor) createDefaultTriggers(existingTriggers []trigger.Trigger) ([]trigger.Trigger, error) {
	createdTriggers := []trigger.Trigger{}

	// if there's already an http event source in the list of existing, do nothing
	if p.hasHTTPTrigger(existingTriggers) {
		return createdTriggers, nil
	}

	httpTrigger, err := p.createDefaultHTTPTrigger()
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

func (p *Processor) createDefaultHTTPTrigger() (trigger.Trigger, error) {
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
			Configuration:  p.processorConfiguration,
			FunctionLogger: p.functionLogger,
		})
}

func (p *Processor) createWebAdminServer() (*webadmin.Server, error) {

	// create the server (TODO: once platform configuration is introduced)
	return nil, nil
}

func (p *Processor) createMetricPusher() (*statistics.MetricPusher, error) {

	// create the server (TODO: once platform configuration is introduced)
	return nil, nil
}
