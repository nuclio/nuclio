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

package eventsource

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	nuclio "github.com/nuclio/nuclio-sdk"
)

type Checkpoint *string

type EventSource interface {

	// start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint Checkpoint) error

	// stop creating events. returns the current checkpoint
	Stop(force bool) (Checkpoint, error)

	// get the user given ID for this event source
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

type AbstractEventSource struct {
	ID              string
	Logger          nuclio.Logger
	WorkerAllocator worker.Allocator
	Class           string
	Kind            string
	Statistics      Statistics
}

func (aes *AbstractEventSource) GetClass() string {
	return aes.Class
}

func (aes *AbstractEventSource) GetKind() string {
	return aes.Kind
}

func (aes *AbstractEventSource) AllocateWorkerAndSubmitEvent(event nuclio.Event,
	functionLogger nuclio.Logger,
	timeout time.Duration) (response interface{}, submitError error, processError error) {

	var workerInstance *worker.Worker

	defer aes.HandleSubmitPanic(&workerInstance, &submitError)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		aes.UpdateStatistics(false)

		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	response, processError = aes.SubmitEventToWorker(functionLogger, workerInstance, event)
	// release worker when we're done, in case of panic HandleSubmitPanic will release the worker
	aes.WorkerAllocator.Release(workerInstance)

	return
}

func (aes *AbstractEventSource) AllocateWorkerAndSubmitEvents(events []nuclio.Event,
	functionLogger nuclio.Logger,
	timeout time.Duration) (responses []interface{}, submitError error, processErrors []error) {

	var workerInstance *worker.Worker

	defer aes.HandleSubmitPanic(&workerInstance, &submitError)

	// create responses / errors slice
	eventResponses := make([]interface{}, 0, len(events))
	eventErrors := make([]error, 0, len(events))

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		aes.UpdateStatistics(false)

		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// iterate over events and process them at the worker
	for _, event := range events {

		response, err := aes.SubmitEventToWorker(functionLogger, workerInstance, event)

		// add response and error
		eventResponses = append(eventResponses, response)
		eventErrors = append(eventErrors, err)
	}

	// release worker
	aes.WorkerAllocator.Release(workerInstance)

	return eventResponses, nil, eventErrors
}

func (aes *AbstractEventSource) GetWorkers() []*worker.Worker {
	return aes.WorkerAllocator.GetWorkers()
}

// get statistics
func (aes *AbstractEventSource) GetStatistics() *Statistics {
	return &aes.Statistics
}

// get user given ID for this event source
func (aes *AbstractEventSource) GetID() string {
	return aes.ID
}

func (aes *AbstractEventSource) HandleSubmitPanic(workerInstance **worker.Worker,
	submitError *error) {

	if err := recover(); err != nil {
		callStack := debug.Stack()

		aes.Logger.ErrorWith("Panic caught during submit events",
			"err",
			err,
			"stack",
			string(callStack))

		*submitError = fmt.Errorf("Caught panic: %s", err)

		if workerInstance != nil {
			aes.WorkerAllocator.Release(*workerInstance)
		}

		aes.UpdateStatistics(false)
	}
}

func (aes *AbstractEventSource) SubmitEventToWorker(functionLogger nuclio.Logger,
	workerInstance *worker.Worker,
	event nuclio.Event) (response interface{}, processError error) {

	// set event source info provider (ourselves)
	event.SetSourceProvider(aes)

	response, processError = workerInstance.ProcessEvent(event, functionLogger)

	// increment statistics based on results. if process error is nil, we successfully handled
	aes.UpdateStatistics(processError == nil)

	return
}

func (aes *AbstractEventSource) UpdateStatistics(success bool) {
	if success {
		aes.Statistics.EventsHandleSuccessTotal++
	} else {
		aes.Statistics.EventsHandleFailureTotal++
	}
}
