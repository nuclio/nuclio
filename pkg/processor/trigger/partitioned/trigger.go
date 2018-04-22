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

package partitioned

import (
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
)

// Stream defines a stream
type Stream interface {

	// CreatePartitions creates partitions, as per configuration
	CreatePartitions() ([]Partition, error)

	// Start starts reading from partitions
	Start(checkpoint functionconfig.Checkpoint) error

	// Start stops reading from partitions
	Stop(force bool) (functionconfig.Checkpoint, error)
}

// AbstractStream implements common stream behavior
type AbstractStream struct {
	trigger.AbstractTrigger
	stream          Stream
	partitions      []Partition
	configuration   *Configuration
	workerAllocator worker.Allocator
}

func NewAbstractStream(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	stream Stream,
	kind string) (*AbstractStream, error) {

	newAbstractStream := &AbstractStream{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            kind,
		},
		workerAllocator: workerAllocator,
		configuration:   configuration,
		stream:          stream,
	}

	return newAbstractStream, nil
}

func (as *AbstractStream) Initialize() error {
	var err error

	as.partitions, err = as.stream.CreatePartitions()
	if err != nil {
		return errors.Wrap(err, "Failed to create partitions")
	}

	return nil
}

func (as *AbstractStream) Start(checkpoint functionconfig.Checkpoint) error {
	for _, partition := range as.partitions {

		// start reading from partition
		go func(partition Partition) {
			if err := partition.Read(); err != nil {
				as.Logger.ErrorWith("Failed to read from partition", "err", err)
			}
		}(partition)
	}

	return nil
}

func (as *AbstractStream) Stop(force bool) (functionconfig.Checkpoint, error) {
	return nil, nil
}

func (as *AbstractStream) GetConfig() map[string]interface{} {
	return common.StructureToMap(as.configuration)
}
