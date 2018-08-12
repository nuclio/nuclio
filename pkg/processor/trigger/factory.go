package trigger

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type Factory struct{}

func (f *Factory) GetWorkerAllocator(workerAllocatorName string,
	namedWorkerAllocators map[string]worker.Allocator,
	workerAllocatorCreator func() (worker.Allocator, error)) (worker.Allocator, error) {

	// if our allocator is unnamed, just create a worker allocator
	if workerAllocatorName == "" {
		return workerAllocatorCreator()
	}

	// try to find worker allocator
	workerAllocator, workerAllocatorExists := namedWorkerAllocators[workerAllocatorName]

	// if it already exists, just use it
	if workerAllocatorExists {
		return workerAllocator, nil
	}

	// if it doesn't exist - create it
	workerAllocator, err := workerAllocatorCreator()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	namedWorkerAllocators[workerAllocatorName] = workerAllocator

	return workerAllocator, nil
}
