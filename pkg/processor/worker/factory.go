package worker

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type WorkerFactory struct{}

// global singleton
var WorkerFactorySingleton = WorkerFactory{}

func (waf *WorkerFactory) CreateFixedPoolWorkerAllocator(logger logger.Logger,
	numWorkers int,
	runtimeConfiguration *viper.Viper) (WorkerAllocator, error) {

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

func (waf *WorkerFactory) CreateSingletonPoolWorkerAllocator(logger logger.Logger,
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

func (waf *WorkerFactory) createWorker(parentLogger logger.Logger,
	workerIndex int,
	runtimeConfiguration *viper.Viper) (*Worker, error) {

	// create logger parent
	workerLogger := parentLogger.GetChild(fmt.Sprintf("w%d", workerIndex)).(logger.Logger)

	// create a runtime for the worker
	runtimeInstance, err := runtime.RegistrySingleton.NewRuntime(workerLogger,
		runtimeConfiguration.GetString("kind"),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return NewWorker(workerLogger, workerIndex, runtimeInstance), nil
}

func (waf *WorkerFactory) createWorkers(logger logger.Logger,
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
