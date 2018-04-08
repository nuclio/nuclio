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
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
)

// Processor interface
type Processor interface {
	GetTriggers() []trigger.Trigger
}

// ProcessorChange is a change to the processor
type ProcessorChange interface {
	Apply(processor Processor) error
	String() string
}

// Updater is a object that updates configuration
type Updater struct {
	logger  logger.Logger
	changes []ProcessorChange
}

// NewUpdater return a Updater
func NewUpdater(logger logger.Logger) *Updater {
	return &Updater{
		logger: logger.GetChild("updater"),
	}
}

// CalculateDiff calculates difference between configuration
func (u *Updater) CalculateDiff(configBefore, configAfter *processor.Configuration) error {
	for triggerID, triggerAfter := range configAfter.Spec.Triggers {
		triggerBefore, found := configBefore.Spec.Triggers[triggerID]

		if !found {
			u.logger.ErrorWith("Unknown trigger", "id", triggerID)
			return errors.Errorf("Unknown trigger (id=%s)", triggerID)
		}

		if err := u.partitionsDiff(triggerID, triggerBefore, triggerAfter); err != nil {
			return err
		}
	}

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

func (u *Updater) partitionsDiff(triggerID string, triggerBefore, triggerAfter functionconfig.Trigger) error {
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
			u.logger.InfoWith("Partition marked for removal", "id", partitionID)
			remover := &partitionRemover{
				partitionChanger{
					triggerID: triggerID,
					partition: partitionBefore,
				},
			}
			u.changes = append(u.changes, remover)
		} else if partitionAfter.Checkpoint != partitionBefore.Checkpoint {
			u.logger.InfoWith("Partition marked for change", "id", partitionID)
			updater := &partitionUpdater{
				partitionChanger{
					triggerID: triggerID,
					partition: partitionAfter,
				},
			}
			u.changes = append(u.changes, updater)
		}
	}

	for partitionID, partitionAfter := range partitionsAfter {
		if _, found := partitionsBefore[partitionID]; !found {
			adder := &partitionAdder{
				partitionChanger{
					triggerID: triggerID,
					partition: partitionAfter,
				},
			}
			u.changes = append(u.changes, adder)
		}
	}

	return nil
}

type partitionChanger struct {
	triggerID string
	partition functionconfig.Partition
	action    string
}

func (pc *partitionChanger) findTrigger(processor Processor) trigger.Trigger {
	for _, trigger := range processor.GetTriggers() {
		if trigger.GetID() == pc.triggerID {
			return trigger
		}
	}

	return nil
}

func (pc *partitionChanger) String() string {
	return fmt.Sprintf("trigger: %s, partition: %s, action: %s", pc.triggerID, pc.partition.ID, pc.action)
}

type partitionAdder struct {
	partitionChanger
}

func (pa *partitionAdder) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	return trigger.AddPartition(&pa.partition)
}

type partitionRemover struct {
	partitionChanger
}

func (pa *partitionRemover) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	return trigger.RemovePartition(&pa.partition)
}

type partitionUpdater struct {
	partitionChanger
}

func (pa *partitionUpdater) Apply(processor Processor) error {
	trigger := pa.findTrigger(processor)
	if trigger == nil {
		return errors.Errorf("no such trigger - %q", pa.triggerID)
	}

	return trigger.UpdatePartition(&pa.partition)
}
