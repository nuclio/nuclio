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

package trigger

import (
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/google/uuid"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const (
	MaxWorkersLimit                              = 100000
	DefaultWorkerAvailabilityTimeoutMilliseconds = 10000 // 10 seconds
)

// Trigger is common trigger interface
type Trigger interface {

	// Initialize performs post creation initializations
	Initialize() error

	// Start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint functionconfig.Checkpoint) error

	// Stop creating events. returns the current checkpoint
	Stop(force bool) (functionconfig.Checkpoint, error)

	// GetID returns the user given ID for this trigger
	GetID() string

	// GetClass returns the class of source (sync, async, etc)
	GetClass() string

	// GetKind returns the specific kind of source (http, rabbit mq, etc)
	GetKind() string

	// GetName returns the trigger name
	GetName() string

	// GetConfig returns trigger configuration
	GetConfig() map[string]interface{}

	// GetStatistics returns the trigger statistics
	GetStatistics() *Statistics

	// GetWorkers gets direct access to workers for things like housekeeping / management
	// TODO: locks and such when relevant
	GetWorkers() []*worker.Worker

	// GetNamespace returns namespace
	GetNamespace() string

	// GetFunctionName returns function name
	GetFunctionName() string

	// GetProjectName returns project name
	GetProjectName() string

	// TimeoutWorker times out a worker
	TimeoutWorker(worker *worker.Worker) error
}

// AbstractTrigger implements common trigger operations
type AbstractTrigger struct {
	Trigger Trigger

	// accessed atomically, keep as first field for alignment
	Statistics Statistics

	ID              string
	Logger          logger.Logger
	WorkerAllocator worker.Allocator
	Class           string
	Kind            string
	Name            string
	Namespace       string
	FunctionName    string
	ProjectName     string
	restartChan     chan Trigger
}

func NewAbstractTrigger(logger logger.Logger,
	allocator worker.Allocator,
	configuration *Configuration,
	class string,
	kind string,
	name string,
	restartTriggerChan chan Trigger) (AbstractTrigger, error) {

	// enrich default trigger configuration
	if configuration.WorkerAvailabilityTimeoutMilliseconds == nil || *configuration.WorkerAvailabilityTimeoutMilliseconds < 0 {
		logger.InfoWith("Setting default worker availability timeout",
			"DefaultWorkerAvailabilityTimeoutMilliseconds",
			DefaultWorkerAvailabilityTimeoutMilliseconds)

		defaultWorkerAvailabilityTimeoutMilliseconds := DefaultWorkerAvailabilityTimeoutMilliseconds
		configuration.WorkerAvailabilityTimeoutMilliseconds = &defaultWorkerAvailabilityTimeoutMilliseconds
	}

	return AbstractTrigger{
		Logger:          logger,
		ID:              configuration.ID,
		WorkerAllocator: allocator,
		Class:           class,
		Kind:            kind,
		Name:            name,
		Namespace:       configuration.RuntimeConfiguration.Meta.Namespace,
		FunctionName:    configuration.RuntimeConfiguration.Meta.Name,
		ProjectName:     configuration.RuntimeConfiguration.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		restartChan:     restartTriggerChan,
	}, nil
}

// Initialize performs post creation initializations
func (at *AbstractTrigger) Initialize() error {
	return nil
}

// GetClass returns the class
func (at *AbstractTrigger) GetClass() string {
	return at.Class
}

// GetKind return the kind
func (at *AbstractTrigger) GetKind() string {
	return at.Kind
}

// GetName returns the name
func (at *AbstractTrigger) GetName() string {
	return at.Name
}

