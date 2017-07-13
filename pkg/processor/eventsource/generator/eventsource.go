package generator

import (
	"errors"
	"math/rand"
	"time"

	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type generator struct {
	eventsource.AbstractEventSource
	configuration *Configuration
}

func newEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("Generator event source requires a shareable worker allocator")
	}

	newEventSource := generator{
		AbstractEventSource: eventsource.AbstractEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "generator",
		},
		configuration: configuration,
	}

	return &newEventSource, nil
}

func (g *generator) Start(checkpoint eventsource.Checkpoint) error {
	g.Logger.InfoWith("Starting", "numWorkers", g.configuration.NumWorkers)

	// seed RNG
	rand.Seed(time.Now().Unix())

	// spawn go routines that each allocate a worker, process an event and then sleep
	for generatorIndex := 0; generatorIndex < g.configuration.NumWorkers; generatorIndex++ {
		go g.generateEvents()
	}

	return nil
}

func (g *generator) Stop(force bool) (eventsource.Checkpoint, error) {

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
