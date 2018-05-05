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

package trigger

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/satori/go.uuid"
)

type Trigger interface {

	// Initialize performs post creation initializations
	Initialize() error

	// start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint functionconfig.Checkpoint) error

	// stop creating events. returns the current checkpoint
	Stop(force bool) (functionconfig.Checkpoint, error)

	// get the user given ID for this trigger
	GetID() string

	// get the class of source (sync, async, etc)
	GetClass() string

	// get specific kind of source (http, rabbit mq, etc)
	GetKind() string

	// get the configuration
	GetConfig() map[string]interface{}

	// get statistics
	GetStatistics() *Statistics

	// get direct access to workers for things like housekeeping / management
	// TODO: locks and such when relevant
	GetWorkers() []*worker.Worker
}

// AbstractTrigger implements common trigger operations
type AbstractTrigger struct {
	ID              string
	Logger          logger.Logger
	WorkerAllocator worker.Allocator
	Class           string
	Kind            string
	Statistics      Statistics
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
	return &at.Statistics
}

// GetID returns user given ID for this trigger
func (at *AbstractTrigger) GetID() string {
	return at.ID
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

		*submitError = fmt.Errorf("Caught panic: %s", err)

		if workerInstance != nil {
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

// UpdateStatistics updates the trigger statistics
func (at *AbstractTrigger) UpdateStatistics(success bool) {
	if success {
		at.Statistics.EventsHandleSuccessTotal++
	} else {
		at.Statistics.EventsHandleFailureTotal++
	}
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
	event.SetID(nuclio.ID(uuid.NewV4().String()))
	event.SetTriggerInfoProvider(at)
	return event, nil
}
