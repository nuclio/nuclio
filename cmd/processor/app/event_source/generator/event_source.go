package generator

import (
	"errors"
	"math/rand"
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
)

type generator struct {
	event_source.DefaultEventSource
	numWorkers int
	minDelayMs int
	maxDelayMs int
}

func NewEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	numWorkers int,
	minDelayMs int,
	maxDelayMs int) (event_source.EventSource, error) {

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("Generator event source requires a shareable worker allocator")
	}

	newEventSource := generator{
		DefaultEventSource: event_source.DefaultEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "generator",
		},
		numWorkers: numWorkers,
		minDelayMs: minDelayMs,
		maxDelayMs: maxDelayMs,
	}

	return &newEventSource, nil
}

func (g *generator) Start(checkpoint event_source.Checkpoint) error {
	g.Logger.With(logger.Fields{
		"numWorkers": g.numWorkers,
	}).Info("Starting")

	// seed RNG
	rand.Seed(time.Now().Unix())

	// spawn go routines that each allocate a worker, process an event and then sleep
	for generatorIndex := 0; generatorIndex < g.numWorkers; generatorIndex++ {
		go g.generateEvents()
	}

	return nil
}

func (g *generator) Stop(force bool) (event_source.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (g *generator) generateEvents() error {
	event := event.DefaultSync{}

	// for ever (for now)
	for {
		g.SubmitEventToWorker(&event, 10*time.Second)

		var sleepMs int

		// randomize sleep
		if g.maxDelayMs != g.minDelayMs {
			sleepMs = rand.Intn(g.maxDelayMs-g.minDelayMs) + g.minDelayMs
		} else {
			sleepMs = g.minDelayMs
		}

		// sleep a bit
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}
}
