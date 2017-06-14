package event_source

import (
	"expvar"
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

const (
	CountMetric   = "num_events"
	ErrorMetric   = "num_errors"
	StartMetric   = "start_time"
	StoppedMetric = "stop_time"
)

type EventState int

const (
	StoppedState EventState = iota
	RunningState
)

type Checkpoint *string

type EventSource interface {

	// Start starts creating events from a given checkpoint
	// (nil - no checkpoint)
	Start(checkpoint Checkpoint) error

	// Stop will stop creating events. returns the current checkpoint
	Stop(force bool) (Checkpoint, error)

	// Class returns the class of source (sync, async, etc)
	Class() string

	// Kind return the kind of source (http, rabbit mq, etc)
	Kind() string

	// Config returns the event source configuration
	Config() map[string]interface{}

	// SetConfig sets the event source configuration
	SetConfig(cfg map[string]interface{})

	// Stats returns statistics for event source
	Stats() *expvar.Map

	// Start return the current state
	State() EventState
}

type AbstractEventSource struct {
	Logger          logger.Logger
	WorkerAllocator worker.WorkerAllocator

	cfg   map[string]interface{}
	class string
	kind  string
	state EventState
	stats *expvar.Map
}

func NewAbstractEventSource(logger logger.Logger, allocator worker.WorkerAllocator, class, kind string) *AbstractEventSource {
	es := &AbstractEventSource{
		Logger:          logger,
		WorkerAllocator: allocator,
		class:           class,
		kind:            kind,
		state:           StoppedState,
	}
	es.stats = new(expvar.Map).Init()

	return es
}

func now() expvar.Var {
	v := &expvar.String{}
	v.Set(time.Now().Format(time.RFC3339))
	return v
}

func (aes *AbstractEventSource) Init() {
	stats := aes.Stats()
	stats.Set(StartMetric, now())
	stats.Add(CountMetric, 0)
	stats.Add(ErrorMetric, 0)

	aes.state = RunningState
}

func (aes *AbstractEventSource) Shutdown() {
	stats := aes.Stats()
	stats.Set(StoppedMetric, now())

	aes.state = StoppedState
}

func (aes *AbstractEventSource) Class() string {
	return aes.class
}

func (aes *AbstractEventSource) Kind() string {
	return aes.kind
}

func (aes *AbstractEventSource) SubmitEventToWorker(event event.Event, timeout time.Duration) (interface{}, error, error) {

	// set event source info provider (ourselves)
	event.SetSourceProvider(aes)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, aes.Logger.Report(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	response, err := workerInstance.ProcessEvent(event)
	if err != nil {
		return nil, aes.Logger.Report(err, "Failed to process event"), nil
	}

	return response, nil, nil
}

func (aes *AbstractEventSource) Config() map[string]interface{} {
	return aes.cfg
}

func (aes *AbstractEventSource) SetConfig(cfg map[string]interface{}) {
	aes.cfg = cfg
}

func (aes *AbstractEventSource) Stats() *expvar.Map {
	return aes.stats
}

func (aes *AbstractEventSource) State() EventState {
	return aes.state
}
