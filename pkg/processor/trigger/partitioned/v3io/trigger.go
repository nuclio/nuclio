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

package v3io

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"
	"github.com/nuclio/nuclio/pkg/processor/util/v3io"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	v3iohttp "github.com/v3io/v3io-go-http"
)

type v3io struct {
	*partitioned.AbstractStream
	configuration *Configuration
	container     *v3iohttp.Container
	streamPath    string
	seekType      v3iohttp.SeekShardInputType
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	// parse URL
	addr, containerAlias, streamPath, err := v3ioutil.ParseURL(configuration.URL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse URL")
	}

	newTrigger := &v3io{
		configuration: configuration,
		streamPath:    streamPath,
	}

	newTrigger.AbstractStream, err = partitioned.NewAbstractStream(parentLogger,
		workerAllocator,
		&configuration.Configuration,
		newTrigger,
		"v3io")

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract stream")
	}

	// get seek type from configuration
	newTrigger.seekType, err = newTrigger.getSeekTypeFromConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get seek type")
	}

	// try to create a container. use half the amount of workers than there are partitions + 1
	newTrigger.container, err = v3ioutil.CreateContainer(newTrigger.Logger,
		addr,
		containerAlias,
		newTrigger.configuration.NumContainerWorkers)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create v3io container")
	}

	return newTrigger, nil
}

func (v *v3io) CreatePartitions() ([]partitioned.Partition, error) {
	var partitions []partitioned.Partition

	// iterate over partitions and create
	for _, partitionID := range v.configuration.Partitions {

		// create the partition
		partition, err := newPartition(v.Logger, v, partitionID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create partition")
		}

		// add partition
		partitions = append(partitions, partition)
	}

	return partitions, nil
}

func (v *v3io) getSeekTypeFromConfiguration(configuration *Configuration) (v3iohttp.SeekShardInputType, error) {
	switch seekToType(configuration.SeekTo) {
	case seekToTypeLatest:
		return v3iohttp.SeekShardInputTypeLatest, nil
	case seekToTypeEarlist:
		return v3iohttp.SeekShardInputTypeEarliest, nil
	default:
		return -1, fmt.Errorf("Unsupported seekTo type: %s", configuration.SeekTo)
	}
}
