/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	commonhealthcheck "github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
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
)

// Processor is responsible to process events
type Processor struct {
	logger                    logger.Logger
	functionLogger            logger.Logger
	triggers                  []trigger.Trigger
	webAdminServer            *webadmin.Server
	healthCheckServer         commonhealthcheck.Server
	metricSinks               []metricsink.MetricSink
	namedWorkerAllocators     *worker.AllocatorSyncMap
	eventTimeoutWatcher       *timeout.EventTimeoutWatcher
	startComplete             bool
	stop                      chan bool
	stopRestartTriggerRoutine chan bool
	restartTriggerChan        chan trigger.Trigger
}

// NewProcessor returns a new Processor
func NewProcessor(configurationPath string, platformConfigurationPath string) (*Processor, error) {
	var err error

	newProcessor := &Processor{
		namedWorkerAllocators:     worker.NewAllocatorSyncMap(),
		stop:                      make(chan bool, 1),
		stopRestartTriggerRoutine: make(chan bool, 1),
		restartTriggerChan:        make(chan trigger.Trigger, 1),
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

	newProcessor.logger.DebugWith("Read configuration",
		"config", string(indentedProcessorConfiguration))

	// restore function configuration from secret if needed
	if !processorConfiguration.Spec.DisableSensitiveFieldsMasking {

		// check if env var to restore is set
		if restoreConfigFromSecret := common.GetEnvOrDefaultBool(common.RestoreConfigFromSecretEnvVar,
			false); restoreConfigFromSecret {
			restoredFunctionConfig, err := newProcessor.restoreFunctionConfig(&processorConfiguration.Config)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to restore function configuration")
			}
			processorConfiguration.Config = *restoredFunctionConfig
		}
	}

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

	// create a goroutine that restarts a trigger if needed
	go p.listenOnRestartTriggerChannel()

	// handles system signals (for now only SIGTERM)
	go p.handleSignals()

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

	// if any worker isn't ready yet, return its status
	for _, workerInstance := range workers {
		if workerStatus := workerInstance.GetStatus(); workerStatus != status.Ready {
			return workerStatus
		}
	}

	// otherwise we're ready
	return status.Ready
}

// Stop stops the processor
func (p *Processor) Stop() {
	p.stopRestartTriggerRoutine <- true
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

// restoreFunctionConfig restores a scrubbed function configuration to the original values from the
// mounted secret, if it exists
func (p *Processor) restoreFunctionConfig(config *functionconfig.Config) (*functionconfig.Config, error) {

	// initialize scrubber, we don't care about sensitive fields and kubeClientSet
	scrubber := functionconfig.NewScrubber(nil, nil)

	secretsMap, err := p.getSecretsMap(scrubber)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get secrets map")
	}

	// if there are no secrets, return
	if len(secretsMap) == 0 {
		p.logger.Debug("Secret is empty, skipping config restoration")
		return config, nil
	}

	restoredFunctionConfig, err := scrubber.Restore(config, secretsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to restore function config")
	}

	return restoredFunctionConfig, nil
}

func (p *Processor) getSecretsMap(scrubber *functionconfig.Scrubber) (map[string]string, error) {

	// the env var is mainly for testing
	filePath := os.Getenv("NUCLIO_FUNCTION_SECRET_VOLUME_PATH")
	if filePath == "" {
		filePath = functionconfig.FunctionSecretMountPath
	}

	contentPath := path.Join(filePath, functionconfig.SecretContentKey)

	// check if a secret is mounted
	if _, err := os.Stat(contentPath); err != nil {
		p.logger.WarnWith("Failed to check if secret file exists",
			"path", contentPath,
			"err", err)
		if os.IsNotExist(err) {
			return nil, errors.New("Secret is not mounted to function pod")
		}
		return nil, errors.Wrap(err, "Failed to check if secret file exists")
	}

	p.logger.Debug("Secret is mounted to function pod, restoring function config")

	// read secret content from file
	encodedSecret, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read function secret")
	}

	if string(encodedSecret) == "" {
		return map[string]string{}, nil
	}

	return scrubber.DecodeSecretsMapContent(string(encodedSecret))
}

