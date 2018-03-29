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
	"github.com/nuclio/nuclio/cmd/processor/app"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
)

// ProcessorChanger is a change to the processor
type ChangeProcessorFunc func(processor *app.Processor) error

// ProcessorUpdater calculate diff and update
type ProcessorUpdater struct {
	Logger  logger.Logger
	Changes []ChangeProcessorFunc
}

// NewProcessorUpdater creates a new process updates
func NewProcessorUpdater(configBefore, configAfter *processor.Configuration, logger logger.Logger) (*ProcessUpdater, error) {
	updater := &ProcessorUpdater{
		Logger: logger.GetChild("process-updater"),
	}

	return updater.calculateDiffs(configBefore, configAfter)
}

func (pu *ProcessorUpdater) (configBefore, configAfter *processor.Configuration) error {
	for triggerID, triggerBefore := range configBefore.Spec.Triggers{
		triggerAfter, found := configAfter[triggerID]
		if !found {
			return errors.Errorf("Trigger removal (id=%s) not supported", triggerID)
		}

		if err := updater.partitionsDiff(triggerID, triggerBefore, triggerAfter); err != nil {
			return err
		}
	}

	for triggerID, triggerAfter := range configAfter.Spec.Triggers {
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
			pu.logger.InfoWith("Partition marked for removal", "id", partitionBefore.ID)
			removedPartition := partitionBefore // Closure copy
			removeFunc := func(processor *app.Processor) error {
				trigger := findTrigger(app, removedID)
				if trigger == nil {
					return errors.Errorf("Can't find trigger %s to update", removedID)
				}

				return trigger.RemovePartition(&removedPartition)

			}
			u.changes = append(u.changes, removeFunc)
		} else if partitionBefore.Checkpoint != partitionAfter.Checkpoint {
			u.logger.InfoWith("Partition marked for change", "id", partitionBefore.ID)
			updatedPartition := partitionBefore // Closure copy
			updateFunc := func(processor *app.Processor) error {
				trigger := findTrigger(app, removedID)
				if trigger == nil {
					return errors.Errorf("Can't find trigger %s to update", removedID)
				}

				return trigger.UpdatePartition(&removedPartition)

			}
			u.changes = append(u.changes, updateFunc)
		}
	}

	for partitionID, partitionAfter := range partitionsAfter {
		if _, found := partitionsBefore[partitionID]; found {
			continue
		}
		u.logger.InfoWith("Partition marked for addition", "id", partitionID)
		addedPartition := partitionAfter // Closure copy
		updateFunc := func(processor *app.Processor) error {
			trigger := findTrigger(app, removedID)
			if trigger == nil {
				return errors.Errorf("Can't find trigger %s to update", removedID)
			}

			return trigger.AddPartition(&addedPartition)

		}
		u.changes = append(u.changes, addFunc)
	}

	return nil
}

func findTrigger(processor *app.Processor, triggerID string) trigger.Trigger {
	for _, trigger := range processor.GetTriggers() {
		if trigger.GetID() == triggerID {
			return trigger
		}
	}

	return nil
}