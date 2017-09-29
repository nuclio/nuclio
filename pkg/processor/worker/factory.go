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

package worker

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type WorkerFactory struct{}

// global singleton
var WorkerFactorySingleton = WorkerFactory{}

func (waf *WorkerFactory) CreateFixedPoolWorkerAllocator(logger nuclio.Logger,
	numWorkers int,
	runtimeConfiguration *viper.Viper) (WorkerAllocator, error) {

	logger.DebugWith("Creating worker pool", "num", numWorkers)

	// create the workers
	workers, err := waf.createWorkers(logger, numWorkers, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP event source")
	}

	// create an allocator
	workerAllocator, err := NewFixedPoolWorkerAllocator(logger, workers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
}

func (waf *WorkerFactory) CreateSingletonPoolWorkerAllocator(logger nuclio.Logger,
	runtimeConfiguration *viper.Viper) (WorkerAllocator, error) {

	// create the workers
	workerInstance, err := waf.createWorker(logger, 0, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP event source")
	}

	// create an allocator
	workerAllocator, err := NewSingletonWorkerAllocator(logger, workerInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
}

func (waf *WorkerFactory) createWorker(parentLogger nuclio.Logger,
	workerIndex int,
	runtimeConfiguration *viper.Viper) (*Worker, error) {

	// create logger parent
	workerLogger := parentLogger.GetChild(fmt.Sprintf("w%d", workerIndex)).(nuclio.Logger)

	// create a runtime for the worker
	runtimeInstance, err := runtime.RegistrySingleton.NewRuntime(workerLogger,
		runtimeConfiguration.GetString("kind"),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return NewWorker(workerLogger, workerIndex, runtimeInstance)
}

func (waf *WorkerFactory) createWorkers(logger nuclio.Logger,
	numWorkers int,
	runtimeConfiguration *viper.Viper) ([]*Worker, error) {
	workers := make([]*Worker, numWorkers)

	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		worker, err := waf.createWorker(logger, workerIndex, runtimeConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create worker")
		}

		workers[workerIndex] = worker
	}

	return workers, nil
}
