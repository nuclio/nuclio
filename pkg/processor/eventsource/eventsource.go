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

	nuclio "github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/pkg/errors"
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
	timeout time.Duration) (response interface{}, submitError error, processError error) {

	defer func() {
		if err := recover(); err != nil {
			aes.Logger.ErrorWith("error during event handlers", "err", err)
			response = nil
			submitError = fmt.Errorf("error during event handler - %s", err)
			processError = nil
		}
	}()

	// set event source info provider (ourselves)
	event.SetSourceProvider(aes)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	response, err = workerInstance.ProcessEvent(event)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to process event"), nil
	}

	return response, nil, nil
}

func (aes *AbstractEventSource) SubmitEventsToWorker(events []nuclio.Event,
	timeout time.Duration) (res []interface{}, err error, errs []error) {

	defer func() {
		if err := recover(); err != nil {
			aes.Logger.ErrorWith("error handling events", "err", err)

			res = nil
			err = fmt.Errorf("error handdling event - %s", err)
			errs = nil
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

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	// iterate over events and process them at the worker
	for _, event := range events {

		response, err := workerInstance.ProcessEvent(event)

		// add response and error
		eventResponses = append(eventResponses, response)
		eventErrors = append(eventErrors, err)
	}

	return eventResponses, nil, eventErrors
}
