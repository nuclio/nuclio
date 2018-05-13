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

// This file can't be in pkg/processor/config since it'll create an import cycle
// TODO: Should this be in cmd/processor/app ?

package updater

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util"

	"github.com/nuclio/logger"
)

// Processor interface
type Processor interface {
	GetTriggers() []trigger.Trigger
	GetConfiguration() *processor.Configuration
}

// ProcessorChange is a change to the processor
type ProcessorChange interface {
	Apply(processor Processor) error
}

// Updater is a object that updates configuration
type Updater struct {
	logger        logger.Logger
	configuration *processor.Configuration
	changes       []ProcessorChange
}

// NewUpdater return a Updater
func NewUpdater(logger logger.Logger) *Updater {
	return &Updater{
		logger: logger.GetChild("updater"),
	}
}

// CalculateDiff calculates difference between configuration
func (u *Updater) CalculateDiff(configBefore *processor.Configuration, configAfter *processor.Configuration) error {
	for triggerID, triggerAfter := range configAfter.Spec.Triggers {
		triggerBefore, found := configBefore.Spec.Triggers[triggerID]

		if !found {
			return errors.Errorf("Unknown trigger (id=%s)", triggerID)
		}

		if err := u.partitionsDiff(triggerID, triggerBefore, triggerAfter); err != nil {
			return err
		}
	}

	u.configuration = util.CopyConfiguration(configAfter)
	u.addRemoved()

	return nil
}

// Apply applies changes to processor
func (u *Updater) Apply(processor Processor) error {
	for _, change := range u.changes {
		if err := change.Apply(processor); err != nil {
			return err
		}
	}

	return nil
}

// GetConfiguration return the processor configuration after updates
func (u *Updater) GetConfiguration() *processor.Configuration {
	return u.configuration
}

func (u *Updater) addRemoved() {
	// Added removed triggers
	for _, change := range u.changes {
		remover, ok := change.(*partitionRemover)
		if !ok {
			continue
		}
		triggerConfig, ok := u.configuration.Spec.Triggers[remover.triggerID]
		if !ok {
			u.logger.WarnWith("Can't find trigger", "id", remover.triggerID)
			// TODO: return error?
			continue
		}

		partition := *remover.partition // copy
		partition.Checkpoint = remover.checkpoint
		triggerConfig.Partitions = append(triggerConfig.Partitions, partition)
		u.configuration.Spec.Triggers[remover.triggerID] = triggerConfig
	}
}

// Calculate actions for partition diff
// We first do removes to free workers and finally add a call to GC workers
func (u *Updater) partitionsDiff(triggerID string, triggerBefore, triggerAfter functionconfig.Trigger) error {
	partitionsBefore := make(map[string]functionconfig.Partition)
	for _, partition := range triggerBefore.Partitions {
		partitionsBefore[partition.ID] = partition
	}
	partitionsAfter := make(map[string]functionconfig.Partition)
	for _, partition := range triggerAfter.Partitions {
		partitionsAfter[partition.ID] = partition
	}

	// We start with removals to free workers
	for partitionID, partitionBefore := range partitionsBefore {
		if partitionAfter, found := partitionsAfter[partitionID]; !found {
			u.logger.InfoWith("Partition marked for removal", "id", partitionID)
			remover := newPartitionRemover(u.logger, triggerID, &partitionBefore)
			u.changes = append(u.changes, remover)
		} else if partitionAfter.Checkpoint != partitionBefore.Checkpoint {
			u.logger.InfoWith("Partition marked for change", "id", partitionID)
			updater := newPartitionUpdater(u.logger, triggerID, &partitionAfter)
			u.changes = append(u.changes, updater)
		}
	}

	for partitionID, partitionAfter := range partitionsAfter {
		if _, found := partitionsBefore[partitionID]; !found {
			adder := newPartitionAdder(u.logger, triggerID, &partitionAfter)
			u.changes = append(u.changes, adder)
		}
	}

	// Finally GC workers
	gc := newPartitionGC(u.logger, triggerID)
	u.changes = append(u.changes, gc)

	return nil
}

type partitionChanger struct {
	triggerID string
	partition *functionconfig.Partition
	logger    logger.Logger
}

func newPartitionChanger(logger logger.Logger, triggerID string, partition *functionconfig.Partition) partitionChanger {

	if partition != nil {
		partitionCopy := *partition
		partition = &partitionCopy
	}

	return partitionChanger{
		logger:    logger,
		triggerID: triggerID,
		partition: partition,
	}
}

func (pc *partitionChanger) findTrigger(processor Processor) trigger.Trigger {
	for _, trigger := range processor.GetTriggers() {
		if trigger.GetID() == pc.triggerID {
			return trigger
		}
	}

	return nil
}

type partitionAdder struct {
	partitionChanger
}

func (pa *partitionAdder) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	pa.logger.InfoWith("Adding partition", "trigger", pa.triggerID, "partition", &pa.partition)
	return trigger.AddPartition(pa.partition)
}

func newPartitionAdder(logger logger.Logger, triggerID string, partition *functionconfig.Partition) *partitionAdder {
	return &partitionAdder{
		newPartitionChanger(logger, triggerID, partition),
	}
}

type partitionRemover struct {
	partitionChanger
	checkpoint functionconfig.Checkpoint
}

func (pa *partitionRemover) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	pa.logger.InfoWith("Removing partition", "trigger", pa.triggerID, "partition", &pa.partition)
	var err error

	pa.checkpoint, err = trigger.RemovePartition(pa.partition)
	return err
}

func newPartitionRemover(logger logger.Logger, triggerID string, partition *functionconfig.Partition) *partitionRemover {
	return &partitionRemover{
		partitionChanger: newPartitionChanger(logger, triggerID, partition),
	}
}

type partitionUpdater struct {
	partitionChanger
}

func (pa *partitionUpdater) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	pa.logger.InfoWith("Updating partition", "trigger", pa.triggerID, "partition", &pa.partition)
	return trigger.UpdatePartition(pa.partition)
}

func newPartitionUpdater(logger logger.Logger, triggerID string, partition *functionconfig.Partition) *partitionUpdater {
	return &partitionUpdater{
		newPartitionChanger(logger, triggerID, partition),
	}
}

type partitionGC struct {
	partitionChanger
}

func (pg *partitionGC) Apply(processor Processor) error {
	trigger := pg.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pg.triggerID)
	}

	pg.logger.InfoWith("Running GC", "trigger", pg.triggerID)
	return trigger.GetAllocator().GC()
}

func newPartitionGC(logger logger.Logger, triggerID string) *partitionGC {
	return &partitionGC{
		newPartitionChanger(logger, triggerID, nil),
	}
}
