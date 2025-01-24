package eventprocessor

import (
	"errors"
	"time"

	"github.com/nuclio/logger"
)

type asyncSingletonAllocator struct {
	logger       logger.Logger
	object       EventProcessor
	isTerminated bool
}

func NewSingletonAllocator(parentLogger logger.Logger, eventProcessor EventProcessor) Allocator {

	return &asyncSingletonAllocator{
		logger: parentLogger.GetChild("singelton_allocator"),
		object: eventProcessor,
	}
}

func (s *asyncSingletonAllocator) Allocate(time.Duration) (EventProcessor, error) {
	if s.isTerminated {
		return nil, ErrAllWorkersAreTerminated
	}
	return s.object, nil
}

func (s *asyncSingletonAllocator) SetObject(object EventProcessor) error {
	s.object = object
	return nil
}

func (s *asyncSingletonAllocator) SetObjects(objects []EventProcessor) error {
	if len(objects) == 0 {
		return errors.New("Length of setting objects is zero")
	}
	s.object = objects[0]
	return nil
}

func (s *asyncSingletonAllocator) Release(processor EventProcessor) {
}

func (s *asyncSingletonAllocator) Shareable() bool {
	return false
}

func (s *asyncSingletonAllocator) GetObjects() []EventProcessor {
	return []EventProcessor{s.object}
}

func (s *asyncSingletonAllocator) GetNumWorkersAvailable() int {
	return 1
}

// GetStatistics returns worker allocator statistics
func (s *asyncSingletonAllocator) GetStatistics() *AllocatorStatistics {
	return nil
}

func (s *asyncSingletonAllocator) SignalDraining() error {
	return s.object.Drain()
}

func (s *asyncSingletonAllocator) SignalContinue() error {
	return s.object.Continue()
}

func (s *asyncSingletonAllocator) SignalTermination() error {
	s.isTerminated = true
	return s.object.Terminate()
}

func (s *asyncSingletonAllocator) IsTerminated() bool {
	return s.isTerminated
}
