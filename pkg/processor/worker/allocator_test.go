package worker

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type AllocatorTestSuite struct {
	suite.Suite
	logger nuclio.Logger
}

func (suite *AllocatorTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)
}

func (suite *AllocatorTestSuite) TestSingletonAllocator() {
	worker1 := &Worker{}

	sa, err := NewSingletonWorkerAllocator(suite.logger, worker1)
	suite.NoError(err)
	suite.NotNil(sa)

	// allocate once, time should be ignored
	allocatedWorker, err := sa.Allocate(time.Hour)
	suite.NoError(err)
	suite.Equal(worker1, allocatedWorker)

	// allocate again, release doesn't need to happen
	allocatedWorker, err = sa.Allocate(time.Hour)
	suite.NoError(err)
	suite.Equal(worker1, allocatedWorker)

	// release shouldn't do anything
	suite.NotPanics(func() {sa.Release(worker1)})

	suite.False(sa.Shareable())
}

func (suite *AllocatorTestSuite) TestFixedPoolAllocator() {
	worker1 := &Worker{index: 0}
	worker2 := &Worker{index: 1}
	workers := []*Worker{worker1, worker2}

	fpa, err := NewFixedPoolWorkerAllocator(suite.logger, workers)
	suite.NoError(err)
	suite.NotNil(fpa)

	// allocate once - should allocate
	firstAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.NoError(err)
	suite.Contains(workers, firstAllocatedWorker)

	// allocate again - should allocate other worker
	secondAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.NoError(err)
	suite.Contains(workers, secondAllocatedWorker)
	suite.NotEqual(firstAllocatedWorker, secondAllocatedWorker)

	// allocate yet again - should time out
	failedAllocationWorker, err := fpa.Allocate(50 * time.Millisecond)
	suite.Error(err)
	suite.Nil(failedAllocationWorker)

	// release the second worker
	suite.NotPanics(func() {fpa.Release(worker2)})

	// allocate again - should allocate second worker
	thirdAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.NoError(err)
	suite.Equal(worker2, thirdAllocatedWorker)

	suite.True(fpa.Shareable())
}

func TestAllocatorTestSuite(t *testing.T) {
	suite.Run(t, new(AllocatorTestSuite))
}
