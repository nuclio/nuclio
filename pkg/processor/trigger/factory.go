package trigger

import (
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type Factory struct{}

func (f *Factory) GetWorkerAllocator(workerAllocatorName string,
	namedWorkerAllocators *worker.AllocatorSyncMap,
	workerAllocatorCreator func() (worker.Allocator, error)) (worker.Allocator, error) {

	return namedWorkerAllocators.LoadOrStore(workerAllocatorName, workerAllocatorCreator)
}
