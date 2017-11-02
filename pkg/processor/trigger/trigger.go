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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	nuclio "github.com/nuclio/nuclio-sdk"
)

type Checkpoint *string

type Trigger interface {

	// start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint Checkpoint) error

	// stop creating events. returns the current checkpoint
	Stop(force bool) (Checkpoint, error)

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

type AbstractTrigger struct {
	ID              string
	Logger          nuclio.Logger
	WorkerAllocator worker.Allocator
	Class           string
	Kind            string
	Statistics      Statistics
}

func (at *AbstractTrigger) GetClass() string {
	return at.Class
}

func (at *AbstractTrigger) GetKind() string {
	return at.Kind
}

func (at *AbstractTrigger) AllocateWorkerAndSubmitEvent(event nuclio.Event,
	functionLogger nuclio.Logger,
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

func (at *AbstractTrigger) AllocateWorkerAndSubmitEvents(events []nuclio.Event,
	functionLogger nuclio.Logger,
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

func (at *AbstractTrigger) GetWorkers() []*worker.Worker {
	return at.WorkerAllocator.GetWorkers()
}

// get statistics
func (at *AbstractTrigger) GetStatistics() *Statistics {
	return &at.Statistics
}

// get user given ID for this trigger
func (at *AbstractTrigger) GetID() string {
	return at.ID
}

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

func (at *AbstractTrigger) SubmitEventToWorker(functionLogger nuclio.Logger,
	workerInstance *worker.Worker,
	event nuclio.Event) (response interface{}, processError error) {

	// set trigger info provider (ourselves)
	event.SetSourceProvider(at)

	response, processError = workerInstance.ProcessEvent(event, functionLogger)

	// increment statistics based on results. if process error is nil, we successfully handled
	at.UpdateStatistics(processError == nil)
	return
}

func (at *AbstractTrigger) UpdateStatistics(success bool) {
	if success {
		at.Statistics.EventsHandleSuccessTotal++
	} else {
		at.Statistics.EventsHandleFailureTotal++
	}
}
