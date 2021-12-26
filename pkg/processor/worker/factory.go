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
	"context"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Factory struct{}

// global singleton
var WorkerFactorySingleton = Factory{}

func (waf *Factory) CreateFixedPoolWorkerAllocator(logger logger.Logger,
	numWorkers int,
	runtimeConfiguration *runtime.Configuration) (Allocator, error) {

	logger.DebugWith("Creating worker pool", "num", numWorkers)

	// create the workers
	workers, err := waf.createWorkers(logger, numWorkers, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create workers")
	}

	// create an allocator
	workerAllocator, err := NewFixedPoolWorkerAllocator(logger, workers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
}

func (waf *Factory) CreateSingletonPoolWorkerAllocator(logger logger.Logger,
	runtimeConfiguration *runtime.Configuration) (Allocator, error) {

	// create the workers
	workerInstance, err := waf.createWorker(logger, 0, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP trigger")
	}

	// create an allocator
	workerAllocator, err := NewSingletonWorkerAllocator(logger, workerInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	return workerAllocator, nil
}

func (waf *Factory) createWorker(parentLogger logger.Logger,
	workerIndex int,
	runtimeConfiguration *runtime.Configuration) (*Worker, error) {

	// copy the runtime configuration since we need to specialize it for this specific runtime
	runtimeConfigurationCopy := *runtimeConfiguration
	runtimeConfigurationCopy.WorkerID = workerIndex

	// create logger parent
	workerLogger := parentLogger.GetChild(fmt.Sprintf("w%d", workerIndex))

	// get the runtime we need to load - if it has a colon, use the first part (e.g. golang:1.8 -> golang)
	runtimeKind := runtimeConfigurationCopy.Spec.Runtime
	runtimeKind = strings.Split(runtimeKind, ":")[0]

	// create a runtime for the worker
	runtimeInstance, err := runtime.RegistrySingleton.NewRuntime(workerLogger,
		runtimeKind,
		&runtimeConfigurationCopy)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	if err := runtimeInstance.Start(); err != nil {
		return nil, errors.Wrap(err, "Failed to start runtime")
	}

	return NewWorker(workerLogger, workerIndex, runtimeInstance)
}

func (waf *Factory) createWorkers(logger logger.Logger,
	numWorkers int,
	runtimeConfiguration *runtime.Configuration) ([]*Worker, error) {
	workers := make([]*Worker, numWorkers)

	errGroup, _ := errgroup.WithContext(context.Background(), logger)

	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		workerIndex := workerIndex

		errGroup.Go("Create worker", func() error {
			worker, err := waf.createWorker(logger, workerIndex, runtimeConfiguration)
			if err != nil {
				return errors.Wrap(err, "Failed to create worker")
			}

			workers[workerIndex] = worker

			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, errors.Wrap(err, "Failed to create workers")
	}

	return workers, nil
}
