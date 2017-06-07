package event_source

import (
	"expvar"
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

const (
	StartMetric = "start_time"
	CountMetric = "num_events"
	ErrorMetric = "num_errors"
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

	// Stats returns statistics for event source
	Stats() *expvar.Map
}

type DefaultEventSource struct {
	Logger          logger.Logger
	WorkerAllocator worker.WorkerAllocator
	Class           string
	Kind            string
	cfg             map[string]interface{}
	stats           *expvar.Map
}

func NewDefaultEventSource(logger logger.Logger, allocator worker.WorkerAllocator, class, kind string) *DefaultEventSource {
	es := &DefaultEventSource{
		Logger:          logger,
		WorkerAllocator: allocator,
		Class:           class,
		Kind:            kind,
	}
	es.stats = new(expvar.Map).Init()

	return es
}

func (des *DefaultEventSource) StartMetrics() {
	v := &expvar.String{}
	v.Set(time.Now().Format(time.RFC3339))

	stats := des.Stats()
	stats.Set(StartMetric, v)
	stats.Add(CountMetric, 0)
	stats.Add(ErrorMetric, 0)
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

func (des *DefaultEventSource) Stats() *expvar.Map {
	return des.stats
}