// AllocateWorkerAndSubmitEvent submits event to allocated worker
func (at *AbstractTrigger) AllocateWorkerAndSubmitEvent(event nuclio.Event,
	functionLogger logger.Logger,
	timeout time.Duration) (response interface{}, submitError error, processError error) {

	var workerInstance *worker.Worker

	defer at.HandleSubmitPanic(workerInstance, &submitError)

	// allocate a worker
	workerInstance, err := at.WorkerAllocator.Allocate(timeout)
	if err != nil {
		at.UpdateStatistics(false)

		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	response, processError = at.SubmitEventToWorker(functionLogger, workerInstance, event)

	// release worker when we're done
	at.WorkerAllocator.Release(workerInstance)

	return
}

// AllocateWorkerAndSubmitEvents submits multiple events to an allocated worker
func (at *AbstractTrigger) AllocateWorkerAndSubmitEvents(events []nuclio.Event,
	functionLogger logger.Logger,
	timeout time.Duration) (responses []interface{}, submitError error, processErrors []error) {
	var workerInstance *worker.Worker

	defer at.HandleSubmitPanic(workerInstance, &submitError)

	// create responses / errors slice
	eventResponses := make([]interface{}, 0, len(events))
	eventErrors := make([]error, 0, len(events))

	// allocate a worker
	workerInstance, err := at.WorkerAllocator.Allocate(timeout)
	if err != nil {
		at.UpdateStatistics(false)

		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// iterate over events and process them at the worker
	for _, event := range events {

		response, err := at.SubmitEventToWorker(functionLogger, workerInstance, event)

		// add response and error
		eventResponses = append(eventResponses, response)
		eventErrors = append(eventErrors, err)
	}

	// release worker
	at.WorkerAllocator.Release(workerInstance)

	return eventResponses, nil, eventErrors
}

// GetWorkers returns the list of workers
func (at *AbstractTrigger) GetWorkers() []*worker.Worker {
	return at.WorkerAllocator.GetWorkers()
}

// GetStatistics returns trigger statistics
func (at *AbstractTrigger) GetStatistics() *Statistics {

	// copy worker allocator statistics
	at.Statistics.WorkerAllocatorStatistics = *at.WorkerAllocator.GetStatistics()

	return &at.Statistics
}

// GetID returns user given ID for this trigger
func (at *AbstractTrigger) GetID() string {
	return at.ID
}

// GetNamespace returns namespace of function
func (at *AbstractTrigger) GetNamespace() string {
	return at.Namespace
}

// GetFunctionName returns function name
func (at *AbstractTrigger) GetFunctionName() string {
	return at.FunctionName
}

// GetProjectName returns project name
func (at *AbstractTrigger) GetProjectName() string {
	return at.ProjectName
}

// HandleSubmitPanic handles a panic when submitting to worker
func (at *AbstractTrigger) HandleSubmitPanic(workerInstance *worker.Worker,
	submitError *error) {

	if err := recover(); err != nil {
		callStack := debug.Stack()

		at.Logger.ErrorWith("Panic caught during submit events",
			"err",
			err,
			"stack",
			string(callStack))

		*submitError = errors.Errorf("Caught panic: %s", err)

		if workerInstance != nil {
			workerInstance.ResetEventTime()
			at.WorkerAllocator.Release(workerInstance)
		}

		at.UpdateStatistics(false)
	}
}

// SubmitEventToWorker submits events to worker and returns response
func (at *AbstractTrigger) SubmitEventToWorker(functionLogger logger.Logger,
	workerInstance *worker.Worker,
	event nuclio.Event) (response interface{}, processError error) {

	event, err := at.prepareEvent(event, workerInstance)
	if err != nil {
		return nil, err
	}

	response, processError = workerInstance.ProcessEvent(event, functionLogger)

	// increment statistics based on results. if process error is nil, we successfully handled
	at.UpdateStatistics(processError == nil)
	return
}

// TimeoutWorker times out a worker
func (at *AbstractTrigger) TimeoutWorker(worker *worker.Worker) error {
	return nil
}

// UpdateStatistics updates the trigger statistics
func (at *AbstractTrigger) UpdateStatistics(success bool) {
	if success {
		atomic.AddUint64(&at.Statistics.EventsHandledSuccessTotal, 1)
	} else {
		atomic.AddUint64(&at.Statistics.EventsHandledFailureTotal, 1)
	}
}

// Restart signals the processor to start the trigger restart procedure
func (at *AbstractTrigger) Restart() error {
	at.Logger.Warn("Restart called in trigger", "triggerKind", at.GetKind(), "triggerName", at.GetName())

	// signal the processor to restart the trigger
	at.restartChan <- at.Trigger

	return nil
}

// SubscribeToControlMessageKind subscribes all workers to control message kind
func (at *AbstractTrigger) SubscribeToControlMessageKind(kind controlcommunication.ControlMessageKind,
	controlMessageChan chan *controlcommunication.ControlMessage) error {

	at.Logger.DebugWith("Subscribing to control message kind",
		"kind", kind,
		"numWorkers", len(at.WorkerAllocator.GetWorkers()))

	for _, workerInstance := range at.WorkerAllocator.GetWorkers() {
		if err := workerInstance.Subscribe(kind, controlMessageChan); err != nil {
			return errors.Wrapf(err,
				"Failed to subscribe to control message kind %s in worker %d",
				kind,
				workerInstance.GetIndex())
		}
	}

	return nil
}

// UnsubscribeFromControlMessageKind unsubscribes all workers from control message kind
func (at *AbstractTrigger) UnsubscribeFromControlMessageKind(kind controlcommunication.ControlMessageKind,
	controlMessageChan chan *controlcommunication.ControlMessage) error {

	at.Logger.DebugWith("Unsubscribing channel from control message kind",
		"kind", kind,
		"numWorkers", len(at.WorkerAllocator.GetWorkers()))

	for _, workerInstance := range at.WorkerAllocator.GetWorkers() {
		if err := workerInstance.Unsubscribe(kind, controlMessageChan); err != nil {
			return errors.Wrapf(err,
				"Failed to unsubscribe channel from control message kind %s in worker %d",
				kind,
				workerInstance.GetIndex())
		}
	}

	return nil
}

// SignalWorkerDraining sends a signal to all workers, telling them to drop or ack events
// that are currently being processed
func (at *AbstractTrigger) SignalWorkerDraining(workerDrainingCompleteChan chan bool) {

	// signal all workers to drain
	if err := at.WorkerAllocator.SignalDraining(); err != nil {
		at.Logger.WarnWith("Failed to signal all workers to drain events", "err", err.Error())
	}

	// signal draining complete
	workerDrainingCompleteChan <- true
}

// ResetWorkerTerminationState resets the worker termination state
func (at *AbstractTrigger) ResetWorkerTerminationState() {
	at.WorkerAllocator.ResetTerminationState()
}

func (at *AbstractTrigger) prepareEvent(event nuclio.Event, workerInstance *worker.Worker) (nuclio.Event, error) {

	// if the content type starts with application/cloudevents, the body
	// contains a structured cloud event (a JSON encoded structure)
	// https://github.com/cloudevents/spec/blob/master/json-format.md
	if strings.HasPrefix(event.GetContentType(), "application/cloudevents") {

		// use the structured cloudevent stored in the worker to wrap this existing event
		structuredCloudEvent := workerInstance.GetStructuredCloudEvent()

		// wrap the received event
		if err := structuredCloudEvent.SetEvent(event); err != nil {
			return nil, errors.Wrap(err, "Failed to wrap structured cloud event")
		}

		return structuredCloudEvent, nil
	}

	// if body does not encode a structured cloudevent, check if this is a
	// binary cloud event by checking the existence of the
	// "CE-CloudEventsVersion" header
	if event.GetHeaderString("CE-CloudEventsVersion") != "" {

		// use the structured cloudevent stored in the worker to wrap this existing event
		binaryCloudEvent := workerInstance.GetBinaryCloudEvent()

		// wrap the received event
		if err := binaryCloudEvent.SetEvent(event); err != nil {
			return nil, errors.Wrap(err, "Failed to wrap binary cloud event")
		}

		return binaryCloudEvent, nil
	}

	// Not a cloud event
	event.SetID(nuclio.ID(uuid.New().String()))
	event.SetTriggerInfoProvider(at)
	return event, nil
}
