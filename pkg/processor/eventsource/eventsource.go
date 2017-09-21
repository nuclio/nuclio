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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	nuclio "github.com/nuclio/nuclio-sdk"
	"runtime/debug"
)

type Checkpoint *string

type EventSource interface {

	// start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint Checkpoint) error

	// stop creating events. returns the current checkpoint
	Stop(force bool) (Checkpoint, error)

	// get the class of source (sync, async, etc)
	GetClass() string

	// get specific kind of source (http, rabbit mq, etc)
	GetKind() string

	// get the configuration
	GetConfig() map[string]interface{}

	// get direct access to workers for things like housekeeping / management
	// TODO: locks and such when relevant
	GetWorkers() []*worker.Worker
}

type AbstractEventSource struct {
	Logger          nuclio.Logger
	WorkerAllocator worker.WorkerAllocator
	Class           string
	Kind            string
}

func (aes *AbstractEventSource) GetClass() string {
	return aes.Class
}

func (aes *AbstractEventSource) GetKind() string {
	return aes.Kind
}

func (aes *AbstractEventSource) SubmitEventToWorker(event nuclio.Event,
	functionLogger nuclio.Logger,
	timeout time.Duration) (response interface{}, submitError error, processError error) {

	var workerInstance *worker.Worker

	defer func() {
		if err := recover(); err != nil {
			callStack := debug.Stack()

			functionLogger.ErrorWith("Panic caught during submit event",
				"err",
				err,
				"stack",
				string(callStack))

			submitError = fmt.Errorf("Caught panic: %s", err)

			response = nil
			processError = nil

			if workerInstance != nil {
				aes.WorkerAllocator.Release(workerInstance)
			}
		}
	}()

	// set event source info provider (ourselves)
	event.SetSourceProvider(aes)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	response, processError = workerInstance.ProcessEvent(event, functionLogger)

	// release worker when we're done
	aes.WorkerAllocator.Release(workerInstance)

	return
}

func (aes *AbstractEventSource) SubmitEventsToWorker(events []nuclio.Event,
	functionLogger nuclio.Logger,
	timeout time.Duration) (responses []interface{}, submitError error, processErrors []error) {

	var workerInstance *worker.Worker

	defer func() {
		if err := recover(); err != nil {
			callStack := debug.Stack()

			functionLogger.ErrorWith("Panic caught during submit events",
				"err",
				err,
				"stack",
				string(callStack))

			submitError = fmt.Errorf("Caught panic: %s", err)

			responses = nil
			processErrors = nil

			if workerInstance != nil {
				aes.WorkerAllocator.Release(workerInstance)
			}
		}
	}()

	// create responses / errors slice
	eventResponses := make([]interface{}, 0, len(events))
	eventErrors := make([]error, 0, len(events))

	// set event source info provider (ourselves)
	for _, event := range events {
		event.SetSourceProvider(aes)
	}

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// iterate over events and process them at the worker
	for _, event := range events {

		response, err := workerInstance.ProcessEvent(event, functionLogger)

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
