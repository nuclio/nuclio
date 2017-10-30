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

package generator

import (
	"errors"
	"math/rand"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
)

type generator struct {
	trigger.AbstractTrigger
	configuration *Configuration
}

func newTrigger(logger nuclio.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("Generator trigger requires a shareable worker allocator")
	}

	newTrigger := generator{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "generator",
		},
		configuration: configuration,
	}

	return &newTrigger, nil
}

func (g *generator) Start(checkpoint trigger.Checkpoint) error {
	g.Logger.InfoWith("Starting", "numWorkers", g.configuration.NumWorkers)

	// seed RNG
	rand.Seed(time.Now().Unix())

	// spawn go routines that each allocate a worker, process an event and then sleep
	for generatorIndex := 0; generatorIndex < g.configuration.NumWorkers; generatorIndex++ {
		go g.generateEvents()
	}

	return nil
}

func (g *generator) Stop(force bool) (trigger.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (g *generator) GetConfig() map[string]interface{} {
	return common.StructureToMap(g.configuration)
}

func (g *generator) generateEvents() error {
	event := processor.AbstractSync{}

	// for ever (for now)
	for {
		g.AllocateWorkerAndSubmitEvent(&event, nil, 10*time.Second)

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
