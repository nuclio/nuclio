package event_source

import (
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
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

	// Config returns the event source configuration
	Config() map[string]interface{}

	// SetConfig sets the event source configuration
	SetConfig(cfg map[string]interface{})
}

type DefaultEventSource struct {
	Logger          logger.Logger
	WorkerAllocator worker.WorkerAllocator
	Class           string
	Kind            string
	cfg             map[string]interface{}
}

func (des *DefaultEventSource) GetClass() string {
	return des.Class
}

func (des *DefaultEventSource) GetKind() string {
	return des.Kind
}

func (des *DefaultEventSource) SubmitEventToWorker(event event.Event, timeout time.Duration) (interface{}, error, error) {

	// set event source info provider (ourselves)
	event.SetSourceProvider(des)

	// allocate a worker
	workerInstance, err := des.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, des.Logger.Report(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer des.WorkerAllocator.Release(workerInstance)

	response, err := workerInstance.ProcessEvent(event)
	if err != nil {
		return nil, des.Logger.Report(err, "Failed to process event"), nil
	}

	return response, nil, nil
}

func (des *DefaultEventSource) Config() map[string]interface{} {
	return des.cfg
}

func (des *DefaultEventSource) SetConfig(cfg map[string]interface{}) {
	des.cfg = cfg
}
