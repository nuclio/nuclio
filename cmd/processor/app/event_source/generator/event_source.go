package generator

import (
	"errors"
	"math/rand"
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type generator struct {
	event_source.AbstractEventSource
	configuration *Configuration
}

func newEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (event_source.EventSource, error) {

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("Generator event source requires a shareable worker allocator")
	}

	newEventSource := generator{
		AbstractEventSource: event_source.AbstractEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "generator",
		},
		configuration: configuration,
	}

	return &newEventSource, nil
}

func (g *generator) Start(checkpoint event_source.Checkpoint) error {
	g.Logger.With(logger.Fields{
		"numWorkers": g.configuration.NumWorkers,
	}).Info("Starting")

	// seed RNG
	rand.Seed(time.Now().Unix())

	// spawn go routines that each allocate a worker, process an event and then sleep
	for generatorIndex := 0; generatorIndex < g.configuration.NumWorkers; generatorIndex++ {
		go g.generateEvents()
	}

	return nil
}

func (g *generator) Stop(force bool) (event_source.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (g *generator) generateEvents() error {
	event := event.AbstractSync{}

	// for ever (for now)
	for {
		g.SubmitEventToWorker(&event, 10*time.Second)

		var sleepMs int

		// randomize sleep
		if g.configuration.MaxDelayMs != g.configuration.MinDelayMs {
			sleepMs = rand.Intn(g.configuration.MaxDelayMs-g.configuration.MinDelayMs) + g.configuration.MinDelayMs
		} else {
			sleepMs = g.configuration.MinDelayMs
		}

		// sleep a bit
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}
}
