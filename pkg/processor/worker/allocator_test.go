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
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	testRuntimeName = fmt.Sprintf("test-runtime-%s", xid.New())
)

type AllocatorTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AllocatorTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *AllocatorTestSuite) TestSingletonAllocator() {
	worker1 := &Worker{}

	sa, err := NewSingletonWorkerAllocator(suite.logger, worker1)
	suite.Require().NoError(err)
	suite.Require().NotNil(sa)

	// allocate once, time should be ignored
	allocatedWorker, err := sa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// allocate again, release doesn't need to happen
	allocatedWorker, err = sa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// release shouldn't do anything
	suite.Require().NotPanics(func() { sa.Release(worker1) })

	suite.Require().False(sa.Shareable())
}

func (suite *AllocatorTestSuite) TestFixedPoolAllocator() {
	worker1 := &Worker{index: 0}
	worker2 := &Worker{index: 1}
	workers := []*Worker{worker1, worker2}

	fpa, err := NewFixedPoolWorkerAllocator(suite.logger, workers)
	suite.Require().NoError(err)
	suite.Require().NotNil(fpa)

	// allocate once - should allocate
	firstAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(workers, firstAllocatedWorker)

	// allocate again - should allocate other worker
	secondAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(workers, secondAllocatedWorker)
	suite.NotEqual(firstAllocatedWorker, secondAllocatedWorker)

	// allocate yet again - should time out
	failedAllocationWorker, err := fpa.Allocate(50 * time.Millisecond)
	suite.Require().Error(err)
	suite.Require().Nil(failedAllocationWorker)

	// release the second worker
	suite.Require().NotPanics(func() { fpa.Release(worker2) })

	// allocate again - should allocate second worker
	thirdAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker2, thirdAllocatedWorker)

	suite.Require().True(fpa.Shareable())
}

type MockCreator struct {
	mock.Mock
}

func (mc *MockCreator) Create(logger.Logger, *runtime.Configuration) (runtime.Runtime, error) {
	return &MockRuntime{}, nil
}

func (suite *AllocatorTestSuite) TestFlexPoolAllocator() {
	runtime.RegistrySingleton.Register(testRuntimeName, &MockCreator{})

	runtimeConfiguration := &runtime.Configuration{
		Configuration: &processor.Configuration{},
	}
	runtimeConfiguration.Spec.Runtime = testRuntimeName

	require := suite.Require()

	allocator, err := NewFlexPoolWorkerAllocator(suite.logger, runtimeConfiguration, nil)
	require.NoError(err, "Can't create flexPool")

	pool, ok := allocator.(*flexPool)
	require.True(ok, "Can't extract flexPool from allocator")

	worker, err := allocator.Allocate(0)
	require.NoError(err, "Can't allocate")
	require.Equal(1, len(pool.allocatedWorkers), "bad number of allocated")
	require.Equal(0, len(pool.freeWorkers), "bad number of free")
	require.Equal(1, pool.nextIndex(), "bad next index")

	allocator.Release(worker)
	require.Equal(0, len(pool.allocatedWorkers), "bad number of allocated")
	require.Equal(1, len(pool.freeWorkers), "bad number of free")
	require.Equal(1, pool.nextIndex(), "bad next index")

	worker2, err := allocator.Allocate(0)
	require.NoError(err, "Can't allocate (2nd time)")
	require.Equal(worker, worker2, "Didn't allocate same worker")
	allocator.Release(worker)
	require.Equal(1, pool.nextIndex(), "bad next index")

	err = allocator.Delete(worker)
	require.NoError(err, "Can't delete")
	require.Equal(0, len(pool.allocatedWorkers), "bad number of allocated")
	require.Equal(0, len(pool.freeWorkers), "bad number of free")
	require.Equal(0, pool.nextIndex(), "bad next index")
}

func TestAllocatorTestSuite(t *testing.T) {
	suite.Run(t, new(AllocatorTestSuite))
}