func (p *Processor) createTriggers(processorConfiguration *processor.Configuration) ([]trigger.Trigger, error) {
	var triggers []trigger.Trigger
	abstractControlMessageBroker := controlcommunication.NewAbstractControlMessageBroker()

	// create error group
	errGroup, _ := errgroup.WithContext(context.Background(), p.logger)
	lock := sync.Mutex{}

	platformKind := processorConfiguration.PlatformConfig.Kind
	if processorConfiguration.Meta.Labels == nil {

		// backwards compatibility, for function created before labels were introduced
		processorConfiguration.Meta.Labels = map[string]string{
			common.NuclioResourceLabelKeyProjectName: "",
		}
	}

	for triggerName, triggerConfiguration := range processorConfiguration.Spec.Triggers {
		triggerName, triggerConfiguration := triggerName, triggerConfiguration

		// skipping cron triggers when platform kind is "kube" and k8s cron jobs are enabled- k8s cron jobs will be created instead
		if triggerConfiguration.Kind == "cron" &&
			platformKind == common.KubePlatformName &&
			processorConfiguration.PlatformConfig.CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {

			p.logger.DebugWith("Skipping cron trigger creation inside the processor",
				"triggerName", triggerName,
				"platformKind", platformKind)

			continue
		}

		errGroup.Go("Creating trigger", func() error {

			// create an event source based on event source configuration and runtime configuration
			triggerInstance, err := trigger.RegistrySingleton.NewTrigger(p.logger,
				triggerConfiguration.Kind,
				triggerName,
				&triggerConfiguration,
				&runtime.Configuration{
					Configuration:        processorConfiguration,
					FunctionLogger:       p.functionLogger,
					ControlMessageBroker: abstractControlMessageBroker,
				},
				p.namedWorkerAllocators,
				p.restartTriggerChan)

			if err != nil {
				return errors.Wrapf(err, "Failed to create triggers")
			}

			// append to triggers (can be nil - ignore unknown triggers)
			if triggerInstance != nil {
				lock.Lock()
				triggers = append(triggers, triggerInstance)
				lock.Unlock()
			} else {
				p.logger.WarnWith("Skipping unknown trigger",
					"name", triggerName,
					"kind", triggerConfiguration.Kind)
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
		p.namedWorkerAllocators,
		p.restartTriggerChan)
}

func (p *Processor) createWebAdminServer(platformConfiguration *platformconfig.Config) (*webadmin.Server, error) {

	// if enabled not passed, default to true
	if platformConfiguration.WebAdmin.Enabled == nil {
		trueValue := true
		platformConfiguration.WebAdmin.Enabled = &trueValue
	}

	if platformConfiguration.WebAdmin.ListenAddress == "" {
		platformConfiguration.WebAdmin.ListenAddress = fmt.Sprintf(":%d", abstract.FunctionContainerWebAdminHTTPPort)
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

func (p *Processor) listenOnRestartTriggerChannel() {
	for {
		select {
		case triggerInstance := <-p.restartTriggerChan:

			p.logger.WarnWith("Restarting trigger",
				"triggerKind", triggerInstance.GetKind(),
				"triggerID", triggerInstance.GetID())
			if err := p.restartTrigger(triggerInstance); err != nil {

				p.logger.ErrorWith("Failed to restart trigger",
					"err", err.Error())

				if err := p.hardRestartTriggerWorkers(triggerInstance); err != nil {

					p.logger.ErrorWith("Failed to restart at least one of the trigger's workers",
						"err", err.Error())

					// set the workers runtime status to Error
					p.setWorkersStatus(triggerInstance, status.Error) // nolint: errcheck
				}
			}

		// stop listening when the processor stops
		case <-p.stopRestartTriggerRoutine:
			return
		}
	}
}

func (p *Processor) restartTrigger(triggerInstance trigger.Trigger) error {

	// force stop the trigger
	p.logger.InfoWith("Stopping trigger",
		"kind", triggerInstance.GetKind(),
		"name", triggerInstance.GetName())

	if _, err := triggerInstance.Stop(true); err != nil {
		p.logger.ErrorWith("Failed to stop trigger",
			"kind", triggerInstance.GetKind(),
			"name", triggerInstance.GetName(),
			"err", err.Error())
		return errors.Wrap(err, "Failed to stop trigger")
	}

	// start the trigger again
	p.logger.InfoWith("Starting trigger",
		"kind", triggerInstance.GetKind(),
		"name", triggerInstance.GetName())

	if err := triggerInstance.Start(nil); err != nil {
		p.logger.ErrorWith("Failed to start trigger",
			"kind", triggerInstance.GetKind(),
			"name", triggerInstance.GetName(),
			"err", err.Error())
		return errors.Wrap(err, "Failed to start trigger")
	}

	return nil
}

func (p *Processor) hardRestartTriggerWorkers(triggerInstance trigger.Trigger) error {

	p.logger.DebugWith("Restarting trigger workers",
		"triggerKind", triggerInstance.GetKind(),
		"triggerName", triggerInstance.GetName())

	// iterate over the trigger's workers and force restart each of them
	for _, workerInstance := range triggerInstance.GetWorkers() {
		if err := workerInstance.Restart(); err != nil {
			return errors.Wrap(err, "Failed to restart worker")
		}
	}

	return nil
}

func (p *Processor) setWorkersStatus(triggerInstance trigger.Trigger, status status.Status) error {

	p.logger.DebugWith("Setting trigger workers status",
		"triggerKind", triggerInstance.GetKind(),
		"triggerName", triggerInstance.GetName(),
		"status", status)

	// iterate over the trigger's workers and set their status
	for _, workerInstance := range triggerInstance.GetWorkers() {

		workerInstance.GetRuntime().SetStatus(status)
	}

	return nil
}

// handleSignals creates a signal handler, so on signal processor gracefully terminated
func (p *Processor) handleSignals() {
	var captureSignal = make(chan os.Signal, 1)

	// when k8s deletes pods, it sends SIGTERM (https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination)
	// when docker stops container it sends SIGTERM by default (https://docs.docker.com/engine/reference/commandline/stop/)
	// but you can specify a specific signal with a specific option, so we support SIGABRT and SIGINT as well
	signal.Notify(captureSignal, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGINT)
	p.terminateAllTriggers(<-captureSignal)
	p.Stop()
}

func (p *Processor) terminateAllTriggers(signal os.Signal) {
	p.logger.WarnWith("Got system signal", "signal", signal.String())

	wg := &sync.WaitGroup{}
	for _, triggerInstance := range p.triggers {
		wg.Add(1)

		// drains all workers in trigger (for each trigger in parallel)
		go func(triggerInstance trigger.Trigger, wg *sync.WaitGroup) {
			defer wg.Done()
			if err := triggerInstance.SignalWorkerDraining(); err != nil {
				p.logger.WarnWith("Failed to signal worker draining",
					"triggerKind", triggerInstance.GetKind(),
					"triggerName", triggerInstance.GetName(),
					"err", err.Error())
			}
		}(triggerInstance, wg)
	}
	wg.Wait()
	p.logger.Info("All triggers are terminated")
}
