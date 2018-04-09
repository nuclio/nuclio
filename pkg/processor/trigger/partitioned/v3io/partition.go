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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/nuclio/logger"
	v3iohttp "github.com/v3io/v3io-go-http"
)

type partition struct {
	*partitioned.AbstractPartition
	partitionID int
	event       Event
	v3ioTrigger *v3io
}

func newPartition(parentLogger logger.Logger, v3ioTrigger *v3io, partitionID int) (*partition, error) {
	var err error

	partitionName := fmt.Sprintf("partition-%d", partitionID)

	// create a partition
	newPartition := &partition{
		partitionID: partitionID,
		v3ioTrigger: v3ioTrigger,
	}

	newPartition.AbstractPartition, err = partitioned.NewAbstractPartition(parentLogger.GetChild(partitionName),
		v3ioTrigger.AbstractStream)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract partition")
	}
	return newPartition, nil
}

func (p *partition) Read() error {
	partitionPath := fmt.Sprintf("%s/%d", p.v3ioTrigger.streamPath, p.partitionID)

	p.Logger.DebugWith("Seeking partition",
		"partitionPath", partitionPath,
		"seekType", p.v3ioTrigger.seekType)

	response, err := p.v3ioTrigger.container.Sync.SeekShard(&v3iohttp.SeekShardInput{
		Path: partitionPath,
		Type: p.v3ioTrigger.seekType,
	})

	if err != nil {
		return errors.Wrap(err, "Failed to seek partition")
	}

	// get the location
	location := response.Output.(*v3iohttp.SeekShardOutput).Location
	if location == "" {
		return errors.New("Got empty location from seek")
	}

	p.Logger.DebugWith("Starting to read from partition",
		"location", location,
		"pollingInterval", p.v3ioTrigger.configuration.PollingIntervalMs)

	// release seek shard response
	response.Release()

	pollingInterval := time.Duration(p.v3ioTrigger.configuration.PollingIntervalMs) * time.Millisecond

	for {
		time.Sleep(pollingInterval)

		// get records
		response, err = p.v3ioTrigger.container.Sync.GetRecords(&v3iohttp.GetRecordsInput{
			Path:     partitionPath,
			Location: location,
			Limit:    p.v3ioTrigger.configuration.ReadBatchSize,
		})

		// TODO: skip errors
		if err != nil {
			return errors.Wrap(err, "Failed to read from partition")
		}

		getRecordsOutput := response.Output.(*v3iohttp.GetRecordsOutput)

		// set next location
		location = getRecordsOutput.NextLocation

		// handle records by processing them in the function
		for _, record := range getRecordsOutput.Records {

			// set the record in the event
			p.event.record = &record

			// submit to worker
			p.Stream.SubmitEventToWorker(nil, p.Worker, &p.event) // nolint: errcheck
		}
	}
}
