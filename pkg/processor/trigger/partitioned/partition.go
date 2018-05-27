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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
)

// Partition interface
type Partition interface {
	// Read starts reading from stream partition
	Read() error

	// GetID returns the id (e.g. partition)
	GetID() int
	// GetCheckpoint return the current checkpoint (e.g. offset)
	GetCheckpoint() functionconfig.Checkpoint
}

// AbstractPartition holds common methods for partitions
type AbstractPartition struct {
	Logger logger.Logger
	Stream *AbstractStream
	Worker *worker.Worker
}

// NewAbstractPartition will return a new AbstractPartition
func NewAbstractPartition(logger logger.Logger, stream *AbstractStream) (*AbstractPartition, error) {
	var err error

	newPartition := &AbstractPartition{
		Logger: logger,
		Stream: stream,
	}

	newPartition.Worker, err = newPartition.Stream.workerAllocator.Allocate(0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker")
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create partition consumer")
	}

	return newPartition, nil
}

// TODO: Remove GetState & GetID once all partitioned triggers support dealer interface

// GetID returns the current ID (e.g. partition)
func (s *AbstractPartition) GetID() int {
	return -1
}

// GetCheckpoint returns the current state (e.g. offset)
func (s *AbstractPartition) GetCheckpoint() functionconfig.Checkpoint {
	return nil
}
