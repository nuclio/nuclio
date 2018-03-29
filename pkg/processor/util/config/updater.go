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

package config

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
)

// ChangeProcessorFunc is a change to the processor
type ChangeProcessorFunc func(processor Processor) error

// ProcessorUpdater calculate diff and update
type ProcessorUpdater struct {
	Logger  logger.Logger
	Changes []ChangeProcessorFunc
}

// Processor interface
type Processor interface {
	GetTriggers() []trigger.Trigger
}

// NewProcessorUpdater creates a new process updates
func NewProcessorUpdater(configBefore, configAfter *processor.Configuration, logger logger.Logger) (*ProcessorUpdater, error) {
	updater := &ProcessorUpdater{
		Logger: logger.GetChild("process-updater"),
	}

	return updater, updater.calculateDiffs(configBefore, configAfter)
}

// Apply changes to processor
func (pu *ProcessorUpdater) Apply(processor Processor) error {
	for _, changeFunc := range pu.Changes {
		if err := changeFunc(processor); err != nil {
			return err
		}
	}

	return nil
}

func (pu *ProcessorUpdater) calculateDiffs(configBefore, configAfter *processor.Configuration) error {
	for triggerID, triggerBefore := range configBefore.Spec.Triggers {
		triggerAfter, found := configAfter.Spec.Triggers[triggerID]
		if !found {
			return errors.Errorf("Trigger removal (id=%s) not supported", triggerID)
		}

		if err := pu.partitionsDiff(triggerID, triggerBefore, triggerAfter); err != nil {
			return err
		}
	}

	for triggerID := range configAfter.Spec.Triggers {
		if _, found := configBefore.Spec.Triggers[triggerID]; !found {
			return errors.Errorf("Trigger addition (id=%s) not supported", triggerID)
		}
	}

	return nil
}

func (pu *ProcessorUpdater) partitionsDiff(triggerID string, triggerBefore, triggerAfter functionconfig.Trigger) error {
	partitionsBefore := make(map[string]functionconfig.Partition)
	for _, partition := range triggerBefore.Partitions {
		partitionsBefore[partition.ID] = partition
	}
	partitionsAfter := make(map[string]functionconfig.Partition)
	for _, partition := range triggerAfter.Partitions {
		partitionsAfter[partition.ID] = partition
	}

	for partitionID, partitionBefore := range partitionsBefore {
		if partitionAfter, found := partitionsAfter[partitionID]; !found {
			pu.Logger.InfoWith("Partition marked for removal", "id", partitionBefore.ID)
			removedPartition := partitionBefore // Closure copy
			removeFunc := func(processor Processor) error {
				trigger := findTrigger(processor, triggerID)
				if trigger == nil {
					return errors.Errorf("Can't find trigger %s to update", triggerID)
				}

				return trigger.RemovePartition(&removedPartition)

			}
			pu.Changes = append(pu.Changes, removeFunc)
		} else if partitionBefore.Checkpoint != partitionAfter.Checkpoint {
			pu.Logger.InfoWith("Partition marked for change", "id", partitionBefore.ID)
			updatedPartition := partitionBefore // Closure copy
			updateFunc := func(processor Processor) error {
				trigger := findTrigger(processor, triggerID)
				if trigger == nil {
					return errors.Errorf("Can't find trigger %s to update", triggerID)
				}

				return trigger.UpdatePartition(&updatedPartition)

			}
			pu.Changes = append(pu.Changes, updateFunc)
		}
	}

	for partitionID, partitionAfter := range partitionsAfter {
		if _, found := partitionsBefore[partitionID]; found {
			continue
		}
		pu.Logger.InfoWith("Partition marked for addition", "id", partitionID)
		addedPartition := partitionAfter // Closure copy
		addFunc := func(processor Processor) error {
			trigger := findTrigger(processor, triggerID)
			if trigger == nil {
				return errors.Errorf("Can't find trigger %s to update", triggerID)
			}

			return trigger.AddPartition(&addedPartition)

		}
		pu.Changes = append(pu.Changes, addFunc)
	}

	return nil
}

func findTrigger(processor Processor, triggerID string) trigger.Trigger {
	for _, trigger := range processor.GetTriggers() {
		if trigger.GetID() == triggerID {
			return trigger
		}
	}

	return nil
}
