package eventsource

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio-sdk/logger"
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
	Logger          logger.Logger
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

func (aes *AbstractEventSource) SubmitEventToWorker(eventInstance event.Event,
	timeout time.Duration) (res interface{}, err error, err2 error) {

	defer func() {
		if err := recover(); err != nil {
			aes.Logger.Error("error during event handler - %s", err)
			res = nil
			err = fmt.Errorf("error during event handler - %s", err)
			err2 = nil
		}
	}()

	// set event source info provider (ourselves)
	eventInstance.SetSourceProvider(aes)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	response, err := workerInstance.ProcessEvent(eventInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to process event"), nil
	}

	return response, nil, nil
}

func (aes *AbstractEventSource) SubmitEventsToWorker(events []event.Event,
	timeout time.Duration) (res []interface{}, err error, errs []error) {

	defer func() {
		if err := recover(); err != nil {
			aes.Logger.Error("error handling events - %s", err)

			res = nil
			err = fmt.Errorf("error handdling event - %s", err)
			errs = nil
		}
	}()

	// create responses / errors slice
	eventResponses := make([]interface{}, 0, len(events))
	eventErrors := make([]error, 0, len(events))

	// set event source info provider (ourselves)
	for _, eventInstance := range events {
		eventInstance.SetSourceProvider(aes)
	}

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	// iterate over events and process them at the worker
	for _, eventInstance := range events {

		response, err := workerInstance.ProcessEvent(eventInstance)

		// add response and error
		eventResponses = append(eventResponses, response)
		eventErrors = append(eventErrors, err)
	}

	return eventResponses, nil, eventErrors
}
