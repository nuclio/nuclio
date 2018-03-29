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

package config

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

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

func (mt *MockTrigger) Start(checkpoint trigger.Checkpoint) error {
	return nil
}

func (mt *MockTrigger) Stop(force bool) (trigger.Checkpoint, error) {
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
		AbstractTrigger: trigger.AbstractTrigger{ID: id},
	}
}

func (suite *UpdaterTestSuite) TestfindTrigger() {
	processor := testProcessor{
		NewMockTrigger("1"),
		NewMockTrigger("2"),
		NewMockTrigger("3"),
		NewMockTrigger("4"),
	}

	triggerID := "2"
	foundTrigger := findTrigger(processor, triggerID)
	suite.Require().NotNilf(foundTrigger, "Can't find trigger %q", triggerID)
	suite.Require().Equal(triggerID, foundTrigger.GetID(), "Bad ID found")

	foundTrigger = findTrigger(processor, "100")
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

	pu, err := NewProcessorUpdater(configBefore, configAfter, logger)
	suite.Require().NoError(err, "Can't create updater")
	suite.Require().Equal(2, len(pu.Changes), "Wrong number of changes")

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
}

func TestUpdaterTestSuite(t *testing.T) {
	suite.Run(t, new(UpdaterTestSuite))
}
