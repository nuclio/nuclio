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

package updater

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testProcessor []trigger.Trigger

func (tp testProcessor) GetTriggers() []trigger.Trigger {
	return tp
}

type UpdaterTestSuite struct {
	suite.Suite
}

type MockTrigger struct {
	trigger.AbstractTrigger
	Removed int
	Added   int
	Updated int
}

func (mt *MockTrigger) GetConfig() map[string]interface{} {
	return nil
}

func (mt *MockTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	return nil
}

func (mt *MockTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	return nil, nil
}

func (mt *MockTrigger) AddPartition(partition *functionconfig.Partition) error {
	mt.Added++
	return nil
}
func (mt *MockTrigger) RemovePartition(partition *functionconfig.Partition) error {
	mt.Removed++
	return nil
}
func (mt *MockTrigger) UpdatePartition(partition *functionconfig.Partition) error {
	mt.Updated++
	return nil
}

func NewMockTrigger(id string) *MockTrigger {
	return &MockTrigger{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              id,
			WorkerAllocator: &MockAllocator{},
		},
	}
}

type MockAllocator struct {
	didGC bool
}

func (ma *MockAllocator) Allocate(timeout time.Duration) (*worker.Worker, error) {
	return nil, nil
}

func (ma *MockAllocator) Release(worker *worker.Worker) {
}

func (ma *MockAllocator) Delete(worker *worker.Worker) error {
	return nil
}

func (ma *MockAllocator) Shareable() bool {
	return false
}

func (ma *MockAllocator) GC() error {
	ma.didGC = true
	return nil
}

func (ma *MockAllocator) GetWorkers() []*worker.Worker {
	panic("not implemented")
}

func (suite *UpdaterTestSuite) TestfindTrigger() {
	processor := testProcessor{
		NewMockTrigger("1"),
		NewMockTrigger("2"),
		NewMockTrigger("3"),
		NewMockTrigger("4"),
	}

	triggerID := "2"
	pc := &partitionChanger{triggerID: triggerID}
	foundTrigger := pc.findTrigger(processor)
	suite.Require().NotNilf(foundTrigger, "Can't find trigger %q", triggerID)
	suite.Require().Equal(triggerID, foundTrigger.GetID(), "Bad ID found")

	pc = &partitionChanger{triggerID: "100"}
	foundTrigger = pc.findTrigger(processor)
	suite.Require().Nil(foundTrigger, "Found non existing trigger")
}

func newProcessorConfig(triggerID string, partitions []functionconfig.Partition) *processor.Configuration {
	config := &processor.Configuration{}
	config.Spec.Triggers = map[string]functionconfig.Trigger{
		triggerID: functionconfig.Trigger{
			Partitions: partitions,
		},
	}

	return config
}

func (suite *UpdaterTestSuite) TestUpdater() {
	triggerID := "7"

	partitionsBefore := []functionconfig.Partition{
		functionconfig.Partition{ID: "1"},
		functionconfig.Partition{ID: "2"},
	}
	configBefore := newProcessorConfig(triggerID, partitionsBefore)

	partitionsAfter := []functionconfig.Partition{
		functionconfig.Partition{ID: "1"},
		functionconfig.Partition{ID: "3"},
	}
	configAfter := newProcessorConfig(triggerID, partitionsAfter)

	logger, err := nucliozap.NewNuclioZapTest("test updater")
	suite.Require().NoError(err, "Can't create logger")

	pu := NewUpdater(logger)
	err = pu.CalculateDiff(configBefore, configAfter)
	suite.Require().NoError(err, "Can't calculate diff")
	// 3 since we have a GC at the end
	suite.Require().Equal(3, len(pu.changes), "Wrong number of changes")

	trigger := NewMockTrigger(triggerID)
	processor := testProcessor{
		NewMockTrigger("1"),
		trigger,
		NewMockTrigger("3"),
	}

	err = pu.Apply(processor)
	suite.Require().NoError(err, "Can't apply changes")
	suite.Require().Equal(1, trigger.Removed, "Wrong number of removed")
	suite.Require().Equal(1, trigger.Added, "Wrong number of added")
	suite.Require().Equal(0, trigger.Updated, "Wrong number of updated")

	ma := trigger.WorkerAllocator.(*MockAllocator)
	suite.Require().True(ma.didGC, "GC not called")
}

func TestUpdaterTestSuite(t *testing.T) {
	suite.Run(t, new(UpdaterTestSuite))
}
